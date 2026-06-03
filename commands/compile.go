// spec: spec/modules.md, spec/c-interop.md, spec/stdlib.md

package commands

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	cbackend "github.com/neutrino2211/gecko/backends/c_backend"
	"github.com/neutrino2211/gecko/compiler"
	"github.com/neutrino2211/gecko/config"
	"github.com/neutrino2211/gecko/logger"
	"github.com/urfave/cli/v2"

	"github.com/fatih/color"
)

func setLogLevel(ctx *cli.Context) {
	level := ctx.String("log-level")
	logger.SetLogLevel(logger.ParseLogLevel(level))
}

// CopyFile copies a file from src to dst. If src and dst files exist, and are
// the same, then return success. Otherise, attempt to create a hard link
// between the two files. If that fail, copy the file contents from src to dst.
func CopyFile(src, dst string) (err error) {
	sfi, err := os.Stat(src)
	if err != nil {
		return
	}
	if !sfi.Mode().IsRegular() {
		// cannot copy non-regular files (e.g., directories,
		// symlinks, devices, etc.)
		return fmt.Errorf("CopyFile: non-regular source file %s (%q)", sfi.Name(), sfi.Mode().String())
	}
	dfi, err := os.Stat(dst)
	if err != nil {
		if !os.IsNotExist(err) {
			return
		}
	} else {
		if !(dfi.Mode().IsRegular()) {
			return fmt.Errorf("CopyFile: non-regular destination file %s (%q)", dfi.Name(), dfi.Mode().String())
		}
		if os.SameFile(sfi, dfi) {
			return
		}
	}
	if err = os.Link(src, dst); err == nil {
		return
	}
	err = copyFileContents(src, dst)
	return
}

// copyFileContents copies the contents of the file named src to the file named
// by dst. The file will be created if it does not already exist. If the
// destination file exists, all it's contents will be replaced by the contents
// of the source file.
func copyFileContents(src, dst string) (err error) {
	in, err := os.Open(src)
	if err != nil {
		return
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return
	}
	defer func() {
		cerr := out.Close()
		if err == nil {
			err = cerr
		}
	}()
	if _, err = io.Copy(out, in); err != nil {
		return
	}
	err = out.Sync()
	return
}

func resolveTargetKey(ctx *cli.Context, projectCfg *config.ProjectConfig) string {
	if ctx.IsSet("target-arch") || ctx.IsSet("target-platform") || ctx.IsSet("target-vendor") {
		arch := ctx.String("target-arch")
		platform := ctx.String("target-platform")
		vendor := ctx.String("target-vendor")
		if arch != "" && platform != "" {
			if vendor != "" {
				return arch + "-" + vendor + "-" + platform
			}
			return arch + "-" + platform
		}
	}

	if projectCfg != nil && projectCfg.Build.DefaultTarget != "" {
		return projectCfg.Build.DefaultTarget
	}

	return ""
}

func dedupeStrings(values []string) []string {
	out := make([]string, 0, len(values))
	seen := make(map[string]bool, len(values))
	for _, v := range values {
		if v == "" {
			continue
		}
		if seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	return out
}

func collectCImportPkgCFlags() []string {
	if len(cbackend.LastCImportLibraries) == 0 {
		return nil
	}
	var flags []string
	for _, lib := range dedupeStrings(cbackend.LastCImportLibraries) {
		if pkgCFlags, err := runPkgConfig("--cflags", []string{lib}); err == nil {
			flags = append(flags, pkgCFlags...)
		}
	}
	return flags
}

func collectCImportPkgLibFlags(staticLink bool) []string {
	if len(cbackend.LastCImportLibraries) == 0 {
		return nil
	}
	pkgFlag := "--libs"
	if staticLink {
		pkgFlag = "--libs --static"
	}

	var flags []string
	for _, lib := range dedupeStrings(cbackend.LastCImportLibraries) {
		if pkgLibs, err := runPkgConfigWithFlags(pkgFlag, []string{lib}); err == nil {
			flags = append(flags, pkgLibs...)
		}
	}
	return flags
}

func collectObjectInputs(projectCfg *config.ProjectConfig, targetKey string) []string {
	var objects []string
	if projectCfg != nil {
		objects = append(objects, projectCfg.GetNativeObjectsForTarget(targetKey)...)
	}
	objects = append(objects, cbackend.LastCImportObjects...)
	return dedupeStrings(objects)
}

var CompileCommand = &cli.Command{
	Name:        "compile",
	Aliases:     []string{"c"},
	Usage:       "gecko compile [sources...] or gecko compile --entry <name>",
	Description: compileHelp,
	Action: func(ctx *cli.Context) error {
		setLogLevel(ctx)

		// Try to load project config
		wd, _ := os.Getwd()
		projectCfg, cfgErr := config.LoadProjectConfig(wd)

		// Determine sources to compile
		var sources []string
		targetKey := resolveTargetKey(ctx, projectCfg)

		if ctx.Args().Len() > 0 {
			// Explicit sources provided
			sources = ctx.Args().Slice()
		} else if cfgErr == nil && projectCfg != nil {
			// No args - use gecko.toml entries
			entryName := ctx.String("entry")

			if entryName != "" {
				// Specific entry requested
				entryPath, err := projectCfg.GetEntry(entryName)
				if err != nil {
					return fmt.Errorf("entry '%s' not found in gecko.toml", entryName)
				}
				sources = []string{entryPath}
			} else if len(projectCfg.Build.Entries) > 0 {
				// Build first entry by default
				for name, path := range projectCfg.Build.Entries {
					fullPath := filepath.Join(projectCfg.ProjectRoot, path)
					sources = []string{fullPath}
					fmt.Printf("Building entry '%s': %s\n", name, path)
					break
				}
			} else {
				return fmt.Errorf("no entries defined in gecko.toml and no sources provided")
			}
		} else {
			println("No sources provided and no gecko.toml found")
			println("Usage: gecko compile <source.gecko> or create a gecko.toml with [build.entries]")
			return nil
		}

		for _, pos := range sources {
			outFile := compiler.Compile(pos, &config.CompileCfg{
				Arch:      ctx.String("target-arch"),
				Platform:  ctx.String("target-platform"),
				Vendor:    ctx.String("target-vendor"),
				TargetKey: targetKey,
				CFlags:    []string{},
				CLFlags:   []string{},
				CObjects:  []string{},
				Ctx:       ctx,
				Project:   projectCfg,
			})

			if outFile != "" {
				destObj := pos + ".o"
				if projectCfg != nil {
					destObj = projectCfg.GetArtifactPath(pos, ".o")
				}
				_ = os.MkdirAll(filepath.Dir(destObj), 0o755)
				CopyFile(outFile, destObj)
			}
		}

		compiler.PrintErrorSummary()

		return nil
	},
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "output-dir",
			Value: ".",
			Usage: "Output directory path " + color.HiYellowString("(warning: this overrides the build configuration's output directory)"),
		},
		&cli.StringFlag{
			Name:    "entry",
			Aliases: []string{"e"},
			Value:   "",
			Usage:   "Entry point name from gecko.toml [build.entries]",
		},
		&cli.StringFlag{
			Name:  "type",
			Value: "executable",
			Usage: "Output type for program. (executable | library)",
		},
		&cli.StringFlag{
			Name:  "backend",
			Value: "c",
			Usage: "The compilation backend to use (c | llvm [experimental])",
		},
		&cli.StringFlag{
			Name:  "target-arch",
			Value: runtime.GOARCH,
			Usage: "The compilation target architecture",
		},
		&cli.StringFlag{
			Name:  "target-platform",
			Value: runtime.GOOS,
			Usage: "The compilation target operating system",
		},
		&cli.StringFlag{
			Name:  "target-vendor",
			Value: "",
			Usage: "The compilation target vendor of file type",
		},
		&cli.BoolFlag{
			Name:  "print-ir",
			Value: false,
			Usage: "Print the file's LLVM IR",
		},
		&cli.BoolFlag{
			Name:  "ir-only",
			Value: false,
			Usage: "Only compile to IR",
		},
		&cli.StringSliceFlag{
			Name:  "llc-args",
			Value: &cli.StringSlice{},
			Usage: "Pass arguments to underlying llc command",
		},
		&cli.StringFlag{
			Name:  "log-level",
			Value: "silent",
			Usage: "Log level (silent | error | warn | info | debug | trace)",
		},
	},
}

var (
	compileHelp  = `compiles a gecko source file to C code`
	buildHelp    = `compiles a gecko source file to an executable`
	runHelp      = `compiles and runs a gecko source file`
	invokeDir, _ = os.Getwd()
)

// Common flags shared between commands
var commonFlags = []cli.Flag{
	&cli.StringFlag{
		Name:  "backend",
		Value: "c",
		Usage: "The compilation backend to use (c | llvm [experimental])",
	},
	&cli.StringFlag{
		Name:  "target-arch",
		Value: runtime.GOARCH,
		Usage: "The compilation target architecture",
	},
	&cli.StringFlag{
		Name:  "target-platform",
		Value: runtime.GOOS,
		Usage: "The compilation target operating system",
	},
	&cli.StringFlag{
		Name:  "target-vendor",
		Value: "",
		Usage: "The compilation target vendor",
	},
	&cli.StringFlag{
		Name:  "log-level",
		Value: "silent",
		Usage: "Log level (silent | error | warn | info | debug | trace)",
	},
	&cli.StringSliceFlag{
		Name:  "cflags",
		Usage: "Additional C compiler flags (can be specified multiple times)",
	},
	&cli.StringSliceFlag{
		Name:  "ldflags",
		Usage: "Additional linker flags (can be specified multiple times)",
	},
	&cli.StringSliceFlag{
		Name:  "pkg-config",
		Usage: "pkg-config packages to include (can be specified multiple times)",
	},
}

// compileToC compiles a gecko file to C and returns the C file path
func compileToC(ctx *cli.Context, source string, projectCfg *config.ProjectConfig) (string, error) {
	targetKey := resolveTargetKey(ctx, projectCfg)

	// Collect CFlags from CLI
	cflags := ctx.StringSlice("cflags")

	// Add pkg-config --cflags if specified via CLI
	pkgConfigPkgs := ctx.StringSlice("pkg-config")
	if len(pkgConfigPkgs) > 0 {
		if pkgCFlags, err := runPkgConfig("--cflags", pkgConfigPkgs); err == nil {
			cflags = append(cflags, pkgCFlags...)
		}
	}

	// Create a modified context with ir-only set
	compiler.Compile(source, &config.CompileCfg{
		Arch:      ctx.String("target-arch"),
		Platform:  ctx.String("target-platform"),
		Vendor:    ctx.String("target-vendor"),
		TargetKey: targetKey,
		CFlags:    cflags,
		CLFlags:   ctx.StringSlice("ldflags"),
		CObjects:  []string{},
		Ctx:       ctx,
		Project:   projectCfg,
	})

	// The generated C file location follows project artifact layout when available.
	cFile := source + ".c"
	if projectCfg != nil {
		cFile = projectCfg.GetArtifactPath(source, ".c")
	}

	return cFile, nil
}

// runPkgConfig executes pkg-config and returns the flags
func runPkgConfig(flag string, packages []string) ([]string, error) {
	args := append([]string{flag}, packages...)
	cmd := exec.Command("pkg-config", args...)
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	flagStr := strings.TrimSpace(string(output))
	if flagStr == "" {
		return nil, nil
	}
	return strings.Fields(flagStr), nil
}

// runPkgConfigWithFlags executes pkg-config with multiple flags (e.g., "--libs --static")
func runPkgConfigWithFlags(flags string, packages []string) ([]string, error) {
	args := append(strings.Fields(flags), packages...)
	cmd := exec.Command("pkg-config", args...)
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	flagStr := strings.TrimSpace(string(output))
	if flagStr == "" {
		return nil, nil
	}
	return strings.Fields(flagStr), nil
}

// BuildCommand compiles gecko to an executable
var BuildCommand = &cli.Command{
	Name:        "build",
	Aliases:     []string{"b"},
	Usage:       "gecko build [source.gecko] [-o output] [--entry name]",
	Description: buildHelp,
	Action: func(ctx *cli.Context) error {
		setLogLevel(ctx)

		// Try to load project config
		wd, _ := os.Getwd()
		projectCfg, cfgErr := config.LoadProjectConfig(wd)

		// Determine source to build
		var source string
		var entryName string

		if ctx.Args().Len() > 0 {
			// Explicit source provided
			source = ctx.Args().First()
		} else if cfgErr == nil && projectCfg != nil {
			// No args - use gecko.toml entries
			entryName = ctx.String("entry")

			if entryName != "" {
				// Specific entry requested
				entryPath, err := projectCfg.GetEntry(entryName)
				if err != nil {
					return fmt.Errorf("entry '%s' not found in gecko.toml", entryName)
				}
				source = entryPath
			} else if len(projectCfg.Build.Entries) > 0 {
				// Build first entry by default
				for name, path := range projectCfg.Build.Entries {
					source = filepath.Join(projectCfg.ProjectRoot, path)
					entryName = name
					fmt.Printf("Building entry '%s': %s\n", name, path)
					break
				}
			} else {
				return fmt.Errorf("no entries defined in gecko.toml and no source provided")
			}
		} else {
			fmt.Println("Usage: gecko build <source.gecko> [-o output]")
			fmt.Println("   or: gecko build --entry <name>  (with gecko.toml)")
			return nil
		}

		// Compile to C
		cFile, err := compileToC(ctx, source, projectCfg)
		if err != nil {
			return err
		}

		// Determine output name
		output := ctx.String("output")
		if output == "" {
			if entryName != "" {
				// Use entry name from gecko.toml
				output = entryName
			} else {
				// Default: source name without extension
				base := filepath.Base(source)
				output = strings.TrimSuffix(base, filepath.Ext(base))
			}
		}

		targetKey := resolveTargetKey(ctx, projectCfg)

		// Add optimization based on --release flag or profile
		var gccArgs []string
		if ctx.Bool("release") {
			gccArgs = append(gccArgs, "-O2")
		}

		// Check for static linking (CLI flag or project config)
		isStatic := ctx.Bool("static")
		if !isStatic && projectCfg != nil && projectCfg.Build.Static {
			isStatic = true
		}
		if isStatic {
			gccArgs = append(gccArgs, "-static")
		}

		// Add compile flags from CLI + project + cimport pkg-config libraries.
		cflags := ctx.StringSlice("cflags")
		pkgConfigPkgs := ctx.StringSlice("pkg-config")
		if len(pkgConfigPkgs) > 0 {
			if pkgCFlags, err := runPkgConfig("--cflags", pkgConfigPkgs); err == nil {
				cflags = append(cflags, pkgCFlags...)
			}
		}
		if projectCfg != nil {
			if projCFlags, err := projectCfg.GetCFlagsForTarget(targetKey); err == nil {
				cflags = append(cflags, projCFlags...)
			}
		}
		cflags = append(cflags, collectCImportPkgCFlags()...)
		gccArgs = append(gccArgs, dedupeStrings(cflags)...)

		// Add linker flags from CLI + project + cimport pkg-config libraries.
		ldflags := ctx.StringSlice("ldflags")
		if len(pkgConfigPkgs) > 0 {
			pkgLibsFlag := "--libs"
			if isStatic {
				pkgLibsFlag = "--libs --static"
			}
			if pkgLibs, err := runPkgConfigWithFlags(pkgLibsFlag, pkgConfigPkgs); err == nil {
				ldflags = append(ldflags, pkgLibs...)
			}
		}
		if projectCfg != nil {
			if projLdFlags, err := projectCfg.GetLdFlagsForTarget(targetKey, isStatic); err == nil {
				ldflags = append(ldflags, projLdFlags...)
			}
		}
		ldflags = append(ldflags, collectCImportPkgLibFlags(isStatic)...)

		// Compile C source and link additional object inputs.
		gccArgs = append(gccArgs, "-o", output, cFile)
		gccArgs = append(gccArgs, collectObjectInputs(projectCfg, targetKey)...)

		// Append linker flags after source/object inputs.
		gccArgs = append(gccArgs, dedupeStrings(ldflags)...)

		cmd := exec.Command("gcc", gccArgs...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			return fmt.Errorf("gcc linking failed: %w", err)
		}

		fmt.Printf("Built: %s\n", output)

		// Clean up C file unless --keep-c is set
		if !ctx.Bool("keep-c") && projectCfg == nil {
			os.Remove(cFile)
		}

		// Run post-build scripts if defined
		if projectCfg != nil && projectCfg.Build.Scripts != nil {
			for _, script := range projectCfg.Build.Scripts.PostBuild {
				// Expand variables in script
				expandedScript := os.Expand(script, func(key string) string {
					switch key {
					case "OUTPUT":
						return output
					case "PROJECT_ROOT":
						return projectCfg.ProjectRoot
					case "PACKAGE_NAME":
						return projectCfg.Package.Name
					case "PACKAGE_VERSION":
						return projectCfg.Package.Version
					default:
						return os.Getenv(key)
					}
				})

				fmt.Printf("Running post-build: %s\n", expandedScript)
				scriptCmd := exec.Command("sh", "-c", expandedScript)
				scriptCmd.Dir = projectCfg.ProjectRoot
				scriptCmd.Stdout = os.Stdout
				scriptCmd.Stderr = os.Stderr
				if err := scriptCmd.Run(); err != nil {
					return fmt.Errorf("post-build script failed: %w", err)
				}
			}
		}

		return nil
	},
	Flags: append(commonFlags,
		&cli.StringFlag{
			Name:    "output",
			Aliases: []string{"o"},
			Value:   "",
			Usage:   "Output executable name",
		},
		&cli.StringFlag{
			Name:    "entry",
			Aliases: []string{"e"},
			Value:   "",
			Usage:   "Entry point name from gecko.toml [build.entries]",
		},
		&cli.BoolFlag{
			Name:  "release",
			Value: false,
			Usage: "Build with optimizations",
		},
		&cli.BoolFlag{
			Name:  "keep-c",
			Value: false,
			Usage: "Keep generated C file",
		},
		&cli.BoolFlag{
			Name:  "ir-only",
			Value: true,
			Usage: "Only compile to IR (always true for build)",
		},
		&cli.BoolFlag{
			Name:  "static",
			Value: false,
			Usage: "Link statically (standalone binary)",
		},
	),
}

// RunCommand compiles and runs a gecko program
var RunCommand = &cli.Command{
	Name:        "run",
	Aliases:     []string{"r"},
	Usage:       "gecko run <source.gecko> [-- args...]",
	Description: runHelp,
	Action: func(ctx *cli.Context) error {
		setLogLevel(ctx)

		if ctx.Args().Len() == 0 {
			fmt.Println("Usage: gecko run <source.gecko> [-- args...]")
			return nil
		}

		source := ctx.Args().First()

		// Try to load project config
		wd, _ := os.Getwd()
		projectCfg, _ := config.LoadProjectConfig(wd)

		// Compile to C
		cFile, err := compileToC(ctx, source, projectCfg)
		if err != nil {
			return err
		}

		// Create temp executable
		tmpDir := os.TempDir()
		base := filepath.Base(source)
		exeName := strings.TrimSuffix(base, filepath.Ext(base))
		tmpExe := filepath.Join(tmpDir, "gecko_"+exeName)

		// Build gcc args with linker flags
		targetKey := resolveTargetKey(ctx, projectCfg)
		gccArgs := []string{}

		// Add compile flags from CLI + project + cimport pkg-config libraries.
		cflags := ctx.StringSlice("cflags")
		pkgConfigPkgs := ctx.StringSlice("pkg-config")
		if len(pkgConfigPkgs) > 0 {
			if pkgCFlags, err := runPkgConfig("--cflags", pkgConfigPkgs); err == nil {
				cflags = append(cflags, pkgCFlags...)
			}
		}
		if projectCfg != nil {
			if projCFlags, err := projectCfg.GetCFlagsForTarget(targetKey); err == nil {
				cflags = append(cflags, projCFlags...)
			}
		}
		cflags = append(cflags, collectCImportPkgCFlags()...)
		gccArgs = append(gccArgs, dedupeStrings(cflags)...)

		// Add linker flags from CLI + project + cimport pkg-config libraries.
		ldflags := ctx.StringSlice("ldflags")
		if len(pkgConfigPkgs) > 0 {
			if pkgLibs, err := runPkgConfig("--libs", pkgConfigPkgs); err == nil {
				ldflags = append(ldflags, pkgLibs...)
			}
		}
		if projectCfg != nil {
			if projLdFlags, err := projectCfg.GetLdFlagsForTarget(targetKey, false); err == nil {
				ldflags = append(ldflags, projLdFlags...)
			}
		}
		ldflags = append(ldflags, collectCImportPkgLibFlags(false)...)

		gccArgs = append(gccArgs, "-o", tmpExe, cFile)
		gccArgs = append(gccArgs, collectObjectInputs(projectCfg, targetKey)...)
		gccArgs = append(gccArgs, dedupeStrings(ldflags)...)

		// Compile C to executable
		cmd := exec.Command("gcc", gccArgs...)
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("gcc linking failed: %w", err)
		}

		// Keep generated C for project builds in .gecko_build; clean up ad-hoc runs.
		if projectCfg == nil {
			os.Remove(cFile)
		}

		// Run the executable
		// Get args after -- if present
		var runArgs []string
		args := ctx.Args().Slice()
		for i, arg := range args {
			if arg == "--" && i+1 < len(args) {
				runArgs = args[i+1:]
				break
			}
		}

		runCmd := exec.Command(tmpExe, runArgs...)
		runCmd.Stdout = os.Stdout
		runCmd.Stderr = os.Stderr
		runCmd.Stdin = os.Stdin

		runErr := runCmd.Run()

		// Clean up executable
		os.Remove(tmpExe)

		if runErr != nil {
			if exitErr, ok := runErr.(*exec.ExitError); ok {
				// Program exited with non-zero - propagate silently
				os.Exit(exitErr.ExitCode())
			}
			return runErr
		}

		return nil
	},
	Flags: append(commonFlags,
		&cli.BoolFlag{
			Name:  "ir-only",
			Value: true,
			Usage: "Only compile to IR (always true for run)",
		},
	),
}

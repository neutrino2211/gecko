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
	"github.com/neutrino2211/gecko/parser"
	"github.com/neutrino2211/gecko/utils"
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

func resolveTreeshakeEnabled(ctx *cli.Context, projectCfg *config.ProjectConfig) bool {
	// Explicit enable wins over all other sources, including explicit disable.
	if ctx.Bool("treeshake") {
		return true
	}
	if ctx.Bool("no-treeshake") {
		return false
	}
	if projectCfg != nil && projectCfg.Build.Treeshake != nil {
		return *projectCfg.Build.Treeshake
	}
	// v1 default: enabled.
	return true
}

func addTreeshakeCompileFlags(flags []string, enabled bool) []string {
	if !enabled {
		return flags
	}
	return append(flags, "-ffunction-sections", "-fdata-sections")
}

func treeshakeLinkerFlagsForPlatform(platform string, enabled bool) []string {
	if !enabled {
		return nil
	}
	switch platform {
	case "darwin":
		return []string{"-Wl,-dead_strip"}
	case "linux":
		return []string{"-Wl,--gc-sections"}
	default:
		return nil
	}
}

func validateStaticLinkRequest(platform string, isStatic bool) error {
	if !isStatic {
		return nil
	}
	if platform == "linux" {
		return nil
	}
	if platform == "darwin" {
		return fmt.Errorf("static linking on darwin is not supported: crt0.o not found")
	}
	return nil
}

func effectiveTargetPlatform(ctx *cli.Context) string {
	platform := strings.TrimSpace(ctx.String("target-platform"))
	if platform == "" {
		return runtime.GOOS
	}
	return platform
}

func defaultLLVMExecutableLinkFlags(platform string, isStatic bool) []string {
	flags := make([]string, 0, 1)
	// Linux toolchains often default to PIE executables. Our LLVM .o artifacts
	// may contain absolute relocations; avoid PIE link-mode unless explicitly
	// requested by user flags.
	if platform == "linux" && !isStatic {
		flags = append(flags, "-no-pie")
	}
	return flags
}



func resolveEffectiveBackend(source string, requestedBackend string) string {
	sourceContents, err := os.ReadFile(source)
	if err != nil {
		return requestedBackend
	}

	parsedFile, parseErr := parser.Parser.ParseString(source, string(sourceContents))
	if parseErr != nil || parsedFile == nil {
		return requestedBackend
	}

	if sourceBackend := parsedFile.GetBackend(); sourceBackend != "" {
		return sourceBackend
	}

	return requestedBackend
}

func collectCImportPkgCFlags() []string {
	if len(cbackend.LastCImportLibraries) == 0 {
		return nil
	}
	var flags []string
	for _, lib := range utils.DedupeStrings(cbackend.LastCImportLibraries) {
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
	for _, lib := range utils.DedupeStrings(cbackend.LastCImportLibraries) {
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
	return utils.DedupeStrings(objects)
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
			treeshakeEnabled := resolveTreeshakeEnabled(ctx, projectCfg)
			outFile := compiler.Compile(pos, &config.CompileCfg{
				Arch:      ctx.String("target-arch"),
				Platform:  ctx.String("target-platform"),
				Vendor:    ctx.String("target-vendor"),
				TargetKey: targetKey,
				Treeshake: treeshakeEnabled,
				CFlags:    []string{},
				CLFlags:   []string{},
				CObjects:  []string{},
				Ctx:       ctx,
				Project:   projectCfg,
			})

			if outFile != "" {
				ext := filepath.Ext(outFile)
				if ext == "" {
					return fmt.Errorf("unknown artifact extension for %s", outFile)
				}
				destObj := pos + ext
				if projectCfg != nil {
					destObj = projectCfg.GetArtifactPath(pos, ext)
				}
				if err := os.MkdirAll(filepath.Dir(destObj), 0o755); err != nil {
					return fmt.Errorf("failed creating artifact directory for %s: %w", destObj, err)
				}

				srcAbs, srcErr := filepath.Abs(outFile)
				dstAbs, dstErr := filepath.Abs(destObj)
				if srcErr == nil && dstErr == nil && srcAbs == dstAbs {
					continue
				}

				if err := CopyFile(outFile, destObj); err != nil {
					return fmt.Errorf("failed copying backend artifact from %s to %s: %w", outFile, destObj, err)
				}
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
		&cli.BoolFlag{
			Name:  "treeshake",
			Value: false,
			Usage: "Enable treeshake (default behavior when not overridden)",
		},
		&cli.BoolFlag{
			Name:  "no-treeshake",
			Value: false,
			Usage: "Disable treeshake",
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
	compileHelp  = `compiles a gecko source file to backend artifacts`
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
		Name:  "llc-args",
		Usage: "Pass arguments to underlying llc command (LLVM backend)",
	},
	&cli.StringSliceFlag{
		Name:  "pkg-config",
		Usage: "pkg-config packages to include (can be specified multiple times)",
	},
	&cli.BoolFlag{
		Name:  "treeshake",
		Value: false,
		Usage: "Enable treeshake (default behavior when not overridden)",
	},
	&cli.BoolFlag{
		Name:  "no-treeshake",
		Value: false,
		Usage: "Disable treeshake",
	},
}

// compileToC compiles a Gecko file for the C backend and returns the generated C artifact path.
func compileToC(ctx *cli.Context, source string, projectCfg *config.ProjectConfig) (string, bool, error) {
	targetKey := resolveTargetKey(ctx, projectCfg)
	treeshakeEnabled := resolveTreeshakeEnabled(ctx, projectCfg)

	// Collect CFlags from CLI
	cflags := ctx.StringSlice("cflags")

	// Add pkg-config --cflags if specified via CLI
	pkgConfigPkgs := ctx.StringSlice("pkg-config")
	if len(pkgConfigPkgs) > 0 {
		if pkgCFlags, err := runPkgConfig("--cflags", pkgConfigPkgs); err == nil {
			cflags = append(cflags, pkgCFlags...)
		}
	}

	// Compile to C/IR first. Empty result means compilation failed.
	compileCfg := &config.CompileCfg{
		Arch:      ctx.String("target-arch"),
		Platform:  ctx.String("target-platform"),
		Vendor:    ctx.String("target-vendor"),
		TargetKey: targetKey,
		Treeshake: treeshakeEnabled,
		CFlags:    cflags,
		CLFlags:   ctx.StringSlice("ldflags"),
		CObjects:  []string{},
		Ctx:       ctx,
		Project:   projectCfg,
	}
	compiled := compiler.Compile(source, compileCfg)
	if compiled == "" {
		return "", treeshakeEnabled, fmt.Errorf("compilation failed for %s", source)
	}

	if cbackend.LastTreeshakeAutoDisabled {
		treeshakeEnabled = false
	}

	if filepath.Ext(compiled) != ".c" {
		return "", treeshakeEnabled, fmt.Errorf("expected C backend artifact to be .c, got %s", compiled)
	}
	if _, err := os.Stat(compiled); err != nil {
		return "", treeshakeEnabled, fmt.Errorf("generated C file not found: %s", compiled)
	}

	return compiled, treeshakeEnabled, nil
}

func compileToLLVMObject(ctx *cli.Context, source string, projectCfg *config.ProjectConfig) (string, error) {
	if _, err := exec.LookPath("llc"); err != nil {
		return "", fmt.Errorf("LLVM toolchain error: llc not found in PATH")
	}

	targetKey := resolveTargetKey(ctx, projectCfg)
	treeshakeEnabled := resolveTreeshakeEnabled(ctx, projectCfg)

	originalIrOnly := ctx.Bool("ir-only")
	_ = ctx.Set("ir-only", "false")
	defer func() {
		if originalIrOnly {
			_ = ctx.Set("ir-only", "true")
			return
		}
		_ = ctx.Set("ir-only", "false")
	}()

	compiled := compiler.Compile(source, &config.CompileCfg{
		Arch:      ctx.String("target-arch"),
		Platform:  ctx.String("target-platform"),
		Vendor:    ctx.String("target-vendor"),
		TargetKey: targetKey,
		Treeshake: treeshakeEnabled,
		CFlags:    []string{},
		CLFlags:   ctx.StringSlice("ldflags"),
		CObjects:  []string{},
		Ctx:       ctx,
		Project:   projectCfg,
	})
	if compiled == "" {
		return "", fmt.Errorf("LLVM compilation failed for %s", source)
	}

	if filepath.Ext(compiled) != ".o" {
		return "", fmt.Errorf("expected LLVM backend artifact to be .o, got %s", compiled)
	}
	if _, err := os.Stat(compiled); err != nil {
		return "", fmt.Errorf("generated LLVM object not found: %s", compiled)
	}

	return compiled, nil
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

func runBuildScripts(projectCfg *config.ProjectConfig, scripts []string, stage string, output string) error {
	if projectCfg == nil || len(scripts) == 0 {
		return nil
	}

	for _, script := range scripts {
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

		fmt.Printf("Running %s: %s\n", stage, expandedScript)
		scriptCmd := exec.Command("sh", "-c", expandedScript)
		scriptCmd.Dir = projectCfg.ProjectRoot
		scriptCmd.Stdout = os.Stdout
		scriptCmd.Stderr = os.Stderr
		if err := scriptCmd.Run(); err != nil {
			return fmt.Errorf("%s script failed: %w", stage, err)
		}
	}

	return nil
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

		if projectCfg != nil && projectCfg.Build.Scripts != nil {
			if err := runBuildScripts(projectCfg, projectCfg.Build.Scripts.PreBuild, "pre-build", output); err != nil {
				return err
			}
		}

		backend := resolveEffectiveBackend(source, ctx.String("backend"))
		targetKey := resolveTargetKey(ctx, projectCfg)
		pkgConfigPkgs := ctx.StringSlice("pkg-config")

		isStatic := ctx.Bool("static")
		if !isStatic && projectCfg != nil && projectCfg.Build.Static {
			isStatic = true
		}
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

		if backend == "llvm" {
			if _, err := exec.LookPath("clang"); err != nil {
				return fmt.Errorf("LLVM toolchain error: clang not found in PATH")
			}

			objFile, err := compileToLLVMObject(ctx, source, projectCfg)
			if err != nil {
				return err
			}
			treeshakeEnabled := resolveTreeshakeEnabled(ctx, projectCfg)
			ldflags = append(ldflags, treeshakeLinkerFlagsForPlatform(effectiveTargetPlatform(ctx), treeshakeEnabled)...)

			clangArgs := []string{}
			if ctx.Bool("release") {
				clangArgs = append(clangArgs, "-O2")
			}
			if isStatic {
				clangArgs = append(clangArgs, "-static")
			}
			clangArgs = append(clangArgs, defaultLLVMExecutableLinkFlags(effectiveTargetPlatform(ctx), isStatic)...)

			clangArgs = append(clangArgs, "-o", output, objFile)
			clangArgs = append(clangArgs, collectObjectInputs(projectCfg, targetKey)...)
			clangArgs = append(clangArgs, ldflags...)

			cmd := exec.Command("clang", clangArgs...)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				return fmt.Errorf("clang linking failed: %w", err)
			}

			fmt.Printf("Built: %s\n", output)

			if projectCfg != nil && projectCfg.Build.Scripts != nil {
				if err := runBuildScripts(projectCfg, projectCfg.Build.Scripts.PostBuild, "post-build", output); err != nil {
					return err
				}
			}
			return nil
		}

		// Compile to C
		cFile, treeshakeEnabled, err := compileToC(ctx, source, projectCfg)
		if err != nil {
			return err
		}

		ldflags = append(ldflags, treeshakeLinkerFlagsForPlatform(effectiveTargetPlatform(ctx), treeshakeEnabled)...)

		// Add optimization based on --release flag or profile
		var gccArgs []string
		if ctx.Bool("release") {
			gccArgs = append(gccArgs, "-O2")
		}
		if isStatic {
			gccArgs = append(gccArgs, "-static")
		}

		// Add compile flags from CLI + project + cimport pkg-config libraries.
		cflags := ctx.StringSlice("cflags")
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
		cflags = addTreeshakeCompileFlags(cflags, treeshakeEnabled)
		gccArgs = append(gccArgs, cflags...)

		// Compile C source and link additional object inputs.
		gccArgs = append(gccArgs, "-o", output, cFile)
		gccArgs = append(gccArgs, collectObjectInputs(projectCfg, targetKey)...)

		// Append linker flags after source/object inputs.
		gccArgs = append(gccArgs, ldflags...)

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

		if projectCfg != nil && projectCfg.Build.Scripts != nil {
			if err := runBuildScripts(projectCfg, projectCfg.Build.Scripts.PostBuild, "post-build", output); err != nil {
				return err
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
			Usage: "Internal C-backend IR mode (ignored for llvm build)",
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

		// Create temp executable in an isolated subdirectory.
		// Some GUI runtimes (e.g. WebKit) expect to create
		// $TMPDIR/<process_name>/... directories at startup.
		// Avoid placing the executable directly at that path.
		tmpDir := os.TempDir()
		base := filepath.Base(source)
		exeName := strings.TrimSuffix(base, filepath.Ext(base))
		tmpRunDir, err := os.MkdirTemp(tmpDir, "gecko-run-")
		if err != nil {
			return fmt.Errorf("failed to create temp run directory: %w", err)
		}
		defer os.RemoveAll(tmpRunDir)
		tmpExe := filepath.Join(tmpRunDir, "gecko_"+exeName)
		backend := resolveEffectiveBackend(source, ctx.String("backend"))
		targetKey := resolveTargetKey(ctx, projectCfg)
		pkgConfigPkgs := ctx.StringSlice("pkg-config")
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

		if backend == "llvm" {
			if _, err := exec.LookPath("clang"); err != nil {
				return fmt.Errorf("LLVM toolchain error: clang not found in PATH")
			}

			objFile, err := compileToLLVMObject(ctx, source, projectCfg)
			if err != nil {
				return err
			}
			treeshakeEnabled := resolveTreeshakeEnabled(ctx, projectCfg)
			ldflags = append(ldflags, treeshakeLinkerFlagsForPlatform(effectiveTargetPlatform(ctx), treeshakeEnabled)...)

			clangArgs := []string{"-o", tmpExe, objFile}
			clangArgs = append(clangArgs, defaultLLVMExecutableLinkFlags(effectiveTargetPlatform(ctx), false)...)
			clangArgs = append(clangArgs, collectObjectInputs(projectCfg, targetKey)...)
			clangArgs = append(clangArgs, ldflags...)

			cmd := exec.Command("clang", clangArgs...)
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				return fmt.Errorf("clang linking failed: %w", err)
			}
		} else {
			// Compile to C
			cFile, treeshakeEnabled, err := compileToC(ctx, source, projectCfg)
			if err != nil {
				return err
			}

			ldflags = append(ldflags, treeshakeLinkerFlagsForPlatform(effectiveTargetPlatform(ctx), treeshakeEnabled)...)

			// Build gcc args with linker flags
			gccArgs := []string{}

			// Add compile flags from CLI + project + cimport pkg-config libraries.
			cflags := ctx.StringSlice("cflags")
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
			cflags = addTreeshakeCompileFlags(cflags, treeshakeEnabled)
			gccArgs = append(gccArgs, cflags...)

			gccArgs = append(gccArgs, "-o", tmpExe, cFile)
			gccArgs = append(gccArgs, collectObjectInputs(projectCfg, targetKey)...)
			gccArgs = append(gccArgs, ldflags...)

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

		if runErr != nil {
			if exitErr, ok := runErr.(*exec.ExitError); ok {
				// os.Exit skips defers; clean temp run directory explicitly.
				_ = os.RemoveAll(tmpRunDir)
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
			Usage: "Internal C-backend IR mode (ignored for llvm run)",
		},
	),
}

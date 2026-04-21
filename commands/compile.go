package commands

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

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
				Arch:     ctx.String("target-arch"),
				Platform: ctx.String("target-platform"),
				Vendor:   ctx.String("target-vendor"),
				CFlags:   []string{},
				CLFlags:  []string{},
				CObjects: []string{},
				Ctx:      ctx,
				Project:  projectCfg,
			})

			if outFile != "" {
				CopyFile(outFile, pos+".o")
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
		Arch:     ctx.String("target-arch"),
		Platform: ctx.String("target-platform"),
		Vendor:   ctx.String("target-vendor"),
		CFlags:   cflags,
		CLFlags:  ctx.StringSlice("ldflags"),
		CObjects: []string{},
		Ctx:      ctx,
		Project:  projectCfg,
	})

	// The C file is generated next to the source
	sourceDir := filepath.Dir(source)
	sourceName := filepath.Base(source)
	cFile := filepath.Join(sourceDir, sourceName+".c")

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

		// Compile C to executable with gcc
		gccArgs := []string{"-o", output, cFile}

		// Add optimization based on --release flag or profile
		if ctx.Bool("release") {
			gccArgs = append([]string{"-O2"}, gccArgs...)
		}

		// Add linker flags from CLI
		ldflags := ctx.StringSlice("ldflags")

		// Add pkg-config --libs if specified via CLI
		pkgConfigPkgs := ctx.StringSlice("pkg-config")
		if len(pkgConfigPkgs) > 0 {
			if pkgLibs, err := runPkgConfig("--libs", pkgConfigPkgs); err == nil {
				ldflags = append(ldflags, pkgLibs...)
			}
		}

		// Add linker flags from project config
		if projectCfg != nil {
			if projLdFlags, err := projectCfg.GetLdFlags(); err == nil {
				ldflags = append(ldflags, projLdFlags...)
			}
		}

		// Append linker flags after the source file
		gccArgs = append(gccArgs, ldflags...)

		cmd := exec.Command("gcc", gccArgs...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			return fmt.Errorf("gcc linking failed: %w", err)
		}

		fmt.Printf("Built: %s\n", output)

		// Clean up C file unless --keep-c is set
		if !ctx.Bool("keep-c") {
			os.Remove(cFile)
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
		gccArgs := []string{"-o", tmpExe, cFile}

		// Add linker flags from CLI
		ldflags := ctx.StringSlice("ldflags")

		// Add pkg-config --libs if specified via CLI
		pkgConfigPkgs := ctx.StringSlice("pkg-config")
		if len(pkgConfigPkgs) > 0 {
			if pkgLibs, err := runPkgConfig("--libs", pkgConfigPkgs); err == nil {
				ldflags = append(ldflags, pkgLibs...)
			}
		}

		// Add linker flags from project config
		if projectCfg != nil {
			if projLdFlags, err := projectCfg.GetLdFlags(); err == nil {
				ldflags = append(ldflags, projLdFlags...)
			}
		}

		gccArgs = append(gccArgs, ldflags...)

		// Compile C to executable
		cmd := exec.Command("gcc", gccArgs...)
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("gcc linking failed: %w", err)
		}

		// Clean up C file
		os.Remove(cFile)

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

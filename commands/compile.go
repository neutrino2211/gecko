// spec: spec/modules.md, spec/c-interop.md, spec/stdlib.md

package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/fatih/color"
	cbackend "github.com/neutrino2211/gecko/backends/c_backend"
	"github.com/neutrino2211/gecko/compiler"
	"github.com/neutrino2211/gecko/config"
	"github.com/neutrino2211/gecko/logger"
	"github.com/urfave/cli/v2"
)

func setLogLevel(ctx *cli.Context) {
	level := ctx.String("log-level")
	logger.SetLogLevel(logger.ParseLogLevel(level))
}

var CompileCommand = &cli.Command{
	Name:        "compile",
	Aliases:     []string{"c"},
	Usage:       "gecko compile [sources...] [-o output] or gecko compile --entry <name>",
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

		outputPath := strings.TrimSpace(ctx.String("output"))
		if outputPath != "" && len(sources) != 1 {
			return fmt.Errorf("--output requires exactly one source file, got %d", len(sources))
		}

		failed := false
		for _, pos := range sources {
			treeshakeEnabled := resolveTreeshakeEnabled(ctx, projectCfg)
			outFile := compiler.Compile(pos, &config.CompileCfg{
				Arch:         ctx.String("target-arch"),
				Platform:     ctx.String("target-platform"),
				Vendor:       ctx.String("target-vendor"),
				TargetKey:    targetKey,
				Treeshake:    treeshakeEnabled,
				ManualMemory: ctx.Bool("no-auto-drop"),
				CFlags:       []string{},
				CLFlags:      []string{},
				CObjects:     []string{},
				Ctx:          ctx,
				Project:      projectCfg,
			})

			if outFile == "" {
				failed = true
				continue
			}

			ext := filepath.Ext(outFile)
			if ext == "" {
				return fmt.Errorf("unknown artifact extension for %s", outFile)
			}
			destObj := pos + ext
			if projectCfg != nil {
				destObj = projectCfg.GetArtifactPath(pos, ext)
			}
			if outputPath != "" {
				destObj = outputPath
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

		hasErrors := compiler.PrintErrorSummary()
		if failed || hasErrors {
			return fmt.Errorf("compilation failed")
		}

		return nil
	},
	Flags: []cli.Flag{
		&cli.BoolFlag{Name: "no-auto-drop", Usage: "Disable automatic Drop hooks; explicit drops and defer still run"},
		&cli.StringFlag{
			Name:    "output",
			Aliases: []string{"o"},
			Value:   "",
			Usage:   "Output artifact path (single source only)",
		},
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
			Usage: "The compilation backend to use (c)",
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
			Usage: "Print generated backend IR (C for the C backend)",
		},
		&cli.BoolFlag{Name: "print-expanded", Usage: "Print generated C with ownership transfers and Drop calls"},
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
	&cli.BoolFlag{Name: "no-auto-drop", Usage: "Disable automatic Drop hooks; explicit drops and defer still run"},
	&cli.BoolFlag{Name: "print-expanded", Usage: "Print generated C with ownership transfers and Drop calls"},
	&cli.StringFlag{
		Name:  "backend",
		Value: "c",
		Usage: "The compilation backend to use (c)",
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
		Arch:         ctx.String("target-arch"),
		Platform:     ctx.String("target-platform"),
		Vendor:       ctx.String("target-vendor"),
		TargetKey:    targetKey,
		Treeshake:    treeshakeEnabled,
		ManualMemory: ctx.Bool("no-auto-drop"),
		CFlags:       cflags,
		CLFlags:      ctx.StringSlice("ldflags"),
		CObjects:     []string{},
		Ctx:          ctx,
		Project:      projectCfg,
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

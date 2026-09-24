// spec: spec/modules.md, spec/c-interop.md, spec/stdlib.md

package commands

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/neutrino2211/gecko/config"
	"github.com/urfave/cli/v2"
)

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
		ldflags = append(ldflags, collectNativePkgLibFlags(isStatic)...)

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
		cflags = append(cflags, collectNativePkgCFlags()...)
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
			Usage: "Internal C-backend IR mode",
		},
		&cli.BoolFlag{
			Name:  "static",
			Value: false,
			Usage: "Link statically (standalone binary)",
		},
	),
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

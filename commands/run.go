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
		ldflags = append(ldflags, collectNativePkgLibFlags(false)...)

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
		cflags = append(cflags, collectNativePkgCFlags()...)
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
			Usage: "Internal C-backend IR mode",
		},
	),
}

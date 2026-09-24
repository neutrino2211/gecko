// spec: spec/modules.md, spec/c-interop.md, spec/stdlib.md

package commands

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/neutrino2211/gecko/compiler"
	"github.com/neutrino2211/gecko/config"
	"github.com/neutrino2211/gecko/utils"
	"github.com/urfave/cli/v2"
)

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

func collectNativePkgCFlags() []string {
	if len(compiler.LastNativeLibraries) == 0 {
		return nil
	}
	var flags []string
	for _, lib := range utils.DedupeStrings(compiler.LastNativeLibraries) {
		if pkgCFlags, err := runPkgConfig("--cflags", []string{lib}); err == nil {
			flags = append(flags, pkgCFlags...)
		}
	}
	return flags
}

func collectNativePkgLibFlags(staticLink bool) []string {
	if len(compiler.LastNativeLibraries) == 0 {
		return nil
	}
	pkgFlag := "--libs"
	if staticLink {
		pkgFlag = "--libs --static"
	}

	var flags []string
	for _, lib := range utils.DedupeStrings(compiler.LastNativeLibraries) {
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
	objects = append(objects, compiler.LastNativeObjects...)
	return utils.DedupeStrings(objects)
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

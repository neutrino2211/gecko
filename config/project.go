// spec: spec/modules.md

package config

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

// ProjectConfig represents the root gecko.toml configuration
type ProjectConfig struct {
	Package      PackageConfig            `toml:"package"`
	Build        BuildConfig              `toml:"build"`
	Targets      map[string]*TargetConfig `toml:"target"`
	Dependencies map[string]*Dependency   `toml:"dependencies"`

	// Runtime fields (not from TOML)
	ProjectRoot string `toml:"-"`
	ConfigPath  string `toml:"-"`
}

// PackageConfig holds package metadata
type PackageConfig struct {
	Name    string `toml:"name"`
	Version string `toml:"version"`
}

// BuildConfig holds build settings
type BuildConfig struct {
	Backend       string                   `toml:"backend"`
	Treeshake     *bool                    `toml:"treeshake"` // Optional; default-enabled when unset
	DefaultTarget string                   `toml:"default_target"`
	Entries       map[string]string        `toml:"entries"`
	Profiles      map[string]*BuildProfile `toml:"profiles"`
	PkgConfig     []string                 `toml:"pkg_config"` // pkg-config packages to include
	CFlags        []string                 `toml:"cflags"`     // Additional C compiler flags
	LdFlags       []string                 `toml:"ldflags"`    // Additional linker flags
	Static        bool                     `toml:"static"`     // Link statically
	Native        *NativeConfig            `toml:"native"`     // Native C ABI integration settings
	Scripts       *BuildScripts            `toml:"scripts"`    // Build scripts
}

// BuildScripts holds pre/post build scripts
type BuildScripts struct {
	PreBuild  []string `toml:"pre_build"`  // Commands to run before build
	PostBuild []string `toml:"post_build"` // Commands to run after build
}

// BuildProfile holds profile-specific build settings
type BuildProfile struct {
	Optimize interface{} `toml:"optimize"` // bool or string ("size")
}

// TargetConfig holds target-specific settings
type TargetConfig struct {
	Freestanding bool          `toml:"freestanding"`
	LinkerScript string        `toml:"linker_script"`
	Native       *NativeConfig `toml:"native"` // Target-specific native overrides
}

// NativeConfig holds C ABI integration settings for headers/libraries/objects.
type NativeConfig struct {
	Headers     []string `toml:"headers"`
	PkgConfig   []string `toml:"pkg_config"`
	IncludeDirs []string `toml:"include_dirs"`
	LibDirs     []string `toml:"lib_dirs"`
	Libs        []string `toml:"libs"`
	Objects     []string `toml:"objects"`
	CFlags      []string `toml:"cflags"`
	LdFlags     []string `toml:"ldflags"`
}

// Dependency represents a project dependency
type Dependency struct {
	Git    string `toml:"git"`
	Path   string `toml:"path"`
	Tag    string `toml:"tag"`
	Branch string `toml:"branch"`
	Commit string `toml:"commit"`
}

// LoadProjectConfig loads gecko.toml from the given directory or searches up
func LoadProjectConfig(startDir string) (*ProjectConfig, error) {
	configPath, err := findConfigFile(startDir)
	if err != nil {
		return nil, err
	}

	return LoadProjectConfigFromFile(configPath)
}

// LoadProjectConfigFromFile loads gecko.toml from a specific path
func LoadProjectConfigFromFile(configPath string) (*ProjectConfig, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config ProjectConfig
	if err := toml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse gecko.toml: %w", err)
	}

	config.ConfigPath = configPath
	config.ProjectRoot = filepath.Dir(configPath)

	// Set defaults
	if config.Build.Backend == "" {
		config.Build.Backend = "c"
	}

	return &config, nil
}

// findConfigFile searches for gecko.toml starting from dir and going up
func findConfigFile(startDir string) (string, error) {
	dir, err := filepath.Abs(startDir)
	if err != nil {
		return "", err
	}

	for {
		configPath := filepath.Join(dir, "gecko.toml")
		if _, err := os.Stat(configPath); err == nil {
			return configPath, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return "", fmt.Errorf("gecko.toml not found in %s or any parent directory", startDir)
}

// GetEntry returns the entry point file for the given entry name
func (c *ProjectConfig) GetEntry(name string) (string, error) {
	if c.Build.Entries == nil || len(c.Build.Entries) == 0 {
		return "", fmt.Errorf("no entries defined in gecko.toml")
	}

	if name == "" {
		// Return first entry if no name specified
		for _, entry := range c.Build.Entries {
			return filepath.Join(c.ProjectRoot, entry), nil
		}
	}

	entry, ok := c.Build.Entries[name]
	if !ok {
		return "", fmt.Errorf("entry '%s' not found in gecko.toml", name)
	}

	return filepath.Join(c.ProjectRoot, entry), nil
}

// GetTarget returns the target config for the given target triple
func (c *ProjectConfig) GetTarget(name string) (*TargetConfig, error) {
	if name == "" {
		name = c.Build.DefaultTarget
	}

	if name == "" {
		// No specific target - return empty config (use defaults)
		return &TargetConfig{}, nil
	}

	// Check if we have specific overrides for this target
	if c.Targets != nil {
		if target, ok := c.Targets[name]; ok {
			return target, nil
		}
	}

	// Return empty config - target triple will be used but no special config
	return &TargetConfig{}, nil
}

// GetProfile returns the build profile settings
func (c *ProjectConfig) GetProfile(name string) *BuildProfile {
	if name == "" {
		name = "debug"
	}

	if c.Build.Profiles == nil {
		return &BuildProfile{Optimize: false}
	}

	profile, ok := c.Build.Profiles[name]
	if !ok {
		return &BuildProfile{Optimize: false}
	}

	return profile
}

// GetDepsDir returns the path to the dependencies directory
func (c *ProjectConfig) GetDepsDir() string {
	return filepath.Join(c.ProjectRoot, ".gecko", "deps")
}

// GetBuildArtifactsDir returns the path to the build artifacts directory.
func (c *ProjectConfig) GetBuildArtifactsDir() string {
	return filepath.Join(c.ProjectRoot, ".gecko_build")
}

// GetArtifactPath returns a deterministic artifact path for a source file.
// Artifacts are rooted under .gecko_build and preserve the source-relative path.
func (c *ProjectConfig) GetArtifactPath(sourcePath string, suffix string) string {
	if c == nil || c.ProjectRoot == "" {
		return sourcePath + suffix
	}

	absSource, err := filepath.Abs(sourcePath)
	if err != nil {
		absSource = sourcePath
	}

	rel, relErr := filepath.Rel(c.ProjectRoot, absSource)
	if relErr != nil || rel == "." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || rel == ".." {
		rel = filepath.Base(absSource)
	}

	rel = filepath.Clean(rel)
	return filepath.Join(c.GetBuildArtifactsDir(), rel+suffix)
}

// GetModuleSearchPaths returns all paths to search for module imports
func (c *ProjectConfig) GetModuleSearchPaths() []string {
	paths := []string{
		c.ProjectRoot,
		c.GetDepsDir(),
	}

	// Add $GECKO_HOME/std if set
	if geckoHome := os.Getenv("GECKO_HOME"); geckoHome != "" {
		paths = append(paths, filepath.Join(geckoHome, "std"))
	}

	return paths
}

// GetCFlags returns C compiler flags for the default target.
func (c *ProjectConfig) GetCFlags() ([]string, error) {
	return c.GetCFlagsForTarget("")
}

// GetCFlagsForTarget returns all C compiler flags for a specific target.
func (c *ProjectConfig) GetCFlagsForTarget(target string) ([]string, error) {
	var flags []string

	// Base build-level C flags.
	flags = append(flags, c.Build.CFlags...)

	// Native include dirs and extra C flags.
	if native := c.mergedNativeConfig(target); native != nil {
		for _, includeDir := range native.IncludeDirs {
			includeDir = strings.TrimSpace(includeDir)
			if includeDir == "" {
				continue
			}
			flags = append(flags, "-I"+c.resolveProjectPath(includeDir))
		}
		flags = append(flags, native.CFlags...)
	}

	// pkg-config flags (build + native).
	pkgPackages := append([]string{}, c.Build.PkgConfig...)
	if native := c.mergedNativeConfig(target); native != nil {
		pkgPackages = append(pkgPackages, native.PkgConfig...)
	}
	pkgPackages = dedupeStrings(pkgPackages)
	if len(pkgPackages) > 0 {
		pkgFlags, err := runPkgConfig("--cflags", pkgPackages)
		if err != nil {
			return nil, err
		}
		flags = append(flags, pkgFlags...)
	}

	return flags, nil
}

// GetLdFlags returns linker flags for the default target.
func (c *ProjectConfig) GetLdFlags() ([]string, error) {
	return c.GetLdFlagsForTarget("", false)
}

// GetLdFlagsForTarget returns all linker flags for a specific target.
func (c *ProjectConfig) GetLdFlagsForTarget(target string, staticLink bool) ([]string, error) {
	var flags []string

	// Base build-level linker flags.
	flags = append(flags, c.Build.LdFlags...)

	// Native linker settings.
	if native := c.mergedNativeConfig(target); native != nil {
		for _, libDir := range native.LibDirs {
			libDir = strings.TrimSpace(libDir)
			if libDir == "" {
				continue
			}
			flags = append(flags, "-L"+c.resolveProjectPath(libDir))
		}
		for _, lib := range native.Libs {
			lib = strings.TrimSpace(lib)
			if lib == "" {
				continue
			}
			if strings.HasPrefix(lib, "-") || strings.Contains(lib, "/") {
				flags = append(flags, lib)
			} else {
				flags = append(flags, "-l"+lib)
			}
		}
		flags = append(flags, native.LdFlags...)
	}

	// pkg-config flags (build + native).
	pkgPackages := append([]string{}, c.Build.PkgConfig...)
	if native := c.mergedNativeConfig(target); native != nil {
		pkgPackages = append(pkgPackages, native.PkgConfig...)
	}
	pkgPackages = dedupeStrings(pkgPackages)
	if len(pkgPackages) > 0 {
		pkgFlag := "--libs"
		if staticLink {
			pkgFlag = "--libs --static"
		}
		pkgFlags, err := runPkgConfigWithFlags(pkgFlag, pkgPackages)
		if err != nil {
			return nil, err
		}
		flags = append(flags, pkgFlags...)
	}

	return flags, nil
}

// GetNativeHeadersForTarget returns include directives configured in gecko.toml.
func (c *ProjectConfig) GetNativeHeadersForTarget(target string) []string {
	native := c.mergedNativeConfig(target)
	if native == nil {
		return nil
	}

	var includes []string
	for _, header := range native.Headers {
		header = strings.TrimSpace(header)
		if header == "" {
			continue
		}

		switch {
		case strings.HasPrefix(header, "#include "):
			includes = append(includes, header)
		case strings.HasPrefix(header, "<") && strings.HasSuffix(header, ">"):
			includes = append(includes, "#include "+header)
		case strings.HasPrefix(header, "\"") && strings.HasSuffix(header, "\""):
			includes = append(includes, "#include "+header)
		default:
			includes = append(includes, "#include \""+header+"\"")
		}
	}

	return dedupeStrings(includes)
}

// GetNativeObjectsForTarget returns object file paths configured in gecko.toml.
func (c *ProjectConfig) GetNativeObjectsForTarget(target string) []string {
	native := c.mergedNativeConfig(target)
	if native == nil {
		return nil
	}

	var objects []string
	for _, obj := range native.Objects {
		obj = strings.TrimSpace(obj)
		if obj == "" {
			continue
		}
		objects = append(objects, c.resolveProjectPath(obj))
	}

	return dedupeStrings(objects)
}

func (c *ProjectConfig) mergedNativeConfig(target string) *NativeConfig {
	var merged NativeConfig

	if c.Build.Native != nil {
		merged.Headers = append(merged.Headers, c.Build.Native.Headers...)
		merged.PkgConfig = append(merged.PkgConfig, c.Build.Native.PkgConfig...)
		merged.IncludeDirs = append(merged.IncludeDirs, c.Build.Native.IncludeDirs...)
		merged.LibDirs = append(merged.LibDirs, c.Build.Native.LibDirs...)
		merged.Libs = append(merged.Libs, c.Build.Native.Libs...)
		merged.Objects = append(merged.Objects, c.Build.Native.Objects...)
		merged.CFlags = append(merged.CFlags, c.Build.Native.CFlags...)
		merged.LdFlags = append(merged.LdFlags, c.Build.Native.LdFlags...)
	}

	if target == "" {
		target = c.Build.DefaultTarget
	}
	if target != "" && c.Targets != nil {
		if targetCfg, ok := c.Targets[target]; ok && targetCfg != nil && targetCfg.Native != nil {
			merged.Headers = append(merged.Headers, targetCfg.Native.Headers...)
			merged.PkgConfig = append(merged.PkgConfig, targetCfg.Native.PkgConfig...)
			merged.IncludeDirs = append(merged.IncludeDirs, targetCfg.Native.IncludeDirs...)
			merged.LibDirs = append(merged.LibDirs, targetCfg.Native.LibDirs...)
			merged.Libs = append(merged.Libs, targetCfg.Native.Libs...)
			merged.Objects = append(merged.Objects, targetCfg.Native.Objects...)
			merged.CFlags = append(merged.CFlags, targetCfg.Native.CFlags...)
			merged.LdFlags = append(merged.LdFlags, targetCfg.Native.LdFlags...)
		}
	}

	if len(merged.Headers) == 0 &&
		len(merged.PkgConfig) == 0 &&
		len(merged.IncludeDirs) == 0 &&
		len(merged.LibDirs) == 0 &&
		len(merged.Libs) == 0 &&
		len(merged.Objects) == 0 &&
		len(merged.CFlags) == 0 &&
		len(merged.LdFlags) == 0 {
		return nil
	}

	return &merged
}

func (c *ProjectConfig) resolveProjectPath(path string) string {
	if path == "" || filepath.IsAbs(path) || strings.HasPrefix(path, "$") {
		return path
	}
	return filepath.Join(c.ProjectRoot, path)
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

// runPkgConfig executes pkg-config with the given flag and packages.
func runPkgConfig(flag string, packages []string) ([]string, error) {
	args := append([]string{flag}, packages...)
	cmd := exec.Command("pkg-config", args...)
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("pkg-config %s %v failed: %s", flag, packages, string(exitErr.Stderr))
		}
		return nil, fmt.Errorf("pkg-config not found or failed: %w", err)
	}

	// Split output into individual flags
	flagStr := strings.TrimSpace(string(output))
	if flagStr == "" {
		return nil, nil
	}

	return strings.Fields(flagStr), nil
}

// runPkgConfigWithFlags executes pkg-config with multiple flags.
func runPkgConfigWithFlags(flags string, packages []string) ([]string, error) {
	args := append(strings.Fields(flags), packages...)
	cmd := exec.Command("pkg-config", args...)
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("pkg-config %s %v failed: %s", flags, packages, string(exitErr.Stderr))
		}
		return nil, fmt.Errorf("pkg-config not found or failed: %w", err)
	}
	flagStr := strings.TrimSpace(string(output))
	if flagStr == "" {
		return nil, nil
	}
	return strings.Fields(flagStr), nil
}

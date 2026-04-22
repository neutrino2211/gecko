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
	DefaultTarget string                   `toml:"default_target"`
	Entries       map[string]string        `toml:"entries"`
	Profiles      map[string]*BuildProfile `toml:"profiles"`
	PkgConfig     []string                 `toml:"pkg_config"` // pkg-config packages to include
	CFlags        []string                 `toml:"cflags"`     // Additional C compiler flags
	LdFlags       []string                 `toml:"ldflags"`    // Additional linker flags
	Static        bool                     `toml:"static"`     // Link statically
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
	Freestanding bool   `toml:"freestanding"`
	LinkerScript string `toml:"linker_script"`
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

// GetCFlags returns all C compiler flags including those from pkg-config
func (c *ProjectConfig) GetCFlags() ([]string, error) {
	var flags []string

	// Add explicit cflags
	flags = append(flags, c.Build.CFlags...)

	// Add pkg-config --cflags
	if len(c.Build.PkgConfig) > 0 {
		pkgFlags, err := runPkgConfig("--cflags", c.Build.PkgConfig)
		if err != nil {
			return nil, err
		}
		flags = append(flags, pkgFlags...)
	}

	return flags, nil
}

// GetLdFlags returns all linker flags including those from pkg-config
func (c *ProjectConfig) GetLdFlags() ([]string, error) {
	var flags []string

	// Add explicit ldflags
	flags = append(flags, c.Build.LdFlags...)

	// Add pkg-config --libs
	if len(c.Build.PkgConfig) > 0 {
		pkgFlags, err := runPkgConfig("--libs", c.Build.PkgConfig)
		if err != nil {
			return nil, err
		}
		flags = append(flags, pkgFlags...)
	}

	return flags, nil
}

// runPkgConfig executes pkg-config with the given flag and packages
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

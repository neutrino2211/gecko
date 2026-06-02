// spec: spec/modules.md

package config

import (
	"embed"
	"encoding/json"

	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/mitchellh/go-homedir"
	"github.com/neutrino2211/gecko/logger"
)

var (
	configLogger  = &logger.Logger{}
	defaultConfig = `{
	"root_path": "$root",
	"std_lib_path": "$root/root/std",
	"modules_path": "$root/root/src",
	"toolchain_path": "$root/toolchains",
	"version": "0.0.0",
	"default_compiler": "g++"
}`
	GeckoConfig = &Config{}
)

func ensureEmbeddedTree(root embed.FS, sourcePath string, destinationPath string) error {
	files, err := root.ReadDir(sourcePath)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(destinationPath, 0o755); err != nil {
		return err
	}

	for _, file := range files {
		embeddedPath := path.Join(sourcePath, file.Name())
		outputPath := filepath.Join(destinationPath, file.Name())

		if file.IsDir() {
			if err := ensureEmbeddedTree(root, embeddedPath, outputPath); err != nil {
				return err
			}
			continue
		}

		content, err := root.ReadFile(embeddedPath)
		if err != nil {
			return err
		}
		if err := os.WriteFile(outputPath, content, 0o644); err != nil {
			return err
		}
	}

	return nil
}

func readConfigJson(file string, cfg *Config) {
	configFile, err := os.Open(file)
	if err != nil {
		configLogger.LogString("opening config file", err.Error())
	}

	jsonParser := json.NewDecoder(configFile)
	if err = jsonParser.Decode(cfg); err != nil {
		configLogger.LogString("parsing config file", err.Error())
	}
}

type Config struct {
	RootPath        string             `json:"root_path"`
	StdLibPath      string             `json:"std_lib_path"`
	ModulesPath     string             `json:"modules_path"`
	ToolchainPath   string             `json:"toolchain_path"`
	Version         string             `json:"version"`
	DefaultCompiler string             `json:"default_compiler"`
	Options         *map[string]string `json:"options"`
	RootDir         embed.FS
	StdLibDir       embed.FS
}

func Init(root embed.FS, stdlib embed.FS) {
	configLogger.Init("config", 6)
	home, err := homedir.Dir()
	geckoPath := path.Join(home, "gecko")
	configFilePath := path.Join(geckoPath, "config.json")
	GeckoConfig.Options = &map[string]string{}

	if err != nil {
		configLogger.Fatal(err.Error())
	}

	if _, err := os.Stat(geckoPath); os.IsNotExist(err) {
		os.MkdirAll(geckoPath, 0o755)
	}

	if _, err := os.Stat(configFilePath); os.IsNotExist(err) {
		defaultConfig = strings.ReplaceAll(defaultConfig, "$root", geckoPath)

		configFile, err := os.Create(configFilePath)
		if err != nil {
			configLogger.Error("Error creating gecko config file")
		}

		_, err = configFile.WriteString(defaultConfig)
		if err != nil {
			configLogger.Error("Error setting gecko config permissions")
		}

		if err != nil {
			configLogger.Fatal(err.Error())
		}
	}

	readConfigJson(configFilePath, GeckoConfig)

	// Backward compatibility for older config files that don't include root_path.
	if GeckoConfig.RootPath == "" {
		GeckoConfig.RootPath = geckoPath
	}
	if GeckoConfig.StdLibPath == "" {
		GeckoConfig.StdLibPath = filepath.Join(GeckoConfig.RootPath, "root", "std")
	}
	if GeckoConfig.ModulesPath == "" {
		GeckoConfig.ModulesPath = filepath.Join(GeckoConfig.RootPath, "root", "src")
	}

	GeckoConfig.RootDir = root
	GeckoConfig.StdLibDir = stdlib

	if err := ensureEmbeddedTree(root, "root", filepath.Join(GeckoConfig.RootPath, "root")); err != nil {
		configLogger.LogString("ensuring embedded root files", err.Error())
	}
	if err := ensureEmbeddedTree(stdlib, "stdlib", GeckoConfig.StdLibPath); err != nil {
		configLogger.LogString("ensuring embedded stdlib files", err.Error())
	}
}

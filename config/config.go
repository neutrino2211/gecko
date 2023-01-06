package config

import (
	"encoding/json"

	"os"
	"path"
	"strings"

	"github.com/mitchellh/go-homedir"
	"github.com/neutrino2211/Gecko/logger"
)

var (
	configLogger  = &logger.Logger{}
	defaultConfig = `{
	"std_lib_path": "$root/std",
	"modules_path": "$root/src",
	"toolchain_path": "$root/toolchains",
	"version": "0.0.0",
	"default_compiler": "g++"
}`
	GeckoConfig = &Config{}
)

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
	StdLibPath      string             `json:"std_lib_path"`
	ModulesPath     string             `json:"modules_path"`
	ToolchainPath   string             `json:"toolchain_path"`
	Version         string             `json:"version"`
	DefaultCompiler string             `json:"default_compiler"`
	Options         *map[string]string `json:"options"`
}

func Init() {
	configLogger.Init("config", 6)
	home, err := homedir.Dir()
	geckoPath := path.Join(home, "gecko")
	configFilePath := path.Join(geckoPath, "config.json")
	GeckoConfig.Options = &map[string]string{}

	if err != nil {
		configLogger.Fatal(err.Error())
	}

	if _, err := os.Stat(geckoPath); os.IsNotExist(err) {
		os.MkdirAll(geckoPath, 0755)
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
}

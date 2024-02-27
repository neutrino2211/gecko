package config

import (
	"embed"
	"encoding/json"

	"os"
	"path"
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

func ensureRootFiles(path string, root embed.FS) {
	files, _ := root.ReadDir(path)
	for _, file := range files {
		if file.IsDir() {
			os.MkdirAll(GeckoConfig.RootPath+"/"+path+"/"+file.Name(), 0o755)
			ensureRootFiles(path+"/"+file.Name(), root)
		} else {
			content, _ := root.ReadFile(path + "/" + file.Name())
			if err := os.WriteFile(GeckoConfig.RootPath+"/"+path+"/"+file.Name(), content, 0o755); err != nil {
				print(err)
			}
		}
	}
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
}

func Init(root embed.FS) {
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

	GeckoConfig.RootDir = root

	readConfigJson(configFilePath, GeckoConfig)
	ensureRootFiles("root", root)
}

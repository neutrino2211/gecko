package compiler

import (
	"github.com/neutrino2211/gecko/config"
	"github.com/neutrino2211/gecko/frontend"
)

type ModuleLocation = frontend.ModuleLocation

func ResolveImportLocation(importerFilePath, importPath string, cfg *config.CompileCfg) ModuleLocation {
	return frontend.ResolveImportLocation(importerFilePath, importPath, cfg)
}

func getGeckoHome() string { return frontend.GeckoHome() }

func readSource(path string, cfg *config.CompileCfg) ([]byte, error) {
	return frontend.ReadSource(path, cfg)
}

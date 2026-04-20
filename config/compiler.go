package config

import "github.com/urfave/cli/v2"

type CompileCfg struct {
	Arch      string
	Platform  string
	Vendor    string
	CFlags    []string
	CLFlags   []string
	CObjects  []string
	CheckOnly bool

	Ctx     *cli.Context
	Project *ProjectConfig // Optional project configuration from gecko.toml
}

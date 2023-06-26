package config

import "github.com/urfave/cli/v2"

type CompileCfg struct {
	Arch     string
	Platform string
	CFlags   []string
	CLFlags  []string
	CObjects []string

	Ctx *cli.Context
}

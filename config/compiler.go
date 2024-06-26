package config

import "github.com/urfave/cli/v2"

type CompileCfg struct {
	Arch     string
	Platform string
	Vendor   string
	CFlags   []string
	CLFlags  []string
	CObjects []string

	LibMode bool

	Ctx *cli.Context
}

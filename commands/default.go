package commands

import (
	"github.com/urfave/cli/v2"
)

var GeckoCommands = []*cli.Command{
	InitCommand,
	RunCommand,
	BuildCommand,
	CompileCommand,
	CheckCommand,
	DocCommand,
	DepsCommand,
}

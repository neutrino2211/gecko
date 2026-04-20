package commands

import (
	"os"
	"runtime"

	"github.com/neutrino2211/gecko/compiler"
	"github.com/neutrino2211/gecko/config"
	"github.com/urfave/cli/v2"
)

var CheckCommand = &cli.Command{
	Name:        "check",
	Aliases:     []string{"ck"},
	Usage:       "gecko check ...sources",
	Description: "Check gecko source files for errors without generating output",
	Action: func(ctx *cli.Context) error {
		setLogLevel(ctx)

		if ctx.Args().Len() == 0 {
			println("No sources provided")
			return nil
		}

		for _, pos := range ctx.Args().Slice() {
			compiler.Compile(pos, &config.CompileCfg{
				Arch:     ctx.String("target-arch"),
				Platform: ctx.String("target-platform"),
				Vendor:   ctx.String("target-vendor"),
				CFlags:   []string{},
				CLFlags:  []string{},
				CObjects: []string{},
				Ctx:      ctx,
				CheckOnly: true,
			})
		}

		hasErrors := compiler.PrintErrorSummary()

		if hasErrors {
			os.Exit(1)
		}

		return nil
	},
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "backend",
			Value: "c",
			Usage: "The compilation backend to use (c | llvm)",
		},
		&cli.StringFlag{
			Name:  "target-arch",
			Value: runtime.GOARCH,
			Usage: "The compilation target architecture",
		},
		&cli.StringFlag{
			Name:  "target-platform",
			Value: runtime.GOOS,
			Usage: "The compilation target operating system",
		},
		&cli.StringFlag{
			Name:  "target-vendor",
			Value: "",
			Usage: "The compilation target vendor",
		},
		&cli.StringFlag{
			Name:  "log-level",
			Value: "silent",
			Usage: "Log level (silent | error | warn | info | debug | trace)",
		},
	},
}

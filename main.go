// spec: spec/modules.md, spec/c-interop.md, spec/stdlib.md

package main

import (
	"embed"
	"log"
	"os"

	"github.com/neutrino2211/gecko/commands"
	"github.com/neutrino2211/gecko/config"
	"github.com/neutrino2211/gecko/logger"
	"github.com/urfave/cli/v2"
)

// Version is set at build time via -ldflags
var Version = "dev"

//go:embed root
//go:embed root/*
var ROOT embed.FS

//go:embed stdlib
//go:embed stdlib/*
var STDLIB embed.FS

func main() {
	config.Init(ROOT, STDLIB)
	logger.SetDefaultChannel("Gecko")

	cmd := &cli.App{
		Commands:    commands.GeckoCommands,
		Description: "Gecko is a programming language designed for writing low level and highly performant applications using a beginner friendly syntax.",
		Name:        "gecko",
		HelpName:    "gecko",
		Usage:       "A playful new language written in Go",
		Version:     Version,
	}

	if err := cmd.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

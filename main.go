package main

import (
	"log"
	"os"

	"github.com/neutrino2211/gecko/commands"
	"github.com/neutrino2211/gecko/config"
	"github.com/neutrino2211/gecko/logger"
	"github.com/urfave/cli/v2"
)

func main() {
	config.Init()
	logger.SetDefaultChannel("Gecko")

	cmd := &cli.App{
		Commands:    commands.GeckoCommands,
		Description: "A playful new language written in Go",
	}

	if err := cmd.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

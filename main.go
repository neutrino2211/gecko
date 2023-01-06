package main

import (
	"os"

	"github.com/neutrino2211/gecko/commander"
	"github.com/neutrino2211/gecko/commands"
	"github.com/neutrino2211/gecko/config"
	"github.com/neutrino2211/gecko/logger"
)

func main() {
	config.Init()
	logger.SetDefaultChannel("Gecko")

	cmd := &commander.Commander{}

	cmd.Init()
	cmd.RegisterCommands(commands.GeckoCommands)

	cmd.Parse(os.Args)
}

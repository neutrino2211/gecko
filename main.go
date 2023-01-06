package main

import (
	"os"

	"github.com/neutrino2211/Gecko/commander"
	"github.com/neutrino2211/Gecko/commands"
	"github.com/neutrino2211/Gecko/config"
	"github.com/neutrino2211/Gecko/logger"
)

func main() {
	config.Init()
	logger.SetDefaultChannel("Gecko")

	cmd := &commander.Commander{}

	cmd.Init()
	cmd.RegisterCommands(commands.GeckoCommands)

	cmd.Parse(os.Args)
}

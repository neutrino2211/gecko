package commands

import (
	"os"

	"github.com/neutrino2211/Gecko/compiler"
	"github.com/neutrino2211/Gecko/config"
	"github.com/neutrino2211/Gecko/logger"

	"github.com/fatih/color"
	"github.com/neutrino2211/Gecko/commander"
)

// func streamPipe(std io.ReadCloser) {
// 	buf := bufio.NewReader(std) // Notice that this is not in a loop
// 	for {

// 		line, _, err := buf.ReadLine()
// 		if err != nil {
// 			break
// 		}
// 		fmt.Println(string(line))
// 	}
// }

// func streamCommand(cmd *exec.Cmd) {
// 	compileCommandLogger.LogString("executing command:", strings.Join(cmd.Args, " "))
// 	stdout, err := cmd.StdoutPipe()
// 	stderr, err := cmd.StderrPipe()
// 	if err != nil {
// 		log.Fatal(err)
// 	}
// 	cmd.Start()
// 	streamPipe(stdout)
// 	streamPipe(stderr)
// }

type CompileCommand struct {
	commander.Command
}

func (c *CompileCommand) Init() {
	c.Optionals = map[string]*commander.Optional{
		"output": {
			Type:        "string",
			Description: "Output file path " + color.HiYellowString("(warning: this overrides the build configuration's output path)"),
		},
		"type": {
			Type:        "string",
			Description: "Output type for program. (executable | library)",
		},
	}

	c.Usage = "gecko compile sources... [options]"

	c.Values = map[string]string{}

	compileCommandLogger.Init(c.CommandName, 2)
	c.Logger = *compileCommandLogger
	c.Description = c.BuildHelp(compileHelp)
}

func (c *CompileCommand) Run() {
	compileCommandLogger.Log(c.Positionals, c.Values, config.GeckoConfig, invokeDir)

	for _, pos := range c.Positionals {
		compiler.Compile(pos)
	}
}

var (
	compileHelp          = `compiles a gecko source file or a gecko project`
	compileCommandLogger = &logger.Logger{}
	invokeDir, _         = os.Getwd()
)

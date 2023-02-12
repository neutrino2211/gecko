package commands

import (
	"os"
	"runtime"

	"github.com/neutrino2211/gecko/compiler"
	"github.com/neutrino2211/gecko/config"
	"github.com/urfave/cli/v2"

	"github.com/fatih/color"
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

var CompileCommand = &cli.Command{
	Name:        "compile",
	Aliases:     []string{"c"},
	Usage:       "gecko compile ...sources",
	Description: compileHelp,
	Action: func(ctx *cli.Context) error {
		if ctx.Args().Len() == 0 {
			println("No sources provided")
			return nil
		}

		for _, pos := range ctx.Args().Slice() {
			compiler.Compile(pos, &config.CompileCfg{
				Arch:     ctx.String("target-arch"),
				CFlags:   []string{},
				CLFlags:  []string{},
				CObjects: []string{},
			})
		}

		compiler.PrintErrorSummary()

		return nil
	},
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "output",
			Value: ".",
			Usage: "Output file path " + color.HiYellowString("(warning: this overrides the build configuration's output path)"),
		},
		&cli.StringFlag{
			Name:  "type",
			Value: "executable",
			Usage: "Output type for program. (executable | library)",
		},
		&cli.StringFlag{
			Name:  "target-arch",
			Value: runtime.GOARCH,
			Usage: "The compilation target architecture",
		},
	},
}

var (
	compileHelp  = `compiles a gecko source file or a gecko project`
	invokeDir, _ = os.Getwd()
)

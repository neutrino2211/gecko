package commands

import (
	"bytes"
	"strings"
	"text/template"

	"github.com/neutrino2211/Gecko/commander"
)

var GeckoCommands = map[string]commander.Commandable{
	"compile": &CompileCommand{},
}

func buildCommandsList(c *DefaultCommand) string {
	s := ""
	buf := bytes.NewBufferString(s)

	maxSpaceKey := 0
	for o := range c.Commands {
		spaces := len(o)
		if maxSpaceKey < spaces {
			maxSpaceKey = spaces
		}
	}

	helpTemplate := `
	Commands:
	{{range $key, $value := .Commands}}
		{{$key}} {{spacer $key}} => {{$value.Description}}
	{{end}}
	`

	tmpl, err := template.New("help").Funcs(template.FuncMap{
		"spacer": func(key string) string {
			return strings.Repeat(" ", maxSpaceKey-len(key))
		},
	}).Parse(helpTemplate)

	if err != nil {
		c.DebugLogString(err.Error())
		c.Fatal("Failed during initialization")
	}

	err = tmpl.Execute(buf, c)

	if err != nil {
		c.DebugLogString(err.Error())
		c.Fatal("Failed during initialization")
	}

	s = buf.String()

	return s
}

type DefaultCommand struct {
	commander.Command
	Commands map[string]commander.Commandable
}

func (c *DefaultCommand) Init() {
	c.Optionals = map[string]*commander.Optional{
		"debug": {
			Type:        "bool",
			Description: "Turns on debug logging",
		},
	}

	c.Commands = GeckoCommands

	c.Description = c.BuildHelp("A programming language made for nerds.")
}

func (c *DefaultCommand) Run() {
	c.Logger.Init("", 2)
	c.Description += buildCommandsList(c)
	c.Help()
}

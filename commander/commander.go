package commander

import (
	"bytes"
	"fmt"
	"os"
	"strconv"
	"strings"
	"text/template"

	"github.com/fatih/color"
	"github.com/neutrino2211/Gecko/logger"
)

var (
	GlobalOptions = make(map[string]*Optional)
)

func confirmType(registeredType string, variable interface{}) bool {
	r := false
	switch variable.(type) {
	case string:
		r = registeredType == "string"
	case int64:
		r = registeredType == "int"
	case int32:
		r = registeredType == "int"
	case float64:
		r = registeredType == "float"
	case float32:
		r = registeredType == "float"
	case bool:
		r = registeredType == "bool"
	}

	return r
}

func fetchType(variable interface{}) string {
	r := "unknown"
	switch variable.(type) {
	case string:
		r = "string"
	case int64:
		r = "int"
	case int32:
		r = "int"
	case float64:
		r = "float"
	case float32:
		r = "float"
	case bool:
		r = "bool"
	}

	return r
}

func getValue(v string) interface{} {
	var r interface{}
	r, err := strconv.ParseInt(v, 0, 32)

	if err == nil {
		return r
	}

	r, err = strconv.ParseBool(v)

	if err == nil {
		return r
	}

	r, err = strconv.ParseFloat(v, 32)

	if err == nil {
		return r
	}

	return v
}

type Optional struct {
	Type        string
	Description string
}

type Listener struct {
	Option *Optional
	Method func(interface{})
}

//Command : Interface describing properties held by command
type Command struct {
	logger.Logger
	CommandName string
	Positionals []string
	Optionals   map[string]*Optional
	Values      map[string]string
	Usage       string
	Description string
	maxSpaceKey uint16
}

// Init : Here to prevent Logger.Init from overriding a command's Init method
func (c *Command) Init() {}

func (c *Command) Help() {
	c.LogString(c.Description)
}

func (c *Command) GetUsage() string {
	return c.Usage
}

func (c *Command) setMaxSpace() {
	c.maxSpaceKey = 0
	for o := range c.Optionals {
		spaces := len(o)
		if c.maxSpaceKey < uint16(spaces) {
			c.maxSpaceKey = uint16(spaces)
		}
	}
}

func (c *Command) Space(key string) string {
	return strings.Repeat(" ", len(key)-int(c.maxSpaceKey))
}

func (c *Command) BuildHelp(helpTemplate string) string {
	s := ""
	buf := bytes.NewBufferString(s)

	c.setMaxSpace()

	helpTemplate += `
	Usage: 
		{{.Usage}}
	{{if .Optionals}}
	Options:
	{{range $key, $value := .Optionals}}
		{{$key}} {{spacer $key}} => {{$value.Description}} [{{$value.Type}}] {{end}}
	{{end}}
	`

	tmpl, err := template.New("help").Funcs(template.FuncMap{
		"spacer": func(key string) string {
			return strings.Repeat(" ", int(c.maxSpaceKey)-len(key))
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

func (c *Command) GetBool(key string) bool {
	r, err := strconv.ParseBool(key)

	if err != nil {
		c.DebugLogString(err.Error())
		return false
	}

	return r
}

func (c *Command) Name() string {
	return c.CommandName
}

func (c *Command) SetName(name string) {
	c.CommandName = name
}

func (c *Command) RegisterOptional(option string, value string) {
	optType := c.Optionals[option]

	if optType == nil {
		return
	}

	optValue := getValue(value)
	isRightType := confirmType(optType.Type, optValue)

	if !isRightType {
		c.Init()
		c.Help()
		c.LogString("expected type " + color.GreenString(optType.Type) + " for option '" + option + "' but got " + color.RedString(fetchType(optValue)+" ("+value+")"))

		os.Exit(1)
	}

	c.Values[option] = value
}

func (c *Command) RegisterPositionals(positionals []string) {
	c.Positionals = positionals
}

type Commandable interface {
	Run()
	Init()
	Help()
	Name() string
	GetUsage() string
	SetName(string)
	RegisterOptional(string, string)
	RegisterPositionals([]string)
}

//Commander : Command line parser
type Commander struct {
	logger.Logger
	commands  map[string]Commandable
	listeners map[string]*Listener
	Ready     func()
}

func (c *Commander) Init() {
	c.Logger.Init("Gecko", 1)
	c.commands = make(map[string]Commandable)
	c.listeners = make(map[string]*Listener)
}

func (c *Commander) Register(name string, cmd Commandable) {
	c.commands[name] = cmd
}

func (c *Commander) RegisterCommands(cmds map[string]Commandable) {
	for name, cmd := range cmds {
		c.Register(name, cmd)
	}
}

func (c *Commander) RegisterOption(name string, listener *Listener) {
	c.listeners[name] = listener
}

// Parse : Parses command line arguments
func (c *Commander) Parse(cmds []string) {
	cmdName := ""
	if len(cmds) > 1 {
		cmdName = cmds[1]
	}
	registeredCmd := c.commands[cmdName]

	// Check if we have that command registered
	if registeredCmd == nil {
		c.LogString("command '" + cmdName + "' not found")

		fmt.Println("Usage:")
		fmt.Println("\t", "gecko <command> [arguments]")
		fmt.Print("\nCommands:\n\n")

		for cmd, command := range c.commands {
			command.SetName(cmd)
			command.Init()
			command.Help()
		}

		fmt.Print("\nGlobal Options:\n\n")

		maxSpaces := 0

		for optName := range c.listeners {
			if maxSpaces < len(optName) {
				maxSpaces = len(optName)
			}
		}

		for optName, globOption := range c.listeners {
			spaces := strings.Repeat(" ", maxSpaces-len(optName))
			fmt.Println("\t", optName+spaces, "=>", globOption.Option.Description, "["+globOption.Option.Type+"]")
		}

		fmt.Println("")

		os.Exit(1)
	}

	if registeredCmd.Name() == "" {
		registeredCmd.SetName(cmdName)
	}

	registeredCmd.Init()

	positionals := []string{}

	for i := 2; i < len(cmds); i++ {
		cmd := cmds[i]
		if !strings.HasPrefix(cmd, "-") {
			positionals = append(positionals, cmd)
		} else if strings.HasPrefix(cmd, "--") {
			option := cmd[2:]
			listener := c.listeners[option]

			if listener != nil && listener.Option.Type == "bool" {
				listener.Method(true)
				continue
			}

			i++

			if len(cmds) > i {
				registeredCmd.RegisterOptional(option, cmds[i])
			} else {
				registeredCmd.RegisterOptional(option, "")
			}

			if listener != nil && len(cmds) > i {
				listener.Method(getValue(cmds[i]))
			}
		}
	}

	if c.Ready != nil {
		c.Ready()
	}

	registeredCmd.RegisterPositionals(positionals)

	registeredCmd.Run()
}

package logger

import (
	"os"
	"strings"

	"github.com/alecthomas/repr"
	"github.com/fatih/color"
)

var (
	defaultChannel    = ""
	defaultDebugLevel = 1
)

func SetDefaultChannel(channel string) {
	defaultChannel = channel
}

func SetDefaultDebugMode(mode int) {
	defaultDebugLevel = mode
}

// Logger : This is the logging utility class
type Logger struct {
	channel string
	debug   bool
}

func (l *Logger) Init(channel string, debug int) {

	l.debug = defaultDebugLevel >= debug

	if channel == "" {
		l.channel = defaultChannel
	} else {
		l.channel = channel
	}
}

func (l *Logger) LogString(params ...string) {
	if defaultDebugLevel == 0 {
		return
	}
	println(l.channel+":", strings.Join(params, " "))
}

func (l *Logger) DebugLogString(params ...string) {
	if l.debug {
		l.LogString(params...)
	}
}

func (l *Logger) Error(params ...string) {
	color.Set(color.FgRed)
	l.LogString(params...)
	color.Unset()
}

func (l *Logger) Log(params ...interface{}) {
	repr.Println(params...)
}

func (l *Logger) DebugLog(params ...interface{}) {
	if l.debug {
		l.LogString("")
		l.Log(params...)
	}
}

func (l *Logger) FatalWithCode(code int, params ...string) {
	l.LogString(params...)

	os.Exit(code)
}

func (l *Logger) Fatal(params ...string) {
	l.LogString(params...)

	os.Exit(1)
}

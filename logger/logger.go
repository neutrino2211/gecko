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

type LogLevel int

const (
	LevelSilent LogLevel = iota
	LevelError
	LevelWarn
	LevelInfo
	LevelDebug
	LevelTrace
)

var globalLogLevel = LevelSilent

func SetLogLevel(level LogLevel) {
	globalLogLevel = level
}

func GetLogLevel() LogLevel {
	return globalLogLevel
}

func IsDebugEnabled() bool {
	return globalLogLevel >= LevelDebug
}

func GlobalError(format string, args ...interface{}) {
	if globalLogLevel >= LevelError {
		color.Set(color.FgRed, color.Bold)
		print("[ERROR] ")
		color.Unset()
		println(format)
	}
}

func GlobalWarn(format string, args ...interface{}) {
	if globalLogLevel >= LevelWarn {
		color.Set(color.FgYellow)
		print("[WARN] ")
		color.Unset()
		println(format)
	}
}

func GlobalInfo(format string, args ...interface{}) {
	if globalLogLevel >= LevelInfo {
		color.Set(color.FgCyan)
		print("[INFO] ")
		color.Unset()
		println(format)
	}
}

func GlobalDebug(section string, format string) {
	if globalLogLevel >= LevelDebug {
		color.Set(color.FgMagenta)
		print("[DEBUG:" + section + "] ")
		color.Unset()
		println(format)
	}
}

func ParseLogLevel(s string) LogLevel {
	switch strings.ToLower(s) {
	case "silent":
		return LevelSilent
	case "error":
		return LevelError
	case "warn", "warning":
		return LevelWarn
	case "info":
		return LevelInfo
	case "debug":
		return LevelDebug
	case "trace":
		return LevelTrace
	default:
		return LevelSilent
	}
}

func init() {
	if os.Getenv("GECKO_DEBUG") != "" {
		globalLogLevel = LevelDebug
	}
	if level := os.Getenv("GECKO_LOG_LEVEL"); level != "" {
		globalLogLevel = ParseLogLevel(level)
	}
}

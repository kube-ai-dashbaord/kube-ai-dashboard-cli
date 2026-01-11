package ui

import (
	"io"
	"log"
	"os"
	"strings"
)

var (
	Logger *log.Logger
	level  int
)

const (
	LevelDebug = iota
	LevelInfo
	LevelWarn
	LevelError
)

func InitLogger(logPath string, logLevel string) {
	var out io.Writer
	if logPath != "" {
		f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err == nil {
			out = f
		} else {
			out = os.Stderr
		}
	} else {
		out = os.Stderr
	}

	Logger = log.New(out, "[k13s] ", log.LstdFlags)

	switch strings.ToLower(logLevel) {
	case "debug":
		level = LevelDebug
	case "warn":
		level = LevelWarn
	case "error":
		level = LevelError
	default:
		level = LevelInfo
	}
}

func Debugf(format string, v ...interface{}) {
	if level <= LevelDebug {
		Logger.Printf("[DEBUG] "+format, v...)
	}
}

func Infof(format string, v ...interface{}) {
	if level <= LevelInfo {
		Logger.Printf("[INFO] "+format, v...)
	}
}

func Warnf(format string, v ...interface{}) {
	if level <= LevelWarn {
		Logger.Printf("[WARN] "+format, v...)
	}
}

func Errorf(format string, v ...interface{}) {
	if level <= LevelError {
		Logger.Printf("[ERROR] "+format, v...)
	}
}

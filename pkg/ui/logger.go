package ui

import (
	"strings"

	"github.com/kube-ai-dashbaord/kube-ai-dashboard-cli/pkg/log"
)

var (
	level int
)

const (
	LevelDebug = iota
	LevelInfo
	LevelWarn
	LevelError
)

func InitLogger(logPath string, logLevel string) {
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
	// pkg/log.Init is typically called in main.go
}

func Debugf(format string, v ...interface{}) {
	if level <= LevelDebug {
		log.Debugf(format, v...)
	}
}

func Infof(format string, v ...interface{}) {
	if level <= LevelInfo {
		log.Infof(format, v...)
	}
}

func Warnf(format string, v ...interface{}) {
	if level <= LevelWarn {
		log.Warnf(format, v...)
	}
}

func Errorf(format string, v ...interface{}) {
	if level <= LevelError {
		log.Errorf(format, v...)
	}
}

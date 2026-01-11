package log

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
)

var logger *log.Logger

func Init(appName string) error {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return err
	}
	logDir := filepath.Join(configDir, appName, "logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return err
	}

	logFile, err := os.OpenFile(filepath.Join(logDir, "k13s.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}

	logger = log.New(logFile, "", log.LstdFlags|log.Lshortfile)
	return nil
}

func Infof(format string, v ...any) {
	if logger != nil {
		logger.Output(2, fmt.Sprintf("[INFO] "+format, v...))
	}
}

func Errorf(format string, v ...any) {
	if logger != nil {
		logger.Output(2, fmt.Sprintf("[ERROR] "+format, v...))
	}
}

func Debugf(format string, v ...any) {
	if logger != nil {
		logger.Output(2, fmt.Sprintf("[DEBUG] "+format, v...))
	}
}

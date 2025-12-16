package logging

import (
	"log"
	"os"
)

var baseLogger = log.New(os.Stdout, "", log.LstdFlags|log.Lshortfile)

func Debugf(format string, args ...any) {
	baseLogger.Printf("[DEBUG] "+format, args...)
}

func Infof(format string, args ...any) {
	baseLogger.Printf("[INFO] "+format, args...)
}

func Warnf(format string, args ...any) {
	baseLogger.Printf("[WARN] "+format, args...)
}

func Errorf(format string, args ...any) {
	baseLogger.Printf("[ERROR] "+format, args...)
}

func Fatalf(format string, args ...any) {
	baseLogger.Printf("[FATAL] "+format, args...)
	os.Exit(1)
}

package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"strings"
)

type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
)

var (
	currentLevel = LevelInfo
	logger       = log.New(io.MultiWriter(os.Stderr), "", log.Ltime)
)

func Init(w io.Writer) {
	logger.SetOutput(io.MultiWriter(os.Stderr, w))
	logger.SetFlags(log.Ltime)

	levelStr := strings.ToUpper(os.Getenv("LOG_LEVEL"))
	switch levelStr {
	case "DEBUG":
		currentLevel = LevelDebug
	case "INFO":
		currentLevel = LevelInfo
	case "WARN", "WARNING":
		currentLevel = LevelWarn
	case "ERROR":
		currentLevel = LevelError
	default:
		currentLevel = LevelInfo
	}
}

func SetLevel(level Level) {
	currentLevel = level
}

func Debug(format string, v ...interface{}) {
	if currentLevel <= LevelDebug {
		logger.Printf("[DEBUG] "+format, v...)
	}
}

func Info(format string, v ...interface{}) {
	if currentLevel <= LevelInfo {
		logger.Printf("[INFO] "+format, v...)
	}
}

func Warn(format string, v ...interface{}) {
	if currentLevel <= LevelWarn {
		logger.Printf("[WARN] "+format, v...)
	}
}

func Error(format string, v ...interface{}) {
	if currentLevel <= LevelError {
		logger.Printf("[ERROR] "+format, v...)
	}
}

func Printf(format string, v ...interface{}) {
	logger.Printf(format, v...)
}

func Println(v ...interface{}) {
	logger.Println(v...)
}

func Fatal(v ...interface{}) {
	logger.Fatal(v...)
}

func Fatalf(format string, v ...interface{}) {
	logger.Fatalf(format, v...)
}

func Print(v ...interface{}) {
	logger.Print(v...)
}

func Sprintf(format string, v ...interface{}) string {
	return fmt.Sprintf(format, v...)
}

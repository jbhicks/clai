package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"sync"
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
	loggers      = make(map[string]*log.Logger)
	loggersMutex sync.RWMutex
)

// Init initializes the default logger with the given writer
func Init(w io.Writer) {
	logger := log.New(w, "", log.Ltime)
	loggersMutex.Lock()
	loggers["default"] = logger
	loggersMutex.Unlock()

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

// InitNamed initializes a named logger with the given writer
func InitNamed(name string, w io.Writer) {
	logger := log.New(w, "", log.Ltime)
	loggersMutex.Lock()
	loggers[name] = logger
	loggersMutex.Unlock()
}

// getLogger returns the named logger, or the default logger if name doesn't exist
func getLogger(name string) *log.Logger {
	loggersMutex.RLock()
	defer loggersMutex.RUnlock()
	if logger, exists := loggers[name]; exists {
		return logger
	}
	return loggers["default"]
}

func SetLevel(level Level) {
	currentLevel = level
}

func Debug(format string, v ...interface{}) {
	if currentLevel <= LevelDebug {
		getLogger("default").Printf("[DEBUG] "+format, v...)
	}
}

func Info(format string, v ...interface{}) {
	if currentLevel <= LevelInfo {
		getLogger("default").Printf("[INFO] "+format, v...)
	}
}

func Warn(format string, v ...interface{}) {
	if currentLevel <= LevelWarn {
		getLogger("default").Printf("[WARN] "+format, v...)
	}
}

func Error(format string, v ...interface{}) {
	if currentLevel <= LevelError {
		getLogger("default").Printf("[ERROR] "+format, v...)
	}
}

func Printf(format string, v ...interface{}) {
	getLogger("default").Printf(format, v...)
}

func Println(v ...interface{}) {
	getLogger("default").Println(v...)
}

func Fatal(v ...interface{}) {
	getLogger("default").Fatal(v...)
}

func Fatalf(format string, v ...interface{}) {
	getLogger("default").Fatalf(format, v...)
}

func Print(v ...interface{}) {
	getLogger("default").Print(v...)
}

// Named logger functions
func DebugNamed(name string, format string, v ...interface{}) {
	if currentLevel <= LevelDebug {
		getLogger(name).Printf("[DEBUG] "+format, v...)
	}
}

func InfoNamed(name string, format string, v ...interface{}) {
	if currentLevel <= LevelInfo {
		getLogger(name).Printf("[INFO] "+format, v...)
	}
}

func WarnNamed(name string, format string, v ...interface{}) {
	if currentLevel <= LevelWarn {
		getLogger(name).Printf("[WARN] "+format, v...)
	}
}

func ErrorNamed(name string, format string, v ...interface{}) {
	if currentLevel <= LevelError {
		getLogger(name).Printf("[ERROR] "+format, v...)
	}
}

func PrintfNamed(name string, format string, v ...interface{}) {
	getLogger(name).Printf(format, v...)
}

func PrintlnNamed(name string, v ...interface{}) {
	getLogger(name).Println(v...)
}

func FatalNamed(name string, v ...interface{}) {
	getLogger(name).Fatal(v...)
}

func FatalfNamed(name string, format string, v ...interface{}) {
	getLogger(name).Fatalf(format, v...)
}

func PrintNamed(name string, v ...interface{}) {
	getLogger(name).Print(v...)
}

func Sprintf(format string, v ...interface{}) string {
	return fmt.Sprintf(format, v...)
}

package logger

import (
	"io"
	"log"
	"os"
	"strings"
	"sync"

	"github.com/rs/zerolog"
)

var (
	infoLogger  *log.Logger
	errorLogger *log.Logger
	warnLogger  *log.Logger
	debugLogger *log.Logger

	zerologger *zerolog.Logger
	structured bool
	initOnce   sync.Once
)

// Init initializes the package loggers. If logFile is nil, os.Stdout is used.
// It respects the LOG_STRUCTURED environment variable (default: true) to enable structured JSON logs via zerolog.
func Init(logFile io.Writer) {
	initOnce.Do(func() {
		if logFile == nil {
			logFile = os.Stdout
		}

		// Check env for structured logs (default ON)
		v := os.Getenv("LOG_STRUCTURED")
		if strings.TrimSpace(strings.ToLower(v)) == "false" {
			structured = false
		} else {
			structured = true
		}

		if structured {
			z := zerolog.New(logFile).With().Timestamp().Logger()
			zerologger = &z
			// keep plain loggers too for any packages expecting text output
			infoLogger = log.New(logFile, "INFO: ", log.Ldate|log.Ltime|log.Lshortfile)
			errorLogger = log.New(logFile, "ERROR: ", log.Ldate|log.Ltime|log.Lshortfile)
			warnLogger = log.New(logFile, "WARN: ", log.Ldate|log.Ltime|log.Lshortfile)
			debugLogger = log.New(logFile, "DEBUG: ", log.Ldate|log.Ltime|log.Lshortfile)
		} else {
			infoLogger = log.New(logFile, "INFO: ", log.Ldate|log.Ltime|log.Lshortfile)
			errorLogger = log.New(logFile, "ERROR: ", log.Ldate|log.Ltime|log.Lshortfile)
			warnLogger = log.New(logFile, "WARN: ", log.Ldate|log.Ltime|log.Lshortfile)
			debugLogger = log.New(logFile, "DEBUG: ", log.Ldate|log.Ltime|log.Lshortfile)
			// also set standard log output
			log.SetOutput(logFile)
			log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
		}
	})
}

func Info(format string, v ...interface{}) {
	if structured && zerologger != nil {
		zerologger.Info().Msgf(format, v...)
		return
	}
	if infoLogger != nil {
		infoLogger.Printf(format, v...)
		return
	}
	log.Printf("INFO: "+format, v...)
}

func Error(format string, v ...interface{}) {
	if structured && zerologger != nil {
		zerologger.Error().Msgf(format, v...)
		return
	}
	if errorLogger != nil {
		errorLogger.Printf(format, v...)
		return
	}
	log.Printf("ERROR: "+format, v...)
}

func Warn(format string, v ...interface{}) {
	if structured && zerologger != nil {
		zerologger.Warn().Msgf(format, v...)
		return
	}
	if warnLogger != nil {
		warnLogger.Printf(format, v...)
		return
	}
	log.Printf("WARN: "+format, v...)
}

func Debug(format string, v ...interface{}) {
	if structured && zerologger != nil {
		zerologger.Debug().Msgf(format, v...)
		return
	}
	if debugLogger != nil {
		debugLogger.Printf(format, v...)
		return
	}
	log.Printf("DEBUG: "+format, v...)
}

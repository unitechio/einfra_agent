package logger

import (
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var Logger zerolog.Logger

// Init initializes the logger with both console and file output
func Init(logLevel, logDir string) error {
	// Parse log level
	level, err := zerolog.ParseLevel(logLevel)
	if err != nil {
		level = zerolog.InfoLevel
	}

	zerolog.SetGlobalLevel(level)

	// Console writer (pretty for terminal)
	consoleWriter := zerolog.ConsoleWriter{
		Out:        os.Stdout,
		TimeFormat: time.RFC3339,
	}

	// File writers
	textLogPath := filepath.Join(logDir, "agent.log")
	jsonLogPath := filepath.Join(logDir, "agent.json")

	textFile, err := os.OpenFile(textLogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}

	jsonFile, err := os.OpenFile(jsonLogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}

	// Multi-writer: console + text file + json file
	multi := io.MultiWriter(consoleWriter, textFile, jsonFile)

	Logger = zerolog.New(multi).With().
		Timestamp().
		Str("service", "einfra-agent").
		Logger()

	log.Logger = Logger

	Logger.Info().Msg("Logger initialized")
	return nil
}

// Info logs info level
func Info() *zerolog.Event {
	return Logger.Info()
}

// Error logs error level
func Error() *zerolog.Event {
	return Logger.Error()
}

// Warn logs warning level
func Warn() *zerolog.Event {
	return Logger.Warn()
}

// Debug logs debug level
func Debug() *zerolog.Event {
	return Logger.Debug()
}

// Fatal logs fatal and exits
func Fatal() *zerolog.Event {
	return Logger.Fatal()
}

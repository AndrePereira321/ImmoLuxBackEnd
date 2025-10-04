package logger

import (
	"github.com/rs/zerolog"
	"gopkg.in/natefinch/lumberjack.v2"
	"immo-lux/internal/server_error"
	"io"
	"os"
	"path/filepath"
	"time"
)

type Logger struct {
	logger *zerolog.Logger
	lumber *lumberjack.Logger
}

func New(name, level, logDir string) (*Logger, error) {
	logLevel := getLogLevel(level)
	writer, lumber, err := getLogWriter(name, logDir)
	if err != nil {
		return nil, err
	}

	logger := zerolog.New(writer).
		Level(logLevel).
		With().
		Timestamp().
		Str("name", name).
		Logger()

	return &Logger{
		logger: &logger,
		lumber: lumber,
	}, nil
}

func (l *Logger) Trace(msg string) {
	l.TraceEvent().Msg(msg)
}

func (l *Logger) TraceEvent() *zerolog.Event {
	return l.logger.Trace()
}

func (l *Logger) Debug(msg string) {
	l.DebugEvent().Msg(msg)
}

func (l *Logger) DebugEvent() *zerolog.Event {
	return l.logger.Debug()
}

func (l *Logger) Info(msg string) {
	l.InfoEvent().Msg(msg)
}

func (l *Logger) InfoEvent() *zerolog.Event {
	return l.logger.Info()
}

func (l *Logger) Warn(msg string) {
	l.WarnEvent().Msg(msg)
}

func (l *Logger) WarnEvent() *zerolog.Event {
	return l.logger.Warn()
}

func (l *Logger) Error(msg string) {
	l.ErrorEvent().Msg(msg)
}

func (l *Logger) ErrorEvent() *zerolog.Event {
	return l.logger.Error()
}

func (l *Logger) Fatal(msg string) {
	l.FatalEvent().Msg(msg)
}

func (l *Logger) FatalEvent() *zerolog.Event {
	return l.logger.Fatal()
}

func (l *Logger) Close() error {
	if l.lumber != nil {
		return l.lumber.Close()
	}
	return nil
}

func getLogLevel(level string) zerolog.Level {
	if logLevel, err := zerolog.ParseLevel(level); err == nil {
		return logLevel
	}
	return zerolog.InfoLevel
}

func getLogWriter(name, logDir string) (io.Writer, *lumberjack.Logger, error) {
	var lumber *lumberjack.Logger

	consoleWriter := zerolog.ConsoleWriter{
		Out:        os.Stdout,
		TimeFormat: time.RFC3339Nano,
	}

	if len(logDir) > 0 {
		if err := os.MkdirAll(logDir, 0755); err != nil {
			return nil, nil, server_error.Wrap("LOG_INIT", "error creating log directory", err)
		}
		lumber = &lumberjack.Logger{
			Filename:   filepath.Join(logDir, name+".log"),
			MaxSize:    10,
			MaxBackups: 30,
			MaxAge:     45,
			Compress:   true,
		}
		return io.MultiWriter(consoleWriter, lumber), lumber, nil
	}

	return consoleWriter, nil, nil
}

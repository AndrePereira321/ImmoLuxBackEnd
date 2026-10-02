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

func New(name, level, logDir string, logToConsole bool) (*Logger, error) {
	logLevel := getLogLevel(level)
	writer, lumber, err := getLogWriter(name, logDir, logToConsole)
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

func getLogWriter(name, logDir string, logToConsole bool) (io.Writer, *lumberjack.Logger, error) {
	var lumber *lumberjack.Logger
	var writers []io.Writer

	if logToConsole {
		writers = append(writers, zerolog.ConsoleWriter{
			Out:        os.Stdout,
			TimeFormat: time.RFC3339Nano,
		})
	}

	if len(logDir) > 0 {
		if err := os.MkdirAll(logDir, 0755); err != nil {
			return nil, nil, server_error.Wrap("LOG_INIT", "error creating log directory", err)
		}
		lumber = &lumberjack.Logger{
			Filename:   filepath.Join(logDir, name+".log"),
			MaxSize:    25,
			MaxBackups: 30,
			MaxAge:     45,
			Compress:   true,
		}
		// lumberjack opens the file lazily on the first write. When the file is
		// the only output, open it now so an unwritable file fails startup
		// instead of silently dropping every log line.
		if !logToConsole {
			if _, err := lumber.Write(nil); err != nil {
				return nil, nil, server_error.Wrap("LOG_INIT", "error opening log file", err)
			}
		}
		writers = append(writers, lumber)
	}

	if len(writers) == 0 {
		return nil, nil, server_error.New("LOG_INIT", "no log output: console disabled and no log directory")
	}
	if len(writers) == 1 {
		return writers[0], lumber, nil
	}
	// Unlike io.MultiWriter, this keeps writing to the file when stdout fails.
	return zerolog.MultiLevelWriter(writers...), lumber, nil
}

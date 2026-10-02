package logger

import (
	"immo-lux/internal/server_error"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewLogger(t *testing.T) {
	t.Run("test logger constructor", func(t *testing.T) {
		noFileLogger, err := New("TEST", "DEBUG", "", true)
		defer func() {
			_ = noFileLogger.Close()
		}()
		if err != nil {
			t.Error("failing logger constructor")
		}
		if noFileLogger.lumber != nil {
			t.Error("lumber initialized without folder")
		}

		tempDir := t.TempDir()
		fileLogger, err := New("TEST", "DEBUG", tempDir, true)
		defer func() {
			_ = fileLogger.Close()
		}()
		if err != nil {
			t.Error("failing logger constructor")
		}
		if fileLogger.lumber == nil {
			t.Error("lumber not initialized with folder")
		}
	})

	t.Run("test logger without console", func(t *testing.T) {
		writer, lumber, err := getLogWriter("TEST", t.TempDir(), false)
		if err != nil {
			t.Fatal("failing file-only writer")
		}
		defer func() {
			_ = lumber.Close()
		}()
		if writer != io.Writer(lumber) {
			t.Errorf("file-only logging must write to lumberjack alone, got %T", writer)
		}

		logDir := t.TempDir()
		fileOnlyLogger, err := New("TEST", "DEBUG", logDir, false)
		if err != nil {
			t.Fatal("failing file-only logger constructor")
		}
		fileOnlyLogger.Info("written to the file")
		_ = fileOnlyLogger.Close()
		logged, err := os.ReadFile(filepath.Join(logDir, "TEST.log"))
		if err != nil || !strings.Contains(string(logged), "written to the file") {
			t.Errorf("expected the log line in TEST.log, got %q (err %v)", logged, err)
		}

		_, err = New("TEST", "DEBUG", "", false)
		if !server_error.IsServerError(err, "LOG_INIT") {
			t.Error("logger without any output should fail")
		}
	})
}

func TestFileOnlyLoggerFailsFastOnUnopenableFile(t *testing.T) {
	// lumberjack opens lazily and works around most bad files (it even renames a
	// directory out of the way), so use a name no OS accepts.
	if _, err := New("TEST\x00", "DEBUG", t.TempDir(), false); !server_error.IsServerError(err, "LOG_INIT") {
		t.Errorf("expected LOG_INIT when the only output cannot be opened, got %v", err)
	}
}

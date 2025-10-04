package logger

import (
	"testing"
)

func TestNewLogger(t *testing.T) {
	t.Run("test logger constructor", func(t *testing.T) {
		noFileLogger, err := New("TEST", "DEBUG", "")
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
		fileLogger, err := New("TEST", "DEBUG", tempDir)
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
}

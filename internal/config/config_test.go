package config

import (
	"bytes"
	"immo-lux/internal/server_error"
	"os"
	"testing"
)

func TestBadFile(t *testing.T) {
	t.Run("test bad file", func(t *testing.T) {
		data := []byte("bad data")
		_, err := GetServerConfig(data)

		if !server_error.IsServerError(err, "CONFIG_PARSER") {
			t.Error("Failing bad file")
		}
	})
}

func TestLoggingConfig(t *testing.T) {
	parse := func(t *testing.T, toml string) (*LoggingConfig, error) {
		t.Helper()
		v := getViper()
		if err := v.ReadConfig(bytes.NewReader([]byte(toml))); err != nil {
			t.Fatalf("invalid test toml: %v", err)
		}
		return getLoggingConfig(v)
	}

	t.Run("console enabled by default", func(t *testing.T) {
		cfg, err := parse(t, "[logging]\nlog_dir=\"./logs\"\n")
		if err != nil || !cfg.LogToConsole() {
			t.Error("log_to_console should default to true")
		}
	})

	t.Run("console disabled with log dir", func(t *testing.T) {
		cfg, err := parse(t, "[logging]\nlog_dir=\"./logs\"\nlog_to_console=false\n")
		if err != nil || cfg.LogToConsole() {
			t.Error("log_to_console=false should be honored")
		}
	})

	t.Run("console disabled without log dir", func(t *testing.T) {
		_, err := parse(t, "[logging]\nlog_to_console=false\n")
		if !server_error.IsServerError(err, "CONFIG_PARSER") {
			t.Error("disabling console without log_dir should fail")
		}
	})
}

func TestProxyHeaderConfig(t *testing.T) {
	parse := func(t *testing.T, toml string) *HttpConfig {
		t.Helper()
		v := getViper()
		if err := v.ReadConfig(bytes.NewReader([]byte(toml))); err != nil {
			t.Fatalf("invalid test toml: %v", err)
		}
		cfg, err := getHttpConfig(v)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		return cfg
	}

	t.Run("no proxy by default", func(t *testing.T) {
		if header := parse(t, "[server]\nhost=\"localhost\"\n").ProxyHeader(); header != "" {
			t.Errorf("expected empty proxy header, got %q", header)
		}
	})

	t.Run("RFC 7239 Forwarded header is rejected", func(t *testing.T) {
		v := getViper()
		if err := v.ReadConfig(bytes.NewReader([]byte("[server]\nhost=\"localhost\"\nproxy_header=\"forwarded\"\n"))); err != nil {
			t.Fatalf("invalid test toml: %v", err)
		}
		if _, err := getHttpConfig(v); !server_error.IsServerError(err, "CONFIG_PARSER") {
			t.Errorf("expected CONFIG_PARSER error, got %v", err)
		}
	})

	t.Run("proxy header is read and trimmed", func(t *testing.T) {
		if header := parse(t, "[server]\nhost=\"localhost\"\nproxy_header=\" X-Forwarded-For \"\n").ProxyHeader(); header != "X-Forwarded-For" {
			t.Errorf("expected X-Forwarded-For, got %q", header)
		}
	})
}

func TestLogToConsoleParsing(t *testing.T) {
	parse := func(t *testing.T, toml string) (*LoggingConfig, error) {
		t.Helper()
		v := getViper()
		if err := v.ReadConfig(bytes.NewReader([]byte(toml))); err != nil {
			t.Fatalf("invalid test toml: %v", err)
		}
		return getLoggingConfig(v)
	}

	t.Run("string booleans are accepted", func(t *testing.T) {
		cfg, err := parse(t, "[logging]\nlog_dir=\"./logs\"\nlog_to_console=\"false\"\n")
		if err != nil || cfg.LogToConsole() {
			t.Errorf("log_to_console=\"false\" should disable the console, err %v", err)
		}
	})

	t.Run("unparseable values are rejected, not read as false", func(t *testing.T) {
		for _, value := range []string{`"yes"`, `"on"`, `""`, `1`} {
			_, err := parse(t, "[logging]\nlog_dir=\"./logs\"\nlog_to_console="+value+"\n")
			if !server_error.IsServerError(err, "CONFIG_PARSER") {
				t.Errorf("log_to_console=%s: expected CONFIG_PARSER error, got %v", value, err)
			}
		}
	})

	t.Run("blank log dir counts as empty", func(t *testing.T) {
		_, err := parse(t, "[logging]\nlog_dir=\"  \"\nlog_to_console=false\n")
		if !server_error.IsServerError(err, "CONFIG_PARSER") {
			t.Error("a whitespace log_dir must not satisfy the file-output requirement")
		}
	})
}

func TestConfigTemplateParses(t *testing.T) {
	data, err := os.ReadFile("../../configs/config.toml.template")
	if err != nil {
		t.Fatalf("failed to read template: %v", err)
	}
	cfg, err := GetServerConfig(data)
	if err != nil {
		t.Fatalf("config.toml.template must be a valid config: %v", err)
	}
	if !cfg.Logging().LogToConsole() || cfg.HttpServer().ProxyHeader() != "" || cfg.HttpServer().Port() != 8082 {
		t.Error("template defaults changed: expected console logging on, no proxy header and port 8082")
	}
}

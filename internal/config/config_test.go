package config

import (
	"os"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	origArgs := os.Args
	origEnvs := []string{
		"ALSAMIXER_WEB_PORT",
		"ALSAMIXER_WEB_BIND",
		"ALSAMIXER_WEB_CARD",
		"ALSAMIXER_WEB_LOG_LEVEL",
	}
	for _, e := range origEnvs {
		os.Unsetenv(e)
	}
	os.Args = []string{"cmd"}
	defer func() {
		os.Args = origArgs
		// Clear any leftovers
		for _, e := range origEnvs {
			os.Unsetenv(e)
		}
	}()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}
	if cfg.Port != 8080 || cfg.BindAddr != "0.0.0.0" || cfg.CardIndex != 0 || cfg.LogLevel != "info" {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
}

func TestLoadEnvOverrides(t *testing.T) {
	origArgs := os.Args

	os.Setenv("ALSAMIXER_WEB_PORT", "1234")
	os.Setenv("ALSAMIXER_WEB_BIND", "127.0.0.1")
	os.Setenv("ALSAMIXER_WEB_CARD", "2")
	os.Setenv("ALSAMIXER_WEB_LOG_LEVEL", "debug")
	os.Args = []string{"cmd"}
	defer func() {
		os.Args = origArgs
		os.Unsetenv("ALSAMIXER_WEB_PORT")
		os.Unsetenv("ALSAMIXER_WEB_BIND")
		os.Unsetenv("ALSAMIXER_WEB_CARD")
		os.Unsetenv("ALSAMIXER_WEB_LOG_LEVEL")
	}()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}
	if cfg.Port != 1234 || cfg.BindAddr != "127.0.0.1" || cfg.CardIndex != 2 || cfg.LogLevel != "debug" {
		t.Fatalf("env override failed: %+v", cfg)
	}
}

func TestLoadCLIOverrides(t *testing.T) {
	origArgs := os.Args

	os.Setenv("ALSAMIXER_WEB_PORT", "1111")
	os.Setenv("ALSAMIXER_WEB_BIND", "0.0.0.0")
	os.Setenv("ALSAMIXER_WEB_CARD", "1")
	os.Setenv("ALSAMIXER_WEB_LOG_LEVEL", "info")
	os.Args = []string{"cmd", "--port", "9090", "--bind", "127.0.0.2", "-c", "4", "--log-level", "error"}
	defer func() {
		os.Args = origArgs
		os.Unsetenv("ALSAMIXER_WEB_PORT")
		os.Unsetenv("ALSAMIXER_WEB_BIND")
		os.Unsetenv("ALSAMIXER_WEB_CARD")
		os.Unsetenv("ALSAMIXER_WEB_LOG_LEVEL")
	}()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}
	if cfg.Port != 9090 || cfg.BindAddr != "127.0.0.2" || cfg.CardIndex != 4 || cfg.LogLevel != "error" {
		t.Fatalf("CLI override failed: %+v", cfg)
	}
}

func TestHelpTextIncludesFlags(t *testing.T) {
	text := HelpText()
	if !(contains(text, "-port") || contains(text, "--port")) {
		t.Fatalf("help text missing port flag: %q", text)
	}
	if !(contains(text, "-p") || contains(text, "--port")) {
		t.Fatalf("help text missing port shorthand: %q", text)
	}
	if !(contains(text, "-bind") || contains(text, "--bind")) {
		t.Fatalf("help text missing bind flag: %q", text)
	}
	if !(contains(text, "-card") || contains(text, "--card")) {
		t.Fatalf("help text missing card flag: %q", text)
	}
	if !contains(text, "-log-level") {
		t.Fatalf("help text missing log-level flag: %q", text)
	}
}

func contains(s, substr string) bool {
	return stringIndex(s, substr) >= 0
}

func stringIndex(s, substr string) int {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

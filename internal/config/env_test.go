package config

import (
	"os"
	"testing"

	"github.com/datasance/router/internal/resources/types"
)

func TestGetConfigPath(t *testing.T) {
	key := types.TransportEnvConfig
	defer func() { _ = os.Unsetenv(key) }()

	// Default when unset
	os.Unsetenv(key)
	if got := GetConfigPath(); got != DefaultConfigPath {
		t.Errorf("GetConfigPath() with unset env = %q, want %q", got, DefaultConfigPath)
	}

	// Uses env when set
	want := "/custom/skrouterd.json"
	os.Setenv(key, want)
	if got := GetConfigPath(); got != want {
		t.Errorf("GetConfigPath() with env set = %q, want %q", got, want)
	}
}

func TestGetSSLProfilePath(t *testing.T) {
	key := types.EnvSSLProfilePath
	defer func() { _ = os.Unsetenv(key) }()

	// Default when unset
	os.Unsetenv(key)
	if got := GetSSLProfilePath(); got != DefaultSSLProfilePath {
		t.Errorf("GetSSLProfilePath() with unset env = %q, want %q", got, DefaultSSLProfilePath)
	}

	// Uses env when set
	want := "/custom/certs"
	os.Setenv(key, want)
	if got := GetSSLProfilePath(); got != want {
		t.Errorf("GetSSLProfilePath() with env set = %q, want %q", got, want)
	}
}

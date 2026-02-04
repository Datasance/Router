package config

import (
	"os"

	"github.com/datasance/router/internal/resources/types"
)

const (
	DefaultConfigPath     = "/tmp/skrouterd.json"
	DefaultSSLProfilePath = "/etc/skupper-router-certs"
)

// GetConfigPath returns the router config file path from QDROUTERD_CONF,
// or DefaultConfigPath if unset.
func GetConfigPath() string {
	if p := os.Getenv(types.TransportEnvConfig); p != "" {
		return p
	}
	return DefaultConfigPath
}

// GetSSLProfilePath returns the directory under which SSL profile certs
// reside (SSL_PROFILE_PATH env), or DefaultSSLProfilePath if unset.
func GetSSLProfilePath() string {
	if p := os.Getenv(types.EnvSSLProfilePath); p != "" {
		return p
	}
	return DefaultSSLProfilePath
}

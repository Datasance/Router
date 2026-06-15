package config

import (
	"os"
	"testing"

	"github.com/eclipse-iofog/router/internal/resources/types"
)

func TestIsKubernetesRouterMode(t *testing.T) {
	key := types.ENV_PLATFORM
	defer func() { _ = os.Unsetenv(key) }()

	tests := []struct {
		name string
		env  string
		want bool
	}{
		{"kubernetes", "kubernetes", true},
		{"pot", "pot", false},
		{"iofog alias", "iofog", false},
		{"unset defaults to pot mode", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.env == "" {
				os.Unsetenv(key)
			} else {
				os.Setenv(key, tt.env)
			}
			if got := IsKubernetesRouterMode(); got != tt.want {
				t.Errorf("IsKubernetesRouterMode() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsPotRouterMode(t *testing.T) {
	key := types.ENV_PLATFORM
	defer func() { _ = os.Unsetenv(key) }()

	tests := []struct {
		name string
		env  string
		want bool
	}{
		{"pot", "pot", true},
		{"iofog alias", "iofog", true},
		{"unset", "", true},
		{"kubernetes", "kubernetes", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.env == "" {
				os.Unsetenv(key)
			} else {
				os.Setenv(key, tt.env)
			}
			if got := IsPotRouterMode(); got != tt.want {
				t.Errorf("IsPotRouterMode() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetPlatformAcceptsIoFog(t *testing.T) {
	key := types.ENV_PLATFORM
	defer func() {
		_ = os.Unsetenv(key)
		ClearPlatform()
	}()

	os.Setenv(key, "iofog")
	ClearPlatform()
	if got := GetPlatform(); got != types.PlatformIoFog {
		t.Errorf("GetPlatform() with iofog = %q, want %q", got, types.PlatformIoFog)
	}
}

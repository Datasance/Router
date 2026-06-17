package config

import (
	"os"
	"slices"
	"strings"

	types "github.com/eclipse-iofog/router/internal/resources/skuppertypes"
	utils "github.com/eclipse-iofog/router/internal/routerutil"
	"k8s.io/utils/ptr"
)

var (
	Platform           string
	configuredPlatform *types.Platform
)

func ClearPlatform() {
	configuredPlatform = nil
}

// GetPlatform returns the runtime platform defined,
// where the lookup goes through the following sequence:
// - Platform variable,
// - SKUPPER_PLATFORM environment variable
// - Static platform defined by skupper switch
// - Default platform "kubernetes" otherwise.
// In case the defined platform is invalid, "kubernetes"
// will be returned.
func GetPlatform() types.Platform {
	if configuredPlatform != nil {
		return *configuredPlatform
	}

	var platform types.Platform
	for i, arg := range os.Args {
		if slices.Contains([]string{"--platform", "-p"}, arg) && i+1 < len(os.Args) {
			platformArg := os.Args[i+1]
			platform = types.Platform(platformArg)
			break
		} else if strings.HasPrefix(arg, "--platform=") || strings.HasPrefix(arg, "-p=") {
			platformArg := strings.Split(arg, "=")[1]
			platform = types.Platform(platformArg)
			break
		}
	}
	if platform == "" {
		platform = types.Platform(utils.DefaultStr(Platform,
			os.Getenv(types.EnvPlatform),
			string(types.PlatformKubernetes)))
	}
	switch platform {
	case types.PlatformPodman, types.PlatformDocker, types.PlatformLinux,
		types.PlatformPot, types.PlatformIoFog, types.PlatformKubernetes:
		configuredPlatform = &platform
	default:
		configuredPlatform = ptr.To(types.PlatformKubernetes)
	}
	return *configuredPlatform
}

// IsKubernetesRouterMode returns true when SKUPPER_PLATFORM is "kubernetes"
// (router config from ConfigMap). Default is pot (config from iofog SDK).
// SKUPPER_PLATFORM=pot or iofog both use SDK LocalAPI v3 mode.
func IsKubernetesRouterMode() bool {
	return os.Getenv(types.EnvPlatform) == string(types.PlatformKubernetes)
}

// IsPotRouterMode returns true when SKUPPER_PLATFORM is unset, "pot", or "iofog".
func IsPotRouterMode() bool {
	platform := os.Getenv(types.EnvPlatform)
	return platform == "" || platform == string(types.PlatformPot) || platform == string(types.PlatformIoFog)
}

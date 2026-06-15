package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	sdk "github.com/datasance/iofog-go-sdk/v3/pkg/microservices"
	"github.com/eclipse-iofog/router/internal/config"
	qdr "github.com/eclipse-iofog/router/internal/qdr"
	rt "github.com/eclipse-iofog/router/internal/router"
	"github.com/eclipse-iofog/router/internal/watch"
)

var (
	router *rt.Router
)

func init() {
	router = new(rt.Router)
	router.Config = &rt.Config{
		SslProfiles: make(map[string]qdr.SslProfile),
		Listeners:   make(map[string]qdr.Listener),
		Connectors:  make(map[string]qdr.Connector),
		Addresses:   make(map[string]qdr.Address),
		LogConfig:   make(map[string]qdr.LogConfig),
		Bridges: qdr.BridgeConfig{
			TcpListeners:  make(map[string]qdr.TcpEndpoint),
			TcpConnectors: make(map[string]qdr.TcpEndpoint),
		},
	}
}

func main() {
	if config.IsKubernetesRouterMode() {
		runKubernetesMode()
		return
	}
	runPotMode()
}

func runKubernetesMode() {
	configPath := config.GetConfigPath()
	// Config file is volume-mounted by the operator at QDROUTERD_CONF; retry briefly if not yet present.
	var data []byte
	var err error
	for i := 0; i < 30; i++ {
		data, err = os.ReadFile(configPath)
		if err == nil {
			break
		}
		if os.IsNotExist(err) && i < 29 {
			time.Sleep(time.Second)
			continue
		}
		log.Fatalf("Failed to read router config from %s: %v", configPath, err)
	}
	qdrConfig, err := qdr.UnmarshalRouterConfig(string(data))
	if err != nil {
		log.Fatalf("Failed to unmarshal router config: %v", err)
	}
	router.Config = &rt.Config{
		Metadata:    qdrConfig.Metadata,
		SslProfiles: qdrConfig.SslProfiles,
		Listeners:   qdrConfig.Listeners,
		Connectors:  qdrConfig.Connectors,
		Addresses:   qdrConfig.Addresses,
		LogConfig:   qdrConfig.LogConfig,
		SiteConfig:  qdrConfig.SiteConfig,
		Bridges:     qdrConfig.Bridges,
	}
	exitChannel := make(chan error)
	go router.StartRouter(exitChannel)
	ctx := context.Background()
	var lastAppliedMu sync.Mutex
	lastApplied := string(data)
	go watch.WatchConfigFile(ctx, configPath, func(configJSON string) error {
		lastAppliedMu.Lock()
		same := lastApplied == configJSON
		lastAppliedMu.Unlock()
		if same {
			return nil
		}
		qdrConfig, err := qdr.UnmarshalRouterConfig(configJSON)
		if err != nil {
			log.Printf("ERROR: Failed to unmarshal router config from file: %v", err)
			return err
		}
		newConfig := &rt.Config{
			Metadata:    qdrConfig.Metadata,
			SslProfiles: qdrConfig.SslProfiles,
			Listeners:   qdrConfig.Listeners,
			Connectors:  qdrConfig.Connectors,
			Addresses:   qdrConfig.Addresses,
			LogConfig:   qdrConfig.LogConfig,
			SiteConfig:  qdrConfig.SiteConfig,
			Bridges:     qdrConfig.Bridges,
		}
		if err := router.UpdateRouter(newConfig); err != nil {
			log.Printf("ERROR: Failed to update router from config file: %v", err)
			return err
		}
		lastAppliedMu.Lock()
		lastApplied = configJSON
		lastAppliedMu.Unlock()
		return nil
	})
	go watch.WatchSSLProfileDir(ctx, config.GetSSLProfilePath(), router.OnSSLProfilesFromDisk)
	<-exitChannel
	os.Exit(0)
}

func runPotMode() {
	ioFogClient, clientError := sdk.NewDefaultIoFogClientV3()
	if clientError != nil {
		log.Fatalln(clientError.Error())
	}
	if err := updateConfig(ioFogClient, router.Config); err != nil {
		log.Fatalln(err.Error())
	}
	confChannel := ioFogClient.EstablishControlWsConnection(0)
	exitChannel := make(chan error)
	go router.StartRouter(exitChannel)
	ctx := context.Background()
	go watch.WatchSSLProfileDir(ctx, config.GetSSLProfilePath(), router.OnSSLProfilesFromDisk)
	for {
		select {
		case <-exitChannel:
			os.Exit(0)
		case <-confChannel:
			newConfig := &rt.Config{
				SslProfiles: make(map[string]qdr.SslProfile),
				Listeners:   make(map[string]qdr.Listener),
				Connectors:  make(map[string]qdr.Connector),
				Addresses:   make(map[string]qdr.Address),
				LogConfig:   make(map[string]qdr.LogConfig),
				Bridges: qdr.BridgeConfig{
					TcpListeners:  make(map[string]qdr.TcpEndpoint),
					TcpConnectors: make(map[string]qdr.TcpEndpoint),
				},
			}
			if err := updateConfig(ioFogClient, newConfig); err != nil {
				log.Printf("Error updating config from ioFog local API: %v", err)
			} else {
				if err := router.UpdateRouter(newConfig); err != nil {
					log.Printf("Error updating router: %v", err)
				}
			}
		}
	}
}

func updateConfig(ioFogClient *sdk.IoFogClient, config interface{}) error {
	const attemptLimit = 5
	var lastErr error

	for attempt := 1; attempt <= attemptLimit; attempt++ {
		lastErr = ioFogClient.GetConfigIntoStruct(config)
		if lastErr == nil {
			return nil
		}
		if attempt == attemptLimit {
			break
		}
		log.Printf("WARN: Failed to get config from ioFog local API (attempt %d/%d): %v", attempt, attemptLimit, lastErr)
		time.Sleep(time.Duration(attempt) * time.Second)
	}

	var authErr *sdk.AuthMaterialError
	if errors.As(lastErr, &authErr) {
		return fmt.Errorf("failed to load ioFog service-account auth material: %w", lastErr)
	}
	var apiErr *sdk.V3APIError
	if errors.As(lastErr, &apiErr) {
		return fmt.Errorf("ioFog local API returned a v3 error while getting config: %w", lastErr)
	}
	return fmt.Errorf("update config failed after %d attempts: %w", attemptLimit, lastErr)
}

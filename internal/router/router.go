/*
 *  *******************************************************************************
 *  * Copyright (c) 2023 Datasance Teknoloji A.S.
 *  *
 *  * This program and the accompanying materials are made available under the
 *  * terms of the Eclipse Public License v. 2.0 which is available at
 *  * http://www.eclipse.org/legal/epl-2.0
 *  *
 *  * SPDX-License-Identifier: EPL-2.0
 *  *******************************************************************************
 *
 */

package router

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/datasance/router/internal/config"
	"github.com/datasance/router/internal/exec"
	"github.com/datasance/router/internal/qdr"
)

type Config struct {
	Metadata    qdr.RouterMetadata
	SslProfiles map[string]qdr.SslProfile
	Listeners   map[string]qdr.Listener
	Connectors  map[string]qdr.Connector
	Addresses   map[string]qdr.Address
	LogConfig   map[string]qdr.LogConfig
	SiteConfig  *qdr.SiteConfig
	Bridges     qdr.BridgeConfig
}

type Router struct {
	Config *Config
}

func (router *Router) UpdateRouter(newConfig *Config) error {
	log.Printf("DEBUG: Starting router configuration update")

	// Create agent pool and get client
	log.Printf("DEBUG: Creating agent pool")
	agentPool := qdr.NewAgentPool("amqp://localhost:5672", nil)
	client, err := agentPool.Get()
	if err != nil {
		log.Printf("ERROR: Failed to get client from pool: %v", err)
		return fmt.Errorf("failed to get client from pool: %v", err)
	}

	// Get current bridge configuration
	log.Printf("DEBUG: Getting current bridge configuration")
	currentBridgeConfig, err := client.GetLocalBridgeConfig()
	if err != nil {
		log.Printf("ERROR: Failed to get current bridge config: %v", err)
		return fmt.Errorf("failed to get current bridge config: %v", err)
	}

	// Calculate differences using qdr's built-in Difference method
	log.Printf("DEBUG: Calculating bridge configuration differences")
	changes := currentBridgeConfig.Difference(&newConfig.Bridges)
	log.Printf("DEBUG: Bridge config changes: %+v", changes)

	// Update via AMQP management using qdr's built-in function
	log.Printf("DEBUG: Updating bridge configuration")
	if err := client.UpdateLocalBridgeConfig(changes); err != nil {
		log.Printf("ERROR: Failed to update bridge config: %v", err)
		return fmt.Errorf("failed to update bridge config: %v", err)
	}

	// Update the configuration file (skip on Kubernetes; config is read-only from ConfigMap)
	if !config.IsKubernetesRouterMode() {
		log.Printf("DEBUG: Updating router configuration file")
		configJSON := router.GetRouterConfig()
		configPath := config.GetConfigPath()
		if err := os.WriteFile(configPath, []byte(configJSON), 0644); err != nil {
			log.Printf("ERROR: Failed to write router configuration: %v", err)
			return fmt.Errorf("failed to write router configuration: %v", err)
		}
	}

	// Update the in-memory configuration
	router.Config = newConfig

	// Return client to the pool instead of closing it
	if client != nil {
		agentPool.Put(client)
	}

	log.Printf("DEBUG: Router configuration update completed successfully")
	return nil
}

// OnSSLProfilesFromDisk merges profiles (from SSL_PROFILE_PATH scan) into Config.SslProfiles,
// writes the router config file, and calls qdr ReloadSslProfile for each profile so the
// running router picks up cert rotation without restart.
func (r *Router) OnSSLProfilesFromDisk(profiles map[string]qdr.SslProfile) {
	if r.Config == nil || r.Config.SslProfiles == nil {
		return
	}
	for name, profile := range profiles {
		r.Config.SslProfiles[name] = profile
	}
	// Write config file only on Pot; on Kubernetes config is read-only from ConfigMap
	if !config.IsKubernetesRouterMode() {
		configPath := config.GetConfigPath()
		configJSON := r.GetRouterConfig()
		if err := os.WriteFile(configPath, []byte(configJSON), 0644); err != nil {
			log.Printf("ERROR: Failed to write router config after SSL profile update: %v", err)
			return
		}
	}
	agentPool := qdr.NewAgentPool("amqp://localhost:5672", nil)
	client, err := agentPool.Get()
	if err != nil {
		log.Printf("ERROR: Failed to get qdr client for SSL profile reload: %v", err)
		return
	}
	defer agentPool.Put(client)
	for name := range profiles {
		if err := client.ReloadSslProfile(name); err != nil {
			log.Printf("ERROR: Failed to reload SSL profile %s: %v", name, err)
		}
	}
}

func (router *Router) GetRouterConfig() string {
	config := router.Config
	configElements := [][]interface{}{}

	// Add router metadata
	configElements = append(configElements, []interface{}{
		"router",
		config.Metadata,
	})

	// Add SSL profiles (file paths are already absolute)
	for _, profile := range config.SslProfiles {
		configElements = append(configElements, []interface{}{
			"sslProfile",
			profile,
		})
	}

	// Add listeners
	for _, listener := range config.Listeners {
		configElements = append(configElements, []interface{}{
			"listener",
			listener,
		})
	}

	// Add connectors
	for _, connector := range config.Connectors {
		configElements = append(configElements, []interface{}{
			"connector",
			connector,
		})
	}

	// Add TCP listeners
	for _, listener := range config.Bridges.TcpListeners {
		configElements = append(configElements, []interface{}{
			"tcpListener",
			listener,
		})
	}

	// Add TCP connectors
	for _, connector := range config.Bridges.TcpConnectors {
		configElements = append(configElements, []interface{}{
			"tcpConnector",
			connector,
		})
	}

	// Add addresses
	for _, address := range config.Addresses {
		configElements = append(configElements, []interface{}{
			"address",
			address,
		})
	}

	// Add log configs
	for _, logConfig := range config.LogConfig {
		configElements = append(configElements, []interface{}{
			"log",
			logConfig,
		})
	}

	// Add site config if present
	if config.SiteConfig != nil {
		configElements = append(configElements, []interface{}{
			"site",
			*config.SiteConfig,
		})
	}

	// Marshal to JSON
	data, err := json.MarshalIndent(configElements, "", "    ")
	if err != nil {
		log.Printf("Error marshaling router config: %v", err)
		return ""
	}

	return string(data)
}

func (router *Router) StartRouter(ch chan<- error) {
	log.Printf("DEBUG: Starting router with configuration")

	configPath := config.GetConfigPath()
	// On Pot we create and write initial config; on Kubernetes config is already mounted at QDROUTERD_CONF
	if !config.IsKubernetesRouterMode() {
		log.Printf("DEBUG: Creating initial router configuration")
		configJSON := router.GetRouterConfig()

		log.Printf("DEBUG: Ensuring configuration directory exists")
		if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
			log.Printf("ERROR: Failed to create configuration directory: %v", err)
			ch <- fmt.Errorf("failed to create configuration directory: %v", err)
			return
		}

		log.Printf("DEBUG: Writing initial configuration to %s", configPath)
		if err := os.WriteFile(configPath, []byte(configJSON), 0644); err != nil {
			log.Printf("ERROR: Failed to write initial configuration: %v", err)
			ch <- fmt.Errorf("failed to write initial configuration: %v", err)
			return
		}
	}

	// Start router with QDROUTERD_CONF and QDROUTERD_CONF_TYPE so launch script uses our config file
	env := []string{
		fmt.Sprintf("QDROUTERD_CONF=%s", configPath),
		"QDROUTERD_CONF_TYPE=json",
	}
	exitChannel := make(chan error)
	log.Printf("DEBUG: Starting router process")
	go exec.Run(ch, "/home/skrouterd/bin/launch.sh", []string{}, env)

	// Monitor for configuration updates
	go func() {
		for {
			select {
			case err := <-exitChannel:
				log.Printf("ERROR: Router process exited with error: %v", err)
				ch <- fmt.Errorf("router process exited with error: %v", err)
				return
			default:
				// Check for configuration updates from ioFog-agent
				time.Sleep(5 * time.Second)
			}
		}
	}()
}

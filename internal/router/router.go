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
	"encoding/base64"
	"encoding/json"
	"fmt"
	// "io/ioutil"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/datasance/router/internal/exec"
	"github.com/datasance/router/internal/qdr"
)

type IncomingSslProfile struct {
	Name    string `json:"name"`
	TlsCert string `json:"tlsCert"`
	TlsKey  string `json:"tlsKey"`
	CaCert  string `json:"caCert"`
}

type Config struct {
	Metadata           qdr.RouterMetadata
	SslProfiles        map[string]IncomingSslProfile
	ConvertedProfiles  map[string]qdr.SslProfile
	Listeners          map[string]qdr.Listener
	Connectors         map[string]qdr.Connector
	Addresses          map[string]qdr.Address
	LogConfig          map[string]qdr.LogConfig
	SiteConfig         *qdr.SiteConfig
	Bridges            qdr.BridgeConfig
}

type Router struct {
	Config *Config
}

func (router *Router) handleTLSFiles(sslProfile IncomingSslProfile) (qdr.SslProfile, error) {
	certDir := fmt.Sprintf("/etc/skupper-router/certs/%s", sslProfile.Name)
	if err := os.MkdirAll(certDir, 0755); err != nil {
		return qdr.SslProfile{}, fmt.Errorf("failed to create cert directory: %v", err)
	}

	// Create a new qdr.SslProfile with the name
	profile := qdr.SslProfile{
		Name: sslProfile.Name,
	}

	// Convert base64 encoded strings to files and update the profile
	if sslProfile.TlsCert != "" {
		certPath := filepath.Join(certDir, "tls.crt")
		if err := decodeCertToFile(sslProfile.TlsCert, certPath); err != nil {
			return qdr.SslProfile{}, fmt.Errorf("failed to write TLS certificate: %v", err)
		}
		profile.CertFile = certPath
	}
	if sslProfile.TlsKey != "" {
		keyPath := filepath.Join(certDir, "tls.key")
		if err := decodeCertToFile(sslProfile.TlsKey, keyPath); err != nil {
			return qdr.SslProfile{}, fmt.Errorf("failed to write TLS key: %v", err)
		}
		profile.PrivateKeyFile = keyPath
	}
	if sslProfile.CaCert != "" {
		caPath := filepath.Join(certDir, "ca.crt")
		if err := decodeCertToFile(sslProfile.CaCert, caPath); err != nil {
			return qdr.SslProfile{}, fmt.Errorf("failed to write CA certificate: %v", err)
		}
		profile.CaCertFile = caPath
	}

	return profile, nil
}

func decodeCertToFile(certString string, outputPath string) error {
	decoded, err := base64.StdEncoding.DecodeString(certString)
	if err != nil {
		return fmt.Errorf("failed to decode certificate: %v", err)
	}
	return os.WriteFile(outputPath, decoded, 0644)
}

func (router *Router) UpdateRouter(newConfig *Config) error {
	// Handle SSL profiles first and convert profiles
	convertedProfiles := make(map[string]qdr.SslProfile)
	for name, profile := range newConfig.SslProfiles {
		convertedProfile, err := router.handleTLSFiles(profile)
		if err != nil {
			return fmt.Errorf("failed to handle TLS files: %v", err)
		}
		convertedProfiles[name] = convertedProfile
	}
	newConfig.ConvertedProfiles = convertedProfiles

	// Create agent pool and get client
	agentPool := qdr.NewAgentPool("amqp://localhost:5672", nil)
	client, err := agentPool.Get()
	if err != nil {
		return fmt.Errorf("failed to get client from pool: %v", err)
	}

	// Get current bridge configuration
	currentBridgeConfig, err := client.GetLocalBridgeConfig()
	if err != nil {
		return fmt.Errorf("failed to get current bridge config: %v", err)
	}

	// Calculate differences using qdr's built-in Difference method
	changes := currentBridgeConfig.Difference(&newConfig.Bridges)

	// Update via AMQP management using qdr's built-in function
	if err := client.UpdateLocalBridgeConfig(changes); err != nil {
		return fmt.Errorf("failed to update bridge config: %v", err)
	}

	// Update the configuration file with the new TLS file paths
	configJSON := router.GetRouterConfig()
	configPath := "/etc/skupper-router/skupper-router.json"
	if err := os.WriteFile(configPath, []byte(configJSON), 0644); err != nil {
		return fmt.Errorf("failed to write router configuration: %v", err)
	}

	// Update the in-memory configuration
	router.Config = newConfig

	// Return client to the pool instead of closing it
	if client != nil {
		agentPool.Put(client)
	}

	return nil
}

func (router *Router) GetRouterConfig() string {
	config := router.Config
	configElements := [][]interface{}{}

	// Add router metadata
	configElements = append(configElements, []interface{}{
		"router",
		config.Metadata,
	})

	// Add SSL profiles with updated file paths
	for _, profile := range config.ConvertedProfiles {
		// Create a clean sslProfile entry for the config file
		sslProfile := qdr.SslProfile{
			Name:           profile.Name,
			CertFile:       profile.CertFile,
			PrivateKeyFile: profile.PrivateKeyFile,
			CaCertFile:     profile.CaCertFile,
		}
		configElements = append(configElements, []interface{}{
			"sslProfile",
			sslProfile,
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
	// Handle TLS files first and convert profiles
	convertedProfiles := make(map[string]qdr.SslProfile)
	for name, profile := range router.Config.SslProfiles {
		convertedProfile, err := router.handleTLSFiles(profile)
		if err != nil {
			ch <- fmt.Errorf("failed to handle TLS files: %v", err)
			return
		}
		convertedProfiles[name] = convertedProfile
	}
	router.Config.ConvertedProfiles = convertedProfiles

	// Create initial configuration
	config := router.GetRouterConfig()
	configPath := "/etc/skupper-router/skupper-router.json"

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		ch <- fmt.Errorf("failed to create configuration directory: %v", err)
		return
	}

	// Write initial configuration
	if err := os.WriteFile(configPath, []byte(config), 0644); err != nil {
		ch <- fmt.Errorf("failed to write initial configuration: %v", err)
		return
	}

	// Start router with JSON configuration
	args := []string{
		"--config", configPath,
	}

	exitChannel := make(chan error)
	go exec.Run(exitChannel, "skrouterd", args, []string{})

	// Monitor for configuration updates
	go func() {
		for {
			select {
			case err := <-exitChannel:
				ch <- fmt.Errorf("router process exited with error: %v", err)
				return
			default:
				// Check for configuration updates from ioFog-agent
				// This should be implemented based on your ioFog-agent integration
				time.Sleep(5 * time.Second)
			}
		}
	}()
}

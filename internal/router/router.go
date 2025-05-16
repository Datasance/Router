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
	log.Printf("DEBUG: Processing SSL profile: %s", sslProfile.Name)
	
	// Create base certs directory first with full permissions for non-root user
	baseCertDir := "/home/runner/skupper-router-certs"
	if err := os.MkdirAll(baseCertDir, 0777); err != nil {
		log.Printf("ERROR: Failed to create base cert directory: %v", err)
		return qdr.SslProfile{}, fmt.Errorf("failed to create base cert directory: %v", err)
	}

	// Create profile-specific directory with full permissions
	certDir := fmt.Sprintf("%s/%s", baseCertDir, sslProfile.Name)
	if err := os.MkdirAll(certDir, 0777); err != nil {
		log.Printf("ERROR: Failed to create cert directory: %v", err)
		return qdr.SslProfile{}, fmt.Errorf("failed to create cert directory: %v", err)
	}

	// Create a new qdr.SslProfile with the name
	profile := qdr.SslProfile{
		Name: sslProfile.Name,
	}

	// Convert base64 encoded strings to files and update the profile
	if sslProfile.TlsCert != "" {
		log.Printf("DEBUG: Processing TLS certificate for profile %s", sslProfile.Name)
		certPath := filepath.Join(certDir, "tls.crt")
		if err := decodeCertToFile(sslProfile.TlsCert, certPath); err != nil {
			log.Printf("ERROR: Failed to write TLS certificate: %v", err)
			return qdr.SslProfile{}, fmt.Errorf("failed to write TLS certificate: %v", err)
		}
		profile.CertFile = certPath
	}
	
	if sslProfile.TlsKey != "" {
		log.Printf("DEBUG: Processing TLS key for profile %s", sslProfile.Name)
		keyPath := filepath.Join(certDir, "tls.key")
		if err := decodeCertToFile(sslProfile.TlsKey, keyPath); err != nil {
			log.Printf("ERROR: Failed to write TLS key: %v", err)
			return qdr.SslProfile{}, fmt.Errorf("failed to write TLS key: %v", err)
		}
		profile.PrivateKeyFile = keyPath
	}
	
	if sslProfile.CaCert != "" {
		log.Printf("DEBUG: Processing CA certificate for profile %s", sslProfile.Name)
		caPath := filepath.Join(certDir, "ca.crt")
		if err := decodeCertToFile(sslProfile.CaCert, caPath); err != nil {
			log.Printf("ERROR: Failed to write CA certificate: %v", err)
			return qdr.SslProfile{}, fmt.Errorf("failed to write CA certificate: %v", err)
		}
		profile.CaCertFile = caPath
	}

	log.Printf("DEBUG: Successfully processed SSL profile: %s", sslProfile.Name)
	return profile, nil
}

func decodeCertToFile(certString string, outputPath string) error {
	log.Printf("DEBUG: Decoding certificate to file: %s", outputPath)
	
	decoded, err := base64.StdEncoding.DecodeString(certString)
	if err != nil {
		log.Printf("ERROR: Failed to decode certificate: %v", err)
		return fmt.Errorf("failed to decode certificate: %v", err)
	}
	
	if err := os.WriteFile(outputPath, decoded, 0644); err != nil {
		log.Printf("ERROR: Failed to write certificate file: %v", err)
		return fmt.Errorf("failed to write certificate file: %v", err)
	}
	
	log.Printf("DEBUG: Successfully wrote certificate to %s", outputPath)
	return nil
}

func (router *Router) UpdateRouter(newConfig *Config) error {
	log.Printf("DEBUG: Starting router configuration update")
	log.Printf("DEBUG: New configuration: %+v", newConfig)

	// Handle SSL profiles first and convert profiles
	convertedProfiles := make(map[string]qdr.SslProfile)
	for name, profile := range newConfig.SslProfiles {
		log.Printf("DEBUG: Converting SSL profile: %s", name)
		convertedProfile, err := router.handleTLSFiles(profile)
		if err != nil {
			log.Printf("ERROR: Failed to handle TLS files: %v", err)
			return fmt.Errorf("failed to handle TLS files: %v", err)
		}
		convertedProfiles[name] = convertedProfile
	}
	newConfig.ConvertedProfiles = convertedProfiles

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

	// Update the configuration file with the new TLS file paths
	log.Printf("DEBUG: Updating router configuration file")
	configJSON := router.GetRouterConfig()
	configPath := "/home/runner/skupper-router-certs/skrouterd.json"
	if err := os.WriteFile(configPath, []byte(configJSON), 0644); err != nil {
		log.Printf("ERROR: Failed to write router configuration: %v", err)
		return fmt.Errorf("failed to write router configuration: %v", err)
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
	log.Printf("DEBUG: Starting router with configuration")
	// log.Printf("DEBUG: Router config: %+v", router.Config)

	// Handle TLS files first and convert profiles
	convertedProfiles := make(map[string]qdr.SslProfile)
	for name, profile := range router.Config.SslProfiles {
		log.Printf("DEBUG: Converting SSL profile: %s", name)
		convertedProfile, err := router.handleTLSFiles(profile)
		if err != nil {
			log.Printf("ERROR: Failed to handle TLS files: %v", err)
			ch <- fmt.Errorf("failed to handle TLS files: %v", err)
			return
		}
		convertedProfiles[name] = convertedProfile
	}
	router.Config.ConvertedProfiles = convertedProfiles

	// Create initial configuration
	log.Printf("DEBUG: Creating initial router configuration")
	config := router.GetRouterConfig()
	configPath := "/home/runner/skupper-router-certs/skrouterd.json"

	// Ensure directory exists
	log.Printf("DEBUG: Ensuring configuration directory exists")
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		log.Printf("ERROR: Failed to create configuration directory: %v", err)
		ch <- fmt.Errorf("failed to create configuration directory: %v", err)
		return
	}

	// Write initial configuration
	log.Printf("DEBUG: Writing initial configuration to %s", configPath)
	if err := os.WriteFile(configPath, []byte(config), 0644); err != nil {
		log.Printf("ERROR: Failed to write initial configuration: %v", err)
		ch <- fmt.Errorf("failed to write initial configuration: %v", err)
		return
	}

	// Start router with JSON configuration
	exitChannel := make(chan error)
	log.Printf("DEBUG: Starting router process")
	go exec.Run(ch, "/home/skrouterd/bin/launch.sh", []string{}, []string{})

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

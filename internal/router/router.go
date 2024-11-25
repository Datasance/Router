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
	"fmt"
	"github.com/datasance/router/internal/exec"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
)

type Listener struct {
	Role             string `json:"role"`
	Host             string `json:"host"`
	Port             int    `json:"port"`
	SaslMechanisms   string `json:"saslMechanisms"`
	AuthenticatePeer string `json:"authenticatePeer"`
	SslProfile       string `json:"sslProfile"`
	RequireSsl       string `json:"requireSsl"`
}

type Connector struct {
	Name           string `json:"name"`
	Role           string `json:"role"`
	Host           string `json:"host"`
	Port           int    `json:"port"`
	SaslMechanisms string `json:"saslMechanisms"`
	SslProfile     string `json:"sslProfile"`
}

type SslProfile struct {
	Name    string `json:"name"`
	TlsCert string `json:"tlsCert"`
	TlsKey  string `json:"tlsKey"`
	CaCert  string `json:"caCert"`
}

type Config struct {
	Mode        string       `json:"mode"`
	Name        string       `json:"id"`
	Listeners   []Listener   `json:"listeners"`
	Connectors  []Connector  `json:"connectors"`
	SslProfiles []SslProfile `json:"sslProfiles"`
}

type Router struct {
	listeners   map[string]Listener
	connectors  map[string]Connector
	sslProfiles map[string]SslProfile
	Config      *Config
}

func skmanage(args []string) {
	exitChannel := make(chan error)
	go exec.Run(exitChannel, "skmanage", args, []string{})
	for {
		select {
		case <-exitChannel:
			return
		}
	}
}

func listenerName(listener Listener) string {
	return fmt.Sprintf("listener-%s-%s-%d", listener.Role, listener.Host, listener.Port)
}

func connectorName(connector Connector) string {
	return fmt.Sprintf("connector-%s-%s-%s-%d", connector.Name, connector.Role, connector.Host, connector.Port)
}

func sslProfileName(sslProfile SslProfile) string {
	return fmt.Sprintf("%s", sslProfile.Name)
}

func deleteEntity(name string) {
	args := []string{
		"delete",
		fmt.Sprintf("--name=%s", name),
	}
	skmanage(args)
}

func (router *Router) createListener(listener Listener) {
	if listener.SaslMechanisms == "" {
		listener.SaslMechanisms = "ANONYMOUS"
	}
	if listener.AuthenticatePeer == "" {
		listener.AuthenticatePeer = "no"
	}
	if listener.RequireSsl == "" {
		listener.RequireSsl = "no"
	}

	args := []string{
		"create",
		"--type=listener",
		fmt.Sprintf("port=%d", listener.Port),
		fmt.Sprintf("role=%s", listener.Role),
		fmt.Sprintf("host=%s", listener.Host),
		fmt.Sprintf("name=%s", listenerName(listener)),
	}

	if listener.SaslMechanisms != "" {
		args = append(args, fmt.Sprintf("sslProfile=%s", listener.SaslMechanisms))
	}

	if listener.AuthenticatePeer != "" {
		args = append(args, fmt.Sprintf("requireSsl=%s", listener.AuthenticatePeer))
	}

	if listener.SslProfile != "" {
		args = append(args, fmt.Sprintf("sslProfile=%s", listener.SslProfile))
	}

	if listener.RequireSsl != "" {
		args = append(args, fmt.Sprintf("requireSsl=%s", listener.RequireSsl))
	}

	skmanage(args)
	router.listeners[listenerName(listener)] = listener
}

func (router *Router) createConnector(connector Connector) {
	if connector.SaslMechanisms == "" {
		connector.SaslMechanisms = "ANONYMOUS"
	}

	args := []string{
		"create",
		"--type=connector",
		fmt.Sprintf("port=%d", connector.Port),
		fmt.Sprintf("role=%s", connector.Role),
		fmt.Sprintf("host=%s", connector.Host),
		fmt.Sprintf("name=%s", connectorName(connector)),
	}

	if connector.SaslMechanisms != "" {
		args = append(args, fmt.Sprintf("sslProfile=%s", connector.SaslMechanisms))
	}

	if connector.SslProfile != "" {
		args = append(args, fmt.Sprintf("sslProfile=%s", connector.SslProfile))
	}

	skmanage(args)
	router.connectors[connectorName(connector)] = connector
}

func (router *Router) deleteListener(listener Listener) {
	deleteEntity(listenerName(listener))
	delete(router.listeners, listenerName(listener))
}

func (router *Router) deleteConnector(connector Connector) {
	deleteEntity(connectorName(connector))
	delete(router.connectors, connectorName(connector))
}

func (router *Router) handleTLSFiles(sslProfile SslProfile) error {
	certDir := fmt.Sprintf("/home/runner/%s-cert", sslProfile.Name)
	log.Printf("Creating directory: %s", certDir)
	caCert := sslProfile.CaCert
	tlsCert := sslProfile.TlsCert
	tlsKey := sslProfile.TlsKey
	if err := os.MkdirAll(certDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %v", err)
	}
	log.Printf("Handling TLS files in directory: %s", certDir)

	if caCert != "" {
		log.Printf("Processing CaCert for %s", sslProfile.Name)
		caPath := filepath.Join(certDir, "ca.crt")
		if err := decodeCertToFile(sslProfile.CaCert, caPath); err != nil {
			return fmt.Errorf("failed to decode CaCert: %v", err)
		}
	}

	if tlsCert != "" {
		log.Printf("Processing TlsCert for %s", sslProfile.Name)
		tlsCertPath := filepath.Join(certDir, "tls.crt")
		if err := decodeCertToFile(sslProfile.TlsCert, tlsCertPath); err != nil {
			return fmt.Errorf("failed to decode TlsCert: %v", err)
		}
	}

	if tlsKey != "" {
		log.Printf("Processing TlsKey for %s", sslProfile.Name)
		tlsKeyPath := filepath.Join(certDir, "tls.key")
		if err := decodeCertToFile(sslProfile.TlsKey, tlsKeyPath); err != nil {
			return fmt.Errorf("failed to decode TlsKey: %v", err)
		}
	}

	return nil
}

func decodeCertToFile(certString string, outputPath string) error {
	log.Printf("Starting decodeCertToFile for outputPath: %s", outputPath)

	// Decode the base64 data
	decodedData, err := base64.StdEncoding.DecodeString(certString)
	if err != nil {
		log.Fatalf("Failed to decode base64 data: %v", err)
	}

	// Write the decoded data to the file
	if err := ioutil.WriteFile(outputPath, decodedData, 0644); err != nil {
		log.Printf("Failed to write PEM data to file %s: %v", outputPath, err)
		return fmt.Errorf("failed to write PEM data to file: %v", err)
	}

	log.Printf("Successfully wrote PEM file to: %s", outputPath)
	return nil
}

func (router *Router) UpdateRouter(newConfig *Config) {
	newListeners := make(map[string]Listener)
	newConnectors := make(map[string]Connector)

	for _, listener := range newConfig.Listeners {
		newListeners[listenerName(listener)] = listener
		if _, ok := router.listeners[listenerName(listener)]; !ok {
			router.createListener(listener)
		}
	}

	for _, connector := range newConfig.Connectors {
		newConnectors[connectorName(connector)] = connector
		if _, ok := router.connectors[connectorName(connector)]; !ok {
			router.createConnector(connector)
		}
	}

	for _, sslProfile := range newConfig.SslProfiles {
		if err := router.handleTLSFiles(sslProfile); err != nil {
		}
	}

	for _, listener := range router.listeners {
		if _, ok := newListeners[listenerName(listener)]; !ok {
			router.deleteListener(listener)
		}
	}

	for _, connector := range router.connectors {
		if _, ok := newConnectors[connectorName(connector)]; !ok {
			router.deleteConnector(connector)
		}
	}

	router.Config = newConfig
}

func (router *Router) GetRouterConfig() string {
	listenersConfig := ""
	for _, listener := range router.listeners {
		// Handle default values for listener fields
		saslMechanisms := listener.SaslMechanisms
		if saslMechanisms == "" {
			saslMechanisms = "ANONYMOUS"
		}

		authenticatePeer := listener.AuthenticatePeer
		if authenticatePeer == "" {
			authenticatePeer = "no"
		}

		requireSsl := ""
		if listener.RequireSsl != "" {
			requireSsl = fmt.Sprintf("  requireSsl: %s\\n", listener.RequireSsl)
		}

		sslProfile := ""
		if listener.SslProfile != "" {
			sslProfile = fmt.Sprintf("  sslProfile: %s\\n", listener.SslProfile)
		}

		listenersConfig += fmt.Sprintf(
			"\\nlistener {\\n  name: %s\\n  role: %s\\n  host: %s\\n  port: %d\\n  saslMechanisms: %s\\n  authenticatePeer: %s\\n%s%s}",
			listenerName(listener),
			listener.Role,
			listener.Host,
			listener.Port,
			saslMechanisms,
			authenticatePeer,
			sslProfile,
			requireSsl,
		)
	}

	connectorsConfig := ""
	for _, connector := range router.connectors {
		// Handle default values for connector fields
		saslMechanisms := connector.SaslMechanisms
		if saslMechanisms == "" {
			saslMechanisms = "ANONYMOUS"
		}

		sslProfile := ""
		if connector.SslProfile != "" {
			sslProfile = fmt.Sprintf("  sslProfile: %s\\n", connector.SslProfile)
		}

		connectorsConfig += fmt.Sprintf(
			"\\nconnector {\\n  name: %s\\n  host: %s\\n  port: %d\\n  role: %s\\n  saslMechanisms: %s\\n%s}",
			connectorName(connector),
			connector.Host,
			connector.Port,
			connector.Role,
			saslMechanisms,
			sslProfile,
		)
	}

	sslProfilesConfig := ""
	for _, sslProfile := range router.sslProfiles {
		sslProfilesConfig += fmt.Sprintf(
			"\\nsslProfile {\\n  name: %s\\n  caCertFile: /home/runner/%s-cert/ca.crt\\n  certFile: /home/runner/%s-cert/tls.crt\\n  privateKeyFile: /home/runner/%s-cert/tls.key\\n}",
			sslProfileName(sslProfile),
			sslProfileName(sslProfile),
			sslProfileName(sslProfile),
			sslProfileName(sslProfile),
		)
	}

	return fmt.Sprintf(
		"router {\\n  mode: %s\\n  id: %s\\n  saslConfigDir: /etc/sasl2/\\n}%s%s%s",
		router.Config.Mode,
		router.Config.Name,
		listenersConfig,
		connectorsConfig,
		sslProfilesConfig,
	)
}

func (router *Router) StartRouter(ch chan<- error) {
	router.listeners = make(map[string]Listener)
	router.connectors = make(map[string]Connector)
	router.sslProfiles = make(map[string]SslProfile)

	for _, listener := range router.Config.Listeners {
		router.listeners[listenerName(listener)] = listener
	}

	for _, connector := range router.Config.Connectors {
		router.connectors[connectorName(connector)] = connector
	}

	for _, sslProfile := range router.Config.SslProfiles {
		router.sslProfiles[sslProfileName(sslProfile)] = sslProfile
		if err := router.handleTLSFiles(sslProfile); err != nil {
		}
	}

	routerConfig := router.GetRouterConfig()
	exec.Run(ch, "/home/skrouterd/bin/launch.sh", []string{}, []string{"QDROUTERD_CONF=" + routerConfig})
}

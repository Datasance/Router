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

package main

import (
	"context"
	"errors"
	"log"
	"os"
	"sync"
	"time"

	sdk "github.com/datasance/iofog-go-sdk/v3/pkg/microservices"
	"github.com/datasance/router/internal/config"
	qdr "github.com/datasance/router/internal/qdr"
	rt "github.com/datasance/router/internal/router"
	"github.com/datasance/router/internal/watch"
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
	ioFogClient, clientError := sdk.NewDefaultIoFogClient()
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
				log.Fatal(err)
			} else {
				if err := router.UpdateRouter(newConfig); err != nil {
					log.Printf("Error updating router: %v", err)
				}
			}
		}
	}
}

func updateConfig(ioFogClient *sdk.IoFogClient, config interface{}) error {
	attemptLimit := 5
	var err error

	for err = ioFogClient.GetConfigIntoStruct(config); err != nil && attemptLimit > 0; attemptLimit-- {
		return err
	}

	if attemptLimit == 0 {
		return errors.New("Update config failed")
	}

	return nil
}

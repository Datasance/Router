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
	"errors"
	"log"
	"os"

	rt "github.com/datasance/router/internal/router"

	sdk "github.com/datasance/iofog-go-sdk/v3/pkg/microservices"
	qdr "github.com/datasance/router/internal/qdr"
)

var (
	router *rt.Router
)

func init() {
	router = new(rt.Router)
	router.Config = &rt.Config{
		SslProfiles:       make(map[string]rt.IncomingSslProfile),
		ConvertedProfiles: make(map[string]qdr.SslProfile),
		Listeners:         make(map[string]qdr.Listener),
		Connectors:        make(map[string]qdr.Connector),
		Addresses:         make(map[string]qdr.Address),
		LogConfig:         make(map[string]qdr.LogConfig),
		Bridges: qdr.BridgeConfig{
			TcpListeners:  make(map[string]qdr.TcpEndpoint),
			TcpConnectors: make(map[string]qdr.TcpEndpoint),
		},
	}
}

func main() {
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

	for {
		select {
		case <-exitChannel:
			os.Exit(0)
		case <-confChannel:
			newConfig := &rt.Config{
				SslProfiles:       make(map[string]rt.IncomingSslProfile),
				ConvertedProfiles: make(map[string]qdr.SslProfile),
				Listeners:         make(map[string]qdr.Listener),
				Connectors:        make(map[string]qdr.Connector),
				Addresses:         make(map[string]qdr.Address),
				LogConfig:         make(map[string]qdr.LogConfig),
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

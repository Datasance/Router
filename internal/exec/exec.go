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

package exec

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"os/exec"
)

func Run(ch chan<- error, command string, args []string, env []string) {
	// log.Printf("Running command: %s with args: %v and env vars: %v", command, args, env)

	cmd := exec.Command(command, args...)
	cmd.Env = append(os.Environ(), env...)

	outReader, err := cmd.StdoutPipe()
	if err != nil {
		log.Fatal(err)
	}
	outScanner := bufio.NewScanner(outReader)
	go func() {
		for outScanner.Scan() {
			fmt.Println(outScanner.Text())
		}
	}()

	errReader, err := cmd.StderrPipe()
	if err != nil {
		log.Fatal(err)
	}
	errScanner := bufio.NewScanner(errReader)
	go func() {
		for errScanner.Scan() {
			fmt.Println(errScanner.Text())
		}
	}()

	if err := cmd.Start(); err != nil {
		log.Fatal(err)
	}

	if err := cmd.Wait(); err != nil {
		log.Fatal(err)
	}
	ch <- err
}

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

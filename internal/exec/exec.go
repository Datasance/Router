package exec

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
)

func Run(ch chan<- error, command string, args []string, env []string) {
	cmd := exec.Command(command, args...)
	cmd.Env = append(os.Environ(), env...)

	outReader, err := cmd.StdoutPipe()
	if err != nil {
		ch <- err
		return
	}
	outScanner := bufio.NewScanner(outReader)
	go func() {
		for outScanner.Scan() {
			_, _ = fmt.Println(outScanner.Text())
		}
	}()

	errReader, err := cmd.StderrPipe()
	if err != nil {
		ch <- err
		return
	}
	errScanner := bufio.NewScanner(errReader)
	go func() {
		for errScanner.Scan() {
			_, _ = fmt.Println(errScanner.Text())
		}
	}()

	if err := cmd.Start(); err != nil {
		ch <- err
		return
	}

	if err := cmd.Wait(); err != nil {
		ch <- err
		return
	}
	ch <- nil
}

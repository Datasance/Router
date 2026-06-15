package routerutil

import (
	"context"
	"fmt"
	"net"
	"sync"
	"testing"

	"gotest.tools/v3/assert"
)

func TestTCPPortNextFree(t *testing.T) {
	minPort, err := TCPPortNextFree(1024)
	assert.Assert(t, err, "no available tcp ports found")

	ctx, cancel := context.WithCancel(context.Background())

	// listening on minPort to validate if it reports as in use
	wg := listenTCPPort(ctx, minPort)
	// waiting on port to be bound
	wg.Wait()

	// assert TCPPortNextFree shows a different port
	newMinPort, err := TCPPortNextFree(minPort)
	assert.Assert(t, err, "no more available tcp ports found")
	assert.Assert(t, newMinPort > minPort, "expected next free port available to be higher than %d but got %d", minPort, newMinPort)
	cancel()
}

func TestTCPPortInUse(t *testing.T) {
	minPort, err := TCPPortNextFree(1024)
	assert.Assert(t, err, "no available tcp ports found")

	ctx := context.Background()

	// listening on minPort to validate if it reports as in use
	wg := listenTCPPort(ctx, minPort)
	// waiting on port to be bound
	wg.Wait()

	// assert TCPPortInUse reports port as being used
	assert.Assert(t, TCPPortInUse("", minPort), "%d expected to be in use", minPort)

	// getting an extra port
	nextMinPort, err := TCPPortNextFree(minPort)
	assert.Assert(t, err, "no more available tcp ports found")
	assert.Assert(t, !TCPPortInUse("", nextMinPort), "tcp port %d expected to be available", nextMinPort)
}

func listenTCPPort(ctx context.Context, port int) *sync.WaitGroup {
	wg := new(sync.WaitGroup)
	wg.Add(1)
	go func() {
		listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
		wg.Done()
		if err != nil {
			<-ctx.Done()
			_ = listener.Close()
		}
	}()
	return wg
}

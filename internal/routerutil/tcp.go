package routerutil

import (
	"errors"
	"net"
	"strconv"
)

func TCPPortInUse(host string, port int) bool {
	address := net.JoinHostPort(host, strconv.Itoa(port))
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return true
	}
	if listener != nil {
		_ = listener.Close()
	}
	return false
}

func TCPPortNextFree(startPort int) (int, error) {
	for port := startPort; port <= 65535; port++ {
		if !TCPPortInUse("", port) {
			return port, nil
		}
	}
	return 0, errors.New("no available ports found")
}

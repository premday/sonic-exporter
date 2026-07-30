//go:build !linux

package main

import (
	"context"
	"fmt"
	"net"
	"runtime"
)

func listenTCPInVRF(_ context.Context, address, vrf string) (net.Listener, error) {
	return nil, fmt.Errorf("VRF listener binding is unsupported on %s; set --web.vrf= to use exporter-toolkit listener handling", runtime.GOOS)
}

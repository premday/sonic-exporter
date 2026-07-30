//go:build linux

package main

import (
	"context"
	"fmt"
	"net"
	"syscall"
)

var (
	listenTCPWithConfig = func(ctx context.Context, config net.ListenConfig, network, address string) (net.Listener, error) {
		return config.Listen(ctx, network, address)
	}
	setSocketOption = syscall.SetsockoptString
)

func listenTCPInVRF(ctx context.Context, address, vrf string) (net.Listener, error) {
	listener, err := listenTCPWithConfig(ctx, net.ListenConfig{Control: bindSocketToDevice(vrf)}, "tcp", address)
	if err != nil {
		return nil, fmt.Errorf("listen TCP: %w", err)
	}
	return listener, nil
}

func bindSocketToDevice(vrf string) func(string, string, syscall.RawConn) error {
	return func(_ string, _ string, rawConn syscall.RawConn) error {
		var socketErr error
		if err := rawConn.Control(func(fd uintptr) {
			socketErr = setSocketOption(int(fd), syscall.SOL_SOCKET, syscall.SO_BINDTODEVICE, vrf)
		}); err != nil {
			return fmt.Errorf("access socket file descriptor for VRF %q: %w", vrf, err)
		}
		if socketErr != nil {
			return fmt.Errorf("set SO_BINDTODEVICE for VRF %q: %w", vrf, socketErr)
		}
		return nil
	}
}

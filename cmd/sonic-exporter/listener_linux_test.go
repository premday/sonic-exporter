//go:build linux

package main

import (
	"context"
	"errors"
	"net"
	"strings"
	"syscall"
	"testing"
)

func TestListenTCPInVRF_usesBindToDeviceSocketControl(t *testing.T) {
	// Given
	wantListener := &testListener{}
	requestContext := context.WithValue(context.Background(), testContextKey{}, "test")
	previousListen := listenTCPWithConfig
	previousSetSocketOption := setSocketOption
	listenTCPWithConfig = func(gotContext context.Context, config net.ListenConfig, network, address string) (net.Listener, error) {
		if gotContext != requestContext {
			t.Error("listener received a different context")
		}
		if network != "tcp" {
			t.Errorf("network = %q, want tcp", network)
		}
		if address != ":9101" {
			t.Errorf("address = %q, want :9101", address)
		}
		if config.Control == nil {
			t.Fatal("ListenConfig.Control is nil")
		}
		if err := config.Control("tcp", address, testRawConn{}); err != nil {
			t.Fatalf("run socket control: %v", err)
		}
		return wantListener, nil
	}
	setSocketOption = func(fd, level, option int, value string) error {
		if fd != 17 {
			t.Errorf("file descriptor = %d, want 17", fd)
		}
		if level != syscall.SOL_SOCKET {
			t.Errorf("socket level = %d, want %d", level, syscall.SOL_SOCKET)
		}
		if option != syscall.SO_BINDTODEVICE {
			t.Errorf("socket option = %d, want %d", option, syscall.SO_BINDTODEVICE)
		}
		if value != "mgmt" {
			t.Errorf("VRF device = %q, want mgmt", value)
		}
		return nil
	}
	t.Cleanup(func() {
		listenTCPWithConfig = previousListen
		setSocketOption = previousSetSocketOption
	})

	// When
	listener, err := listenTCPInVRF(requestContext, ":9101", "mgmt")

	// Then
	if err != nil {
		t.Fatalf("listen TCP in VRF: %v", err)
	}
	if listener != wantListener {
		t.Errorf("listener = %v, want %v", listener, wantListener)
	}
}

func TestBindSocketToDevice_preservesControlError_withVRFContext(t *testing.T) {
	// Given
	controlErr := errors.New("control failed")
	rawConn := testRawConn{controlErr: controlErr}

	// When
	err := bindSocketToDevice("mgmt")("tcp", ":9101", rawConn)

	// Then
	if !errors.Is(err, controlErr) {
		t.Errorf("control error = %v, want wrapped %v", err, controlErr)
	}
	if !strings.Contains(err.Error(), "mgmt") {
		t.Errorf("control error = %q, want VRF context", err)
	}
}

func TestBindSocketToDevice_preservesSocketOptionError_withVRFContext(t *testing.T) {
	// Given
	socketOptionErr := errors.New("socket option failed")
	previousSetSocketOption := setSocketOption
	setSocketOption = func(fd, level, option int, value string) error {
		return socketOptionErr
	}
	t.Cleanup(func() {
		setSocketOption = previousSetSocketOption
	})

	// When
	err := bindSocketToDevice("mgmt")("tcp", ":9101", testRawConn{})

	// Then
	if !errors.Is(err, socketOptionErr) {
		t.Errorf("socket option error = %v, want wrapped %v", err, socketOptionErr)
	}
	if !strings.Contains(err.Error(), "mgmt") {
		t.Errorf("socket option error = %q, want VRF context", err)
	}
}

type testRawConn struct {
	controlErr error
}

func (connection testRawConn) Control(callback func(fd uintptr)) error {
	if connection.controlErr != nil {
		return connection.controlErr
	}
	callback(17)
	return nil
}

func (testRawConn) Read(func(fd uintptr) (done bool)) error {
	return nil
}

func (testRawConn) Write(func(fd uintptr) (done bool)) error {
	return nil
}

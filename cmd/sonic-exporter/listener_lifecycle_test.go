package main

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"testing"

	"github.com/prometheus/exporter-toolkit/web"
)

func TestWebServer_closesPreBoundListeners_whenServeMultipleFails(t *testing.T) {
	// Given
	serveErr := errors.New("serve failed")
	closeErr := errors.New("close failed")
	listeners := []*lifecycleListener{{}, {closeErr: closeErr}}
	previousListenerFactory := listenWebAddress
	listenWebAddress = func(_ context.Context, address, vrf string) (net.Listener, error) {
		if address == ":9101" {
			return listeners[0], nil
		}
		return listeners[1], nil
	}
	t.Cleanup(func() {
		listenWebAddress = previousListenerFactory
	})

	previousServeMultiple := serveMultiple
	serveMultiple = func([]net.Listener, *http.Server, *web.FlagConfig, *slog.Logger) error {
		return serveErr
	}
	t.Cleanup(func() {
		serveMultiple = previousServeMultiple
	})

	server := webServer{
		server: &http.Server{},
		config: testWebFlagConfig([]string{":9101", ":9200"}, false),
		logger: slog.Default(),
	}

	// When
	err := server.serve(context.Background(), "mgmt")

	// Then
	if !errors.Is(err, serveErr) {
		t.Errorf("serve error = %v, want wrapped %v", err, serveErr)
	}
	if !errors.Is(err, closeErr) {
		t.Errorf("close error = %v, want wrapped %v", err, closeErr)
	}
	for index, listener := range listeners {
		if !listener.closed {
			t.Errorf("listener %d was not closed", index)
		}
	}
}

type lifecycleListener struct {
	closed   bool
	closeErr error
}

func (listener *lifecycleListener) Accept() (net.Conn, error) {
	return nil, errors.New("not implemented")
}

func (listener *lifecycleListener) Close() error {
	listener.closed = true
	return listener.closeErr
}

func (listener *lifecycleListener) Addr() net.Addr {
	return testAddress("lifecycle")
}

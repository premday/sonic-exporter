package main

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"testing"

	"github.com/alecthomas/kingpin/v2"
	"github.com/prometheus/exporter-toolkit/web"
)

func TestAddWebVRFFlag_defaultsToManagementVRF(t *testing.T) {
	// Given
	application := kingpin.New("sonic-exporter", "test")
	webVRF := addWebVRFFlag(application)

	// When
	_, err := application.Parse(nil)

	// Then
	if err != nil {
		t.Fatalf("parse flags: %v", err)
	}
	if got, want := *webVRF, "mgmt"; got != want {
		t.Errorf("web VRF default = %q, want %q", got, want)
	}
}

func TestAddWebVRFFlag_preservesExplicitValue(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "custom VRF", args: []string{"--web.vrf=management"}, want: "management"},
		{name: "disabled VRF", args: []string{"--web.vrf="}, want: ""},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Given
			application := kingpin.New("sonic-exporter", "test")
			webVRF := addWebVRFFlag(application)

			// When
			_, err := application.Parse(test.args)

			// Then
			if err != nil {
				t.Fatalf("parse flags: %v", err)
			}
			if got := *webVRF; got != test.want {
				t.Errorf("web VRF = %q, want %q", got, test.want)
			}
		})
	}
}

func TestBindWebListeners_closesEarlierListeners_whenLaterBindFails(t *testing.T) {
	// Given
	firstListener := &testListener{}
	bindErr := errors.New("bind failed")
	previousListenerFactory := listenWebAddress
	listenWebAddress = func(_ context.Context, address, vrf string) (net.Listener, error) {
		if address == ":9200" {
			return nil, bindErr
		}
		return firstListener, nil
	}
	t.Cleanup(func() {
		listenWebAddress = previousListenerFactory
	})

	// When
	_, err := bindWebListeners(context.Background(), []string{":9101", ":9200"}, "mgmt")

	// Then
	if !errors.Is(err, bindErr) {
		t.Errorf("bind error = %v, want wrapped %v", err, bindErr)
	}
	if !strings.Contains(err.Error(), ":9200") || !strings.Contains(err.Error(), "mgmt") {
		t.Errorf("bind error = %q, want address and VRF context", err)
	}
	if !firstListener.closed {
		t.Error("first listener was not closed after a later bind failed")
	}
}

func TestWebServer_usesExporterToolkitListenerPath_whenVRFDisabled(t *testing.T) {
	// Given
	server := webServer{
		server: &http.Server{},
		config: testWebFlagConfig([]string{":9101"}, false),
		logger: slog.Default(),
	}
	serveErr := errors.New("serve failed")
	previousListenAndServe := listenAndServe
	listenAndServe = func(server *http.Server, flags *web.FlagConfig, logger *slog.Logger) error {
		return serveErr
	}
	t.Cleanup(func() {
		listenAndServe = previousListenAndServe
	})

	// When
	err := server.serve(context.Background(), "")

	// Then
	if !errors.Is(err, serveErr) {
		t.Errorf("serve error = %v, want %v", err, serveErr)
	}
}

func TestWebServer_usesPreBoundListeners_whenVRFEnabled(t *testing.T) {
	// Given
	listeners := []net.Listener{&testListener{}, &testListener{}}
	requestContext := context.WithValue(context.Background(), testContextKey{}, "test")
	previousListenerFactory := listenWebAddress
	listenWebAddress = func(gotContext context.Context, address, vrf string) (net.Listener, error) {
		if gotContext != requestContext {
			t.Error("listener factory received a different context")
		}
		if vrf != "management" {
			t.Errorf("VRF = %q, want %q", vrf, "management")
		}
		if address == ":9101" {
			return listeners[0], nil
		}
		return listeners[1], nil
	}
	t.Cleanup(func() {
		listenWebAddress = previousListenerFactory
	})

	config := testWebFlagConfig([]string{":9101", ":9200"}, false)
	server := webServer{server: &http.Server{}, config: config, logger: slog.Default()}
	previousServeMultiple := serveMultiple
	serveMultiple = func(gotListeners []net.Listener, gotServer *http.Server, gotConfig *web.FlagConfig, gotLogger *slog.Logger) error {
		if gotServer != server.server {
			t.Error("ServeMultiple received a different HTTP server")
		}
		if gotConfig != config {
			t.Error("ServeMultiple received a different web configuration")
		}
		if gotLogger != server.logger {
			t.Error("ServeMultiple received a different logger")
		}
		if len(gotListeners) != len(listeners) {
			t.Errorf("listener count = %d, want %d", len(gotListeners), len(listeners))
		}
		for index, listener := range gotListeners {
			if listener != listeners[index] {
				t.Errorf("listener %d = %v, want %v", index, listener, listeners[index])
			}
		}
		return nil
	}
	t.Cleanup(func() {
		serveMultiple = previousServeMultiple
	})

	// When
	err := server.serve(requestContext, "management")

	// Then
	if err != nil {
		t.Fatalf("serve: %v", err)
	}
}

func TestWebServer_rejectsSystemdSocketActivation_whenVRFEnabled(t *testing.T) {
	// Given
	server := webServer{
		server: &http.Server{},
		config: testWebFlagConfig([]string{":9101"}, true),
		logger: slog.Default(),
	}

	// When
	err := server.serve(context.Background(), "mgmt")

	// Then
	if err == nil || !strings.Contains(err.Error(), "systemd") || !strings.Contains(err.Error(), "--web.vrf=") {
		t.Errorf("error = %v, want actionable systemd and --web.vrf= error", err)
	}
}

func TestWebServer_rejectsVsockAddress_whenVRFEnabled(t *testing.T) {
	// Given
	server := webServer{
		server: &http.Server{},
		config: testWebFlagConfig([]string{"vsock://9101"}, false),
		logger: slog.Default(),
	}

	// When
	err := server.serve(context.Background(), "mgmt")

	// Then
	if err == nil || !strings.Contains(err.Error(), "vsock") || !strings.Contains(err.Error(), "--web.vrf=") {
		t.Errorf("error = %v, want actionable vsock and --web.vrf= error", err)
	}
}

func testWebFlagConfig(addresses []string, systemdSocket bool) *web.FlagConfig {
	return &web.FlagConfig{
		WebListenAddresses: &addresses,
		WebSystemdSocket:   &systemdSocket,
	}
}

type testListener struct {
	closed bool
}

func (listener *testListener) Accept() (net.Conn, error) {
	return nil, errors.New("not implemented")
}

func (listener *testListener) Close() error {
	listener.closed = true
	return nil
}

func (listener *testListener) Addr() net.Addr {
	return testAddress("test")
}

type testAddress string

type testContextKey struct{}

func (address testAddress) Network() string {
	return "tcp"
}

func (address testAddress) String() string {
	return string(address)
}

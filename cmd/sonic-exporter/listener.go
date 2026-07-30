package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strings"

	"github.com/alecthomas/kingpin/v2"
	"github.com/prometheus/exporter-toolkit/web"
)

const defaultWebVRF = "mgmt"

var (
	listenWebAddress = listenTCPInVRF
	listenAndServe   = web.ListenAndServe
	serveMultiple    = web.ServeMultiple
)

type webServer struct {
	server *http.Server
	config *web.FlagConfig
	logger *slog.Logger
}

func addWebVRFFlag(application *kingpin.Application) *string {
	return application.Flag("web.vrf", "VRF device for TCP web listeners. Set empty to use exporter-toolkit listener handling.").Default(defaultWebVRF).String()
}

func (server webServer) serve(ctx context.Context, vrf string) error {
	if vrf == "" {
		return listenAndServe(server.server, server.config, server.logger)
	}
	if *server.config.WebSystemdSocket {
		return fmt.Errorf("--web.vrf %q cannot be used with systemd socket activation; set --web.vrf= to use it", vrf)
	}
	for _, address := range *server.config.WebListenAddresses {
		if strings.HasPrefix(address, "vsock://") {
			return fmt.Errorf("--web.vrf %q cannot be used with vsock listener %q; set --web.vrf= to use it", vrf, address)
		}
	}

	listeners, err := bindWebListeners(ctx, *server.config.WebListenAddresses, vrf)
	if err != nil {
		return err
	}
	serveErr := serveMultiple(listeners, server.server, server.config, server.logger)
	if serveErr == nil {
		return nil
	}
	if closeErr := closeWebListeners(listeners); closeErr != nil {
		return fmt.Errorf("serve pre-bound web listeners for VRF %q: %w", vrf, errors.Join(serveErr, closeErr))
	}
	return serveErr
}

func bindWebListeners(ctx context.Context, addresses []string, vrf string) ([]net.Listener, error) {
	listeners := make([]net.Listener, 0, len(addresses))
	for _, address := range addresses {
		listener, err := listenWebAddress(ctx, address, vrf)
		if err != nil {
			if closeErr := closeWebListeners(listeners); closeErr != nil {
				return nil, fmt.Errorf("bind web listener %q to VRF %q: %w", address, vrf, errors.Join(err, closeErr))
			}
			return nil, fmt.Errorf("bind web listener %q to VRF %q: %w", address, vrf, err)
		}
		listeners = append(listeners, listener)
	}
	return listeners, nil
}

func closeWebListeners(listeners []net.Listener) error {
	var errs []error
	for _, listener := range listeners {
		if err := listener.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

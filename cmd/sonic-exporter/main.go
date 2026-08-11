package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"

	"github.com/alecthomas/kingpin/v2"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/promslog"
	"github.com/prometheus/common/promslog/flag"
	webflag "github.com/prometheus/exporter-toolkit/web/kingpinflag"
	nodecollector "github.com/prometheus/node_exporter/collector"
	"github.com/vinted/sonic-exporter/internal/collector"
)

var nodeCollectorNames = [...]string{
	"loadavg",
	"cpu",
	"diskstats",
	"filesystem",
	"meminfo",
	"time",
	"stat",
}

func nodeCollectorArgs(rootfs string) []string {
	arguments := make([]string, 0, len(nodeCollectorNames)+1)
	for _, collectorName := range nodeCollectorNames {
		arguments = append(arguments, "--collector."+collectorName)
	}
	return append(arguments, "--path.rootfs="+rootfs)
}

func main() {
	// New kingpin instance to prevent imported code from adding flags (node exporter)
	kp := kingpin.New("sonic-exporter", "Prometheus exporter for SONiC network switches")

	var (
		webConfig   = webflag.AddFlags(kp, ":9101")
		webVRF      = addWebVRFFlag(kp)
		metricsPath = kp.Flag("web.telemetry-path", "Path under which to expose metrics.").Default("/metrics").String()
		rootfs      = kp.Flag("path.rootfs", "Root filesystem mountpoint for embedded node_exporter.").Default("/").String()
	)

	promslogConfig := &promslog.Config{}
	flag.AddFlags(kp, promslogConfig)
	kp.HelpFlag.Short('h')
	kp.UsageWriter(os.Stdout)
	if _, err := kp.Parse(os.Args[1:]); err != nil {
		slog.Error("failed to parse command line arguments", "error", err)
		os.Exit(1)
	}
	if _, err := kingpin.CommandLine.Parse(nodeCollectorArgs(*rootfs)); err != nil {
		slog.Error("failed to parse node exporter collector defaults", "error", err)
		os.Exit(1)
	}

	logger := promslog.New(promslogConfig)
	metricFilter := collector.NewMetricFilter(logger)

	// SONiC collectors
	interfaceCollector := collector.NewInterfaceCollector(logger, metricFilter)
	hwCollector := collector.NewHwCollector(logger, metricFilter)
	crmCollector := collector.NewCrmCollector(logger, metricFilter)
	queueCollector := collector.NewQueueCollector(logger, metricFilter)
	lldpCollector := collector.NewLldpCollector(logger, metricFilter)
	vlanCollector := collector.NewVlanCollector(logger, metricFilter)
	lagCollector := collector.NewLagCollector(logger, metricFilter)
	fdbCollector := collector.NewFdbCollector(logger, metricFilter)
	routingCollector := collector.NewRoutingCollector(logger, metricFilter)
	switchCollector := collector.NewSwitchCollector(logger, metricFilter)
	thermalCollector := collector.NewThermalCollector(logger, metricFilter)
	transceiverCollector := collector.NewTransceiverCollector(logger, metricFilter)
	platformHealthCollector := collector.NewPlatformHealthCollector(logger, metricFilter)
	systemCollector := collector.NewSystemCollector(logger, metricFilter)
	dockerCollector := collector.NewDockerCollector(logger, metricFilter)
	frrCollector := collector.NewFrrCollector(logger)
	prometheus.MustRegister(interfaceCollector)
	prometheus.MustRegister(hwCollector)
	prometheus.MustRegister(crmCollector)
	prometheus.MustRegister(queueCollector)
	if lldpCollector.IsEnabled() {
		prometheus.MustRegister(lldpCollector)
	}
	if vlanCollector.IsEnabled() {
		prometheus.MustRegister(vlanCollector)
	}
	if lagCollector.IsEnabled() {
		prometheus.MustRegister(lagCollector)
	}
	if fdbCollector.IsEnabled() {
		prometheus.MustRegister(fdbCollector)
	}
	if routingCollector.IsEnabled() {
		prometheus.MustRegister(routingCollector)
	}
	if switchCollector.IsEnabled() {
		prometheus.MustRegister(switchCollector)
	}
	if thermalCollector.IsEnabled() {
		prometheus.MustRegister(thermalCollector)
	}
	if transceiverCollector.IsEnabled() {
		prometheus.MustRegister(transceiverCollector)
	}
	if platformHealthCollector.IsEnabled() {
		prometheus.MustRegister(platformHealthCollector)
	}
	if systemCollector.IsEnabled() {
		prometheus.MustRegister(systemCollector)
	}
	if dockerCollector.IsEnabled() {
		prometheus.MustRegister(dockerCollector)
	}
	if frrCollector.IsEnabled() {
		prometheus.MustRegister(frrCollector)
	}

	// Node exporter collectors
	nodeCollector, err := nodecollector.NewNodeCollector(logger, nodeCollectorNames[:]...)
	if err != nil {
		logger.Error("Failed to create node collector", "error", err)
		os.Exit(1)
	}
	prometheus.MustRegister(nodeCollector)

	http.Handle(*metricsPath, promhttp.Handler())
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		_, err := w.Write([]byte(`<html>
             <head><title>Sonic Exporter</title></head>
             <body>
             <h1>Sonic Exporter</h1>
             <p><a href='` + *metricsPath + `'>Metrics</a></p>
             </body>
             </html>`))
		if err != nil {
			logger.Error("Error writing response", "error", err)
		}
	})
	srv := &http.Server{}
	if err := (webServer{server: srv, config: webConfig, logger: logger}).serve(context.Background(), *webVRF); err != nil {
		logger.Error("Error starting HTTP server", "error", err)
		os.Exit(1)
	}
}

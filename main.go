package main

import (
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/prometheus/client_golang/prometheus"
	versioncollector "github.com/prometheus/client_golang/prometheus/collectors/version"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/promslog"
	promslogflag "github.com/prometheus/common/promslog/flag"
	"github.com/prometheus/common/version"
	"github.com/prometheus/exporter-toolkit/web"
	webflag "github.com/prometheus/exporter-toolkit/web/kingpinflag"

	"github.com/bringg/cluster_autoscaler_status_exporter/internal/collector"
	"github.com/bringg/cluster_autoscaler_status_exporter/internal/source"
)

const exporterName = "cluster_autoscaler_status_exporter"

var (
	configMapNamespace = kingpin.Flag("configmap.namespace", "Namespace of the cluster-autoscaler status ConfigMap.").Default("kube-system").String()
	configMapName      = kingpin.Flag("configmap.name", "Name of the cluster-autoscaler status ConfigMap.").Default("cluster-autoscaler-status").String()
	configMapKey       = kingpin.Flag("configmap.key", "Key inside the ConfigMap holding the status document.").Default("status").String()
	statusFile         = kingpin.Flag("status.file", "Read the status document from this file instead of the Kubernetes API.").String()
	kubeconfig         = kingpin.Flag("kubernetes.kubeconfig", "Path to a kubeconfig file. Defaults to in-cluster credentials, then the standard kubeconfig lookup.").String()
	requestTimeout     = kingpin.Flag("kubernetes.timeout", "Timeout for a single Kubernetes API request.").Default("10s").Duration()
	metricsPath        = kingpin.Flag("web.telemetry-path", "Path under which to expose metrics.").Default("/metrics").String()
	toolkitFlags       = webflag.AddFlags(kingpin.CommandLine, ":8080")
)

func main() {
	promslogConfig := &promslog.Config{}
	promslogflag.AddFlags(kingpin.CommandLine, promslogConfig)
	kingpin.CommandLine.UsageWriter(os.Stdout)
	kingpin.Version(version.Print(exporterName))
	kingpin.HelpFlag.Short('h')
	kingpin.Parse()

	logger := promslog.New(promslogConfig)

	if *metricsPath == "" {
		logger.Error("web.telemetry-path must not be empty")
		os.Exit(1)
	}

	src, err := newSource()
	if err != nil {
		logger.Error("Failed to build the status source", "err", err)
		os.Exit(1)
	}

	prometheus.MustRegister(
		versioncollector.NewCollector(exporterName),
		collector.New(src, *requestTimeout, logger),
	)

	logger.Info("Starting "+exporterName, "version", version.Info(), "build_context", version.BuildContext())

	if err := serve(logger); err != nil {
		logger.Error("Server stopped", "err", err)
		os.Exit(1)
	}
}

func newSource() (collector.Source, error) {
	if *statusFile != "" {
		return source.NewFile(*statusFile), nil
	}

	client, err := source.NewKubeClient(*kubeconfig)
	if err != nil {
		return nil, err
	}

	return source.NewConfigMap(client, *configMapNamespace, *configMapName, *configMapKey), nil
}

func serve(logger *slog.Logger) error {
	http.Handle(*metricsPath, promhttp.Handler())

	if *metricsPath != "/" {
		landingPage, err := web.NewLandingPage(web.LandingConfig{
			Name:        "Cluster Autoscaler Status Exporter",
			Description: "Prometheus exporter for the cluster-autoscaler-status ConfigMap",
			Version:     version.Info(),
			Links:       []web.LandingLinks{{Address: *metricsPath, Text: "Metrics"}},
		})
		if err != nil {
			return err
		}

		http.Handle("/", landingPage)
	}

	server := &http.Server{ReadHeaderTimeout: 5 * time.Second}

	return web.ListenAndServe(server, toolkitFlags, logger)
}

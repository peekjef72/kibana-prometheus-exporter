package main

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/go-kit/log/level"
	"github.com/peekjef72/kibana-prometheus-exporter/config"
	"github.com/peekjef72/kibana-prometheus-exporter/exporter"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/expfmt"
	"github.com/prometheus/common/promlog"
	"github.com/prometheus/common/promlog/flag"
	"github.com/prometheus/common/version"
	"github.com/rs/zerolog"
	kingpin "gopkg.in/alecthomas/kingpin.v2"
)

var (
	listenAddress  = kingpin.Flag("web.listen-address", "The address to listen on for HTTP requests.").Default(":9684").String()
	metricsPath    = kingpin.Flag("web.telemetry-path", "The address to listen on for HTTP requests.").Default("/metrics").String()
	configFile     = kingpin.Flag("config-file", "Exporter configuration file.").Short('c').Default("").String()
	dry_run        = kingpin.Flag("dry-run", "Only check exporter configuration file and exit.").Short('n').Default("false").Bool()
	kibanaURI      = kingpin.Flag("kibana.uri", "The Kibana API to fetch metrics from").Default("").String()
	kibanaUsername = kingpin.Flag("kibana.username", "The username to use for Kibana API").Short('u').String()
	kibanaPassword = kingpin.Flag("kibana.password", "The password to use for Kibana API").Short('p').String()
	kibanaSkipTLS  = kingpin.Flag("kibana.skip-tls", "Skip TLS verification for TLS secured Kibana URLs").Short('d').Default("false").Bool()
	debug          = kingpin.Flag("debug", "Output verbose details during metrics collection, use for development only").Short('s').Default("false").Bool()
	wait           = kingpin.Flag("wait", "Wait for Kibana to be responsive before starting, setting this to false would cause the exporter to error out instead of waiting").Short('w').Default("false").Bool()
	namespace      = "kibana"
	exporter_name  = "kibana_exporter"
)

//***********************************************************************************************
func handler(w http.ResponseWriter, r *http.Request, exporter *exporter.Exporter) {
	params := r.URL.Query()
	target := params.Get("target")
	if target == "" {
		exporter.SetTarget(exporter.Collectors[0])
	} else {
		exporter.SetTarget(exporter.Collectors[0])

	}
	registry := prometheus.NewRegistry()
	registry.MustRegister(exporter)
	h := promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
	h.ServeHTTP(w, r)
}

//***********************************************************************************************
func main() {
	var kibanas *config.KibanaConfigs
	var err error
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnixMs

	logConfig := promlog.Config{}
	flag.AddFlags(kingpin.CommandLine, &logConfig)

	kingpin.Version(version.Print(exporter_name))
	// kingpin.VersionFlag.Short('v')
	kingpin.HelpFlag.Short('h')
	kingpin.Parse()

	logger := promlog.New(&logConfig)
	level.Info(logger).Log("msg", fmt.Sprintf("Starting %s", exporter_name), "version", version.Info())
	level.Info(logger).Log("msg", "Build context", "build_context", version.BuildContext())

	// read the configuration if not empty
	if *configFile != "" {
		kibanas, err = config.Load(*configFile)
		if err != nil {
			level.Error(logger).Log("Errmsg", fmt.Sprintf("Error loading config: %s", err))
			os.Exit(1)
		}
	}

	*kibanaURI = strings.TrimSpace(*kibanaURI)
	*kibanaUsername = strings.TrimSpace(*kibanaUsername)
	*kibanaPassword = strings.TrimSpace(*kibanaPassword)
	*kibanaURI = strings.TrimSuffix(*kibanaURI, "/")

	if *kibanaURI != "" {
		kibana := &config.KibanaConfig{
			Name:     "default",
			Protocol: "",
			Host:     "",
			Port:     "",
			Username: *kibanaUsername,
			Password: *kibanaPassword,
		}
		kibana.SetDefault(*kibanaURI, *kibanaSkipTLS, *wait)
		kibanas.Kibanas = make([]config.KibanaConfig, 0)
		kibanas.Kibanas = append(kibanas.Kibanas, *kibana)
	}
	if kibanas == nil || len(kibanas.Kibanas) == 0 {
		level.Error(logger).Log("Errmsg", "No config found.")
		os.Exit(1)
	}
	if *dry_run {
		level.Info(logger).Log("msg", "configuration OK.")
		// os.Exit(0)
	}

	collectors := make([]*exporter.KibanaCollector, 0)
	for _, kibana := range kibanas.Kibanas {
		collector, err := exporter.NewCollector(&kibana, logger)
		if err != nil {
			level.Error(logger).
				Log("msg", fmt.Sprintf("error while initializing collector: %s", err))
		}
		collectors = append(collectors, collector)
	}
	exporter, err := exporter.NewExporter(namespace, collectors, *debug, logger)
	if err != nil {
		level.Error(logger).
			Log("msg", fmt.Sprintf("error while initializing exporter: %s", err))
	}

	level.Info(logger).Log("msg", fmt.Sprintf("%s initialized", exporter_name))

	if *dry_run {

		level.Info(logger).Log("msg", fmt.Sprintf("%s runs once in dry-mode (output to stdout).", exporter_name))

		registry := prometheus.NewRegistry()
		registry.MustRegister(exporter)

		exporter.SetTarget(collectors[0])
		mfs, err := registry.Gather()
		if err != nil {
			level.Error(logger).Log("Errmsg", "Error gathering metrics", "err", err)
			os.Exit(1)
		}
		enc := expfmt.NewEncoder(os.Stdout, expfmt.FmtText)

		for _, mf := range mfs {
			err := enc.Encode(mf)
			if err != nil {
				level.Error(logger).Log("Errmsg", err)
				break
			}
		}
		if closer, ok := enc.(expfmt.Closer); ok {
			// This in particular takes care of the final "# EOF\n" line for OpenMetrics.
			closer.Close()
		}
		// if kibana.WaitKibana() {
		// 	// blocking wait for Kibana to be responsive
		// 	collector.WaitForConnection()
		// } else {
		// 	if !collector.TestConnection() {
		// 		level.Error(logger).
		// 			Log("not waiting for Kibana to be responsive")
		// 	}
		// }
		os.Exit(1)
	}

	prometheus.MustRegister(exporter)

	var landingPage = []byte(`<html>
			<head><title>Kibana Exporter</title></head>
			<body>
			<h1>Kibana Exporter</h1>
			<p><a href='` + *metricsPath + `'>Metrics</a></p>
			</body>
			</html>
	`)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=UTF-8") // nolint: errcheck
		w.Write(landingPage)                                       // nolint: errcheck
	})

	http.HandleFunc(*metricsPath, func(w http.ResponseWriter, r *http.Request) {
		handler(w, r, exporter)
	})
	level.Info(logger).Log("msg", "Listening on address", "address", *listenAddress)
	if err := http.ListenAndServe(*listenAddress, nil); err != nil {
		level.Error(logger).Log("msg", "Error starting HTTP server")
		os.Exit(1)
	}
}

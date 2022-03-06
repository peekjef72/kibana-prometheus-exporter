package exporter

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
)

// Exporter implements the prometheus.Collector interface. This will
// be used to register the metrics with Prometheus.
type Exporter struct {
	logger     log.Logger
	lock       sync.RWMutex
	Collectors []*KibanaCollector
	target     *KibanaCollector
	debug      bool

	KibanaByName map[string]*KibanaCollector

	// metrics
	status                prometheus.Gauge
	info                  *prometheus.GaugeVec
	concurrentConnections prometheus.Gauge
	uptime                prometheus.Gauge
	heapTotal             prometheus.Gauge
	heapUsed              prometheus.Gauge
	load1m                prometheus.Gauge
	load5m                prometheus.Gauge
	load15m               prometheus.Gauge
	respTimeAvg           prometheus.Gauge
	respTimeMax           prometheus.Gauge
	reqDisconnects        prometheus.Gauge
	reqTotal              prometheus.Gauge
}

var InfosLabels = []string{"version", "build"}

// NewExporter will create a Exporter struct and initialize the metrics
// that will be scraped by Prometheus. It will use the provided Kibana
// details to populate a KibanaCollector struct.
func NewExporter(namespace string, collectors []*KibanaCollector, debug bool, logger log.Logger) (*Exporter, error) {

	exporter := &Exporter{
		logger:     logger,
		Collectors: collectors,
		debug:      debug,

		// up: prometheus.NewGauge(
		// 	prometheus.GaugeOpts{
		// 		Name:      "up",
		// 		Help:      "Kibana acces is OK (0: down, 1:up)",
		// 		Namespace: namespace,
		// 	}),
		status: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name:      "status",
				Help:      "Kibana overall status (0: down, 1:up)",
				Namespace: namespace,
			}),
		info: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name:      "info",
				Help:      "Kibana overall info, version build; see labels, always 1",
				Namespace: namespace,
			}, InfosLabels),
		concurrentConnections: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name:      "concurrent_connections",
				Namespace: namespace,
				Help:      "Kibana Concurrent Connections",
			}),
		uptime: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name:      "millis_uptime",
				Namespace: namespace,
				Help:      "Kibana uptime in milliseconds",
			}),
		heapTotal: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name:      "heap_max_in_bytes",
				Namespace: namespace,
				Help:      "Kibana Heap maximum in bytes",
			}),
		heapUsed: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name:      "heap_used_in_bytes",
				Namespace: namespace,
				Help:      "Kibana Heap usage in bytes",
			}),
		load1m: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name:      "os_load_1m",
				Namespace: namespace,
				Help:      "Kibana load average 1m",
			}),
		load5m: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name:      "os_load_5m",
				Namespace: namespace,
				Help:      "Kibana load average 5m",
			}),
		load15m: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name:      "os_load_15m",
				Namespace: namespace,
				Help:      "Kibana load average 15m",
			}),
		respTimeAvg: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name:      "response_average",
				Namespace: namespace,
				Help:      "Kibana average response time in milliseconds",
			}),
		respTimeMax: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name:      "response_max",
				Namespace: namespace,
				Help:      "Kibana maximum response time in milliseconds",
			}),
		reqDisconnects: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name:      "requests_disconnects",
				Namespace: namespace,
				Help:      "Kibana request disconnections count",
			}),
		reqTotal: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name:      "requests_total",
				Namespace: namespace,
				Help:      "Kibana total request count",
			}),
	}
	// initialize the map
	exporter.KibanaByName = make(map[string]*KibanaCollector)
	// build the map with the name of each kibana's name
	for _, coll := range exporter.Collectors {
		exporter.KibanaByName[coll.kibana.Name] = coll
	}

	return exporter, nil
}

//*************************************************************************************************
//
func (e *Exporter) SetTarget(target *KibanaCollector) error {
	e.target = target
	return nil
}

//*************************************************************************************************

// try to find a kibana config that matchs the specified target's name
// target: string as specified in ymal config file.

func (e *Exporter) FindTarget(target string) *KibanaCollector {
	return e.KibanaByName[target]
}

//*************************************************************************************************

// parseMetrics will set the metrics values using the KibanaMetrics
// struct, converting values to float64 where needed.
func (e *Exporter) parseMetrics(m *KibanaMetrics) error {
	level.Debug(e.logger).
		Log("msg", "parsing received metrics from kibana")

	// any value other than "green" is assumed to be less than 1
	statusVal := 0.0
	if strings.ToLower(m.Status.Overall.State) == "green" {
		statusVal = 1.0
	}

	e.status.Set(statusVal)
	if statusVal == 1.0 {
		// info is always 1; labels may change
		labels := make([]string, len(InfosLabels))
		labels[0] = m.VersionPart.Version
		labels[1] = fmt.Sprintf("%d", m.VersionPart.Build)
		e.info.WithLabelValues(labels[:]...).Set(1.0)

		e.concurrentConnections.Set(float64(m.Metrics.ConcurrentConnections))
		e.uptime.Set(float64(m.Metrics.Process.UptimeInMillis))
		e.heapTotal.Set(float64(m.Metrics.Process.Memory.Heap.TotalInBytes))
		e.heapUsed.Set(float64(m.Metrics.Process.Memory.Heap.UsedInBytes))
		e.load1m.Set(m.Metrics.Os.Load.Load1m)
		e.load5m.Set(m.Metrics.Os.Load.Load5m)
		e.load15m.Set(m.Metrics.Os.Load.Load15m)
		e.respTimeAvg.Set(m.Metrics.ResponseTimes.AvgInMillis)
		e.respTimeMax.Set(m.Metrics.ResponseTimes.MaxInMillis)
		e.reqDisconnects.Set(float64(m.Metrics.Requests.Disconnects))
		e.reqTotal.Set(float64(m.Metrics.Requests.Total))
	}

	return nil
}

func (e *Exporter) send(ch chan<- prometheus.Metric) error {
	ch <- e.status
	if e.target.State {
		e.info.Collect(ch)
		ch <- e.concurrentConnections
		ch <- e.uptime
		ch <- e.heapTotal
		ch <- e.heapUsed
		ch <- e.load1m
		ch <- e.load5m
		ch <- e.load15m
		ch <- e.respTimeAvg
		ch <- e.respTimeMax
		ch <- e.reqDisconnects
		ch <- e.reqTotal
	}
	return nil
}

// Describe is the Exporter implementing prometheus.Collector
func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	ch <- e.status.Desc()
	e.info.Describe(ch)
	ch <- e.concurrentConnections.Desc()
	ch <- e.uptime.Desc()
	ch <- e.heapTotal.Desc()
	ch <- e.heapUsed.Desc()
	ch <- e.load1m.Desc()
	ch <- e.load5m.Desc()
	ch <- e.load15m.Desc()
	ch <- e.respTimeAvg.Desc()
	ch <- e.respTimeMax.Desc()
	ch <- e.reqDisconnects.Desc()
	ch <- e.reqTotal.Desc()
}

// Collect is the Exporter implementing prometheus.Collector
func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	level.Debug(e.logger).
		Log("msg", "a Collect() call received")

	e.lock.Lock()
	defer e.lock.Unlock()

	level.Debug(e.logger).
		Log("msg", "issueing a scrape() call to the collector")

	if e.target == nil {
		level.Error(e.logger).
			Log("msg", "target not set: Can't scrape.")
		return

	}
	metrics, err := e.target.scrape()
	if err != nil {
		level.Error(e.logger).
			Log("msg", fmt.Sprintf("error while scraping metrics from Kibana: %s", err))
	}

	if e.target.State {
		// output for debugging
		if e.debug {
			res, err := json.Marshal(metrics)
			if err != nil {
				level.Error(e.logger).
					Log("msg", fmt.Sprintf("error convert to json: %s", err))
			} else {
				level.Debug(e.logger).
					Log("msg", "returned metrics content", "metrics", res)
			}
		}

		err = e.parseMetrics(metrics)
		if err != nil {
			level.Error(e.logger).
				Log("msg", fmt.Sprintf("error while parsing metrics from Kibana: %s", err))
			return
		}
	} else {
		e.status.Set(0.0)
	}
	err = e.send(ch)
	if err != nil {
		level.Error(e.logger).
			Log("msg", fmt.Sprintf("error while responding to Prometheus with metrics: %s", err))
	}
}

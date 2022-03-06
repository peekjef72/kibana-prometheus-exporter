package exporter

import (
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/peekjef72/kibana-prometheus-exporter/config"
)

// KibanaCollector collects the Kibana information together to be used by
// the exporter to scrape metrics.
type KibanaCollector struct {
	// state of the client reachable or not
	State bool

	// config of the Kibana instance or the service
	kibana config.KibanaConfig
	// reference to global looger
	logger log.Logger

	// authHeader is the string that should be used as the value
	// for the "Authorization" header. If this is empty, it is
	// assumed that no authorization is needed.
	authHeader string

	// client is the http.Client that will be used to make
	// requests to collect the Kibana metrics
	client *http.Client
}

//"version":{"number":"7.17.1","build_hash":"78e8422ed4e7d2054bd35b82a91299b3f7bd6231","build_number":46635,"build_snapshot":false},
//"status":{"overall":{"since":"2022-03-06T10:35:22.586Z","state":"yellow","title":"Yellow","nickname":"I'll be back","icon":"warning","uiColor":"warning"}
// KibanaMetrics is used to unmarshal the metrics response from Kibana.
type KibanaMetrics struct {
	VersionPart struct {
		Version string `json:"number"`
		Build   int    `json:"build_number"`
	} `json:"version"`
	Status struct {
		Overall struct {
			State string `json:"state"`
		} `json:"overall"`
	} `json:"status"`
	Metrics struct {
		ConcurrentConnections int `json:"concurrent_connections"`
		Process               struct {
			UptimeInMillis float64 `json:"uptime_in_millis"`
			Memory         struct {
				Heap struct {
					TotalInBytes int64 `json:"total_in_bytes"`
					UsedInBytes  int64 `json:"used_in_bytes"`
				} `json:"heap"`
			} `json:"memory"`
		} `json:"process"`
		Os struct {
			Load struct {
				Load1m  float64 `json:"1m"`
				Load5m  float64 `json:"5m"`
				Load15m float64 `json:"15m"`
			} `json:"load"`
		} `json:"os"`
		ResponseTimes struct {
			AvgInMillis float64 `json:"avg_in_millis"`
			MaxInMillis float64 `json:"max_in_millis"`
		} `json:"response_times"`
		Requests struct {
			Disconnects int `json:"disconnects"`
			Total       int `json:"total"`
		} `json:"requests"`
	} `json:"metrics"`
}

// TestConnection checks whether the connection to Kibana is healthy
func (c *KibanaCollector) TestConnection(logger log.Logger) bool {
	level.Debug(logger).
		Log("msg", "checking for kibana status")

	_, err := c.scrape()
	if err != nil {
		level.Info(logger).
			Log("msg", fmt.Sprintf("test connection to kibana failed: %s", err))
		return false
	}

	return true
}

// WaitForConnection is a method to block until Kibana becomes available
func (c *KibanaCollector) WaitForConnection(logger log.Logger) {
	for {
		if !c.TestConnection(logger) {
			level.Info(logger).
				Log("msg", "waiting for kibana to be responsive")

			// hardcoded since it's unlikely this is user controlled
			time.Sleep(10 * time.Second)
			continue
		}

		level.Info(logger).
			Log("msg", "kibana is up")
		return
	}
}

// NewCollector builds a KibanaCollector struct
func NewCollector(kibana *config.KibanaConfig, logger log.Logger) (*KibanaCollector, error) {
	collector := &KibanaCollector{}
	collector.kibana = *kibana
	collector.logger = logger
	if strings.HasPrefix(kibana.Protocol, "https") {
		level.Debug(logger).
			Log("msg", fmt.Sprintf("kibana URL is a TLS one: %s", kibana.Url()))

		if kibana.SkipTls() {
			level.Info(logger).
				Log("msg", fmt.Sprintf("skipping TLS verification for Kibana URL: %s", kibana.Url()))
		}

		tConf := &tls.Config{
			InsecureSkipVerify: kibana.SkipTls(),
		}

		tr := &http.Transport{
			TLSClientConfig: tConf,
		}

		collector.client = &http.Client{
			Transport: tr,
		}
	} else {
		level.Debug(logger).
			Log("msg", fmt.Sprintf("kibana URL is a plain text one: %s", kibana.Url()))

		collector.client = &http.Client{}
		if kibana.SkipTls() {
			level.Info(logger).
				Log("msg", fmt.Sprintf("kibana.skip-tls is enabled for an http URL, ignoring: %s", kibana.Url()))
		}
	}

	if kibana.Username != "" && kibana.Password != "" {
		level.Debug(logger).
			Log("msg", "using authenticated requests with Kibana")

		creds := fmt.Sprintf("%s:%s", kibana.Username, kibana.Password)
		encCreds := base64.StdEncoding.EncodeToString([]byte(creds))
		collector.authHeader = fmt.Sprintf("Basic %s", encCreds)
	} else {
		level.Info(logger).
			Log("msg", "Kibana username or password is not provided, assuming unauthenticated communication")
	}

	return collector, nil
}

// scrape will connect to the Kibana instance, using the details
// provided by the KibanaCollector struct, and return the metrics as a
// KibanaMetrics representation.
func (c *KibanaCollector) scrape() (*KibanaMetrics, error) {
	level.Debug(c.logger).
		Log("msg", "building request for api/status from kibana")

	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/api/status", c.kibana.Url()), nil)
	if err != nil {
		return nil, fmt.Errorf("could not initialize a request to scrape metrics: %s", err)
	}

	if c.authHeader != "" {
		level.Debug(c.logger).
			Log("msg", "adding auth header")
		req.Header.Add("Authorization", c.authHeader)
	}

	req.Header.Add("Accept", "application/json")

	level.Debug(c.logger).
		Log("msg", "requesting api/status from kibana")
	resp, err := c.client.Do(req)
	if err != nil {
		c.State = false
		return nil, fmt.Errorf("error while reading Kibana status: %s", err)
	}
	c.State = true

	defer resp.Body.Close()

	level.Debug(c.logger).
		Log("msg", "processing api/status response")

	if resp.StatusCode != http.StatusOK {
		c.State = false
		return nil, fmt.Errorf("invalid response from Kibana status: %s", resp.Status)
	}

	respContent, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error while reading response from Kibana status: %s", err)
	}

	metrics := &KibanaMetrics{}
	err = json.Unmarshal(respContent, &metrics)
	if err != nil {
		return nil, fmt.Errorf("error while unmarshalling Kibana status: %s\nProblematic content:\n%s", err, respContent)
	}

	return metrics, nil
}

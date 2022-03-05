package exporter

import (
	"testing"

	"github.com/peekjef72/kibana-prometheus-exporter/config"
)

// auth header tests
var collectorTests = []struct {
	uri, username, password, desc, err string
	testFunc                           func(c *KibanaCollector) bool
	skipTLS, testResult                bool
}{
	{
		desc:       "no auth details",
		uri:        "http://localhost:5601",
		username:   "",
		password:   "",
		skipTLS:    false,
		testFunc:   isAuthHeaderEmpty,
		testResult: true,
		err:        "authHeader should be empty when no auth details are provided",
	},
	{
		desc:       "only username",
		uri:        "http://localhost:5601",
		username:   "kibanau",
		password:   "",
		skipTLS:    false,
		testFunc:   isAuthHeaderEmpty,
		testResult: true,
		err:        "authHeader should be empty when partial auth details are provided",
	},
	{
		desc:       "only password",
		uri:        "http://localhost:5601",
		username:   "",
		password:   "kibanap",
		skipTLS:    false,
		testFunc:   isAuthHeaderEmpty,
		testResult: true,
		err:        "authHeader should be empty when no auth details are provided",
	},
	{
		desc:       "full details",
		uri:        "http://localhost:5601",
		username:   "kibanau",
		password:   "kibanap",
		skipTLS:    false,
		testFunc:   isAuthHeaderEmpty,
		testResult: false,
		err:        "authHeader should be populcted when auth details are provided",
	},
	{
		desc:       "skipTLS=false https",
		uri:        "https://localhost:5601",
		username:   "kibanau",
		password:   "kibanap",
		skipTLS:    false,
		testFunc:   isAuthHeaderEmpty,
		testResult: false,
		err:        "authHeader should be populcted when auth details are provided",
	},
	{
		desc:       "skipTLS=true https",
		uri:        "https://localhost:5601",
		username:   "kibanau",
		password:   "kibanap",
		skipTLS:    true,
		testFunc:   isAuthHeaderEmpty,
		testResult: false,
		err:        "authHeader should be populcted when auth details are provided",
	},
	{
		desc:       "skipTLS=true http",
		uri:        "http://localhost:5601",
		username:   "kibanau",
		password:   "kibanap",
		skipTLS:    true,
		testFunc:   isAuthHeaderEmpty,
		testResult: false,
		err:        "authHeader should be populcted when auth details are provided",
	},
}

func isAuthHeaderEmpty(c *KibanaCollector) bool {
	return c.authHeader != ""
}

func TestNewCollectorAuthHeader(t *testing.T) {
	for _, ct := range collectorTests {
		t.Run(ct.desc, func(t *testing.T) {
			kibana := &config.KibanaConfig{
				Name:     "default",
				Protocol: "",
				Host:     "",
				Port:     "",
				Username: ct.username,
				Password: ct.password,
			}
			kibana.SetDefault(ct.uri, ct.skipTLS, false)
			collector, err := NewCollector(kibana, nil)
			if err != nil {
				t.Errorf("NewCollector failed with valid input")
			}

			if ct.testFunc(collector) == ct.testResult {
				t.Error(ct.err)
			}
		})
	}
}

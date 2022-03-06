package config

import (
	"fmt"
	"io/ioutil"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v2"
)

type KibanaConfigs struct {
	Kibanas []KibanaConfig `yaml:"kibanas"`

	// Catches all undefined fields and must be empty after parsing.
	XXX map[string]interface{} `yaml:",inline" json:"-"`
}

type KibanaConfig struct {
	Name     string `yaml:"name"`
	Protocol string `yaml:"protocol,omitempty"`
	Host     string `yaml:"host,omitempty"`
	Port     string `yaml:"port,omitempty"`
	Username string `yaml:"username,omitempty"`
	Password string `yaml:"password,omitempty"`
	Skip     string `yaml:"skip-tls,omitempty"`
	Wait     string `yaml:"wait,omitempty"`

	uri  string
	skip bool
	wait bool
}

// *************************************************************
//
// *************************************************************
// Load attempts to parse the given config file and return a Config object.
func Load(configFile string) (*KibanaConfigs, error) {
	//	log.Infof("Loading profiles from %s", profilesFile)
	buf, err := ioutil.ReadFile(configFile)
	if err != nil {
		return nil, err
	}

	kibanas := KibanaConfigs{}
	err = yaml.Unmarshal(buf, &kibanas)
	if err != nil {
		return nil, err
	}
	err = kibanas.check()
	if err != nil {
		return nil, err
	}

	err = checkOverflow(kibanas.XXX, "kibanas")
	if err != nil {
		return nil, err
	}

	return &kibanas, nil
}

// *************************************************************
//
// KibanaConfigs: list of KibanaConfig
//
// *************************************************************
// check the sanity of the sockets in the set
func (c *KibanaConfigs) check() error {
	if len(c.Kibanas) == 0 {
		return fmt.Errorf("no valid config found")
	}
	for index := range c.Kibanas {
		err := c.Kibanas[index].check()
		if err != nil {
			return (err)
		}
	}
	return (nil)
}

// *************************************************************
//
// KibanaConfig: one element of Config (for KibanaConfigs)
//
// *************************************************************
// *************************************************************
// UnmarshalYAML implements the yaml.Unmarshaler interface for ProfileConfig.
// func (c *KibanaConfig) UnmarshalYAML(unmarshal func(interface{}) error) error {

// 	type plain KibanaConfig
// 	if err := unmarshal((*plain)(c)); err != nil {
// 		return err
// 	}

// 	if c.check() != nil {
// 		return fmt.Errorf("no commands defined for profile %q", c.Name)
// 	}

// 	return checkOverflow(c.XXX, "profiles")
// }
func parseBool(val string) (bool, error) {
	val = strings.ToLower(val)
	if val == "no" {
		val = "0"
	} else if val == "yes" {
		val = "1"
	}
	return strconv.ParseBool(val)
}

// *************************************************************
// Check the sanity of the socket and fills the default values
func (c *KibanaConfig) check() error {

	if c.Name == "" {
		return (fmt.Errorf("config must have the field name set"))
	}

	if c.Protocol == "" {
		c.Protocol = "http"
	}

	if c.Host == "" {
		c.Host = "localhost"
	}

	if c.Port == "" {
		c.Port = "5601"
	}

	if c.Skip == "" {
		c.skip = false
	} else {
		var err error
		c.skip, err = parseBool(c.Skip)
		if err != nil {
			return err
		}
	}

	if c.Wait == "" {
		c.wait = false
	} else {
		var err error
		c.wait, err = parseBool(c.Wait)
		if err != nil {
			return err
		}
	}

	c.uri = c.url()

	return (nil)
}

func (c *KibanaConfig) url() string {
	var s strings.Builder

	s.WriteString(fmt.Sprintf("%s://%s:%s", c.Protocol, c.Host, c.Port))

	return s.String()

}
func (c *KibanaConfig) SetDefault(url string, skipTls bool, wait bool) error {
	urlLineRE := regexp.MustCompile(`^(https?)://([^:]+)(?::(\d+))`)
	match := urlLineRE.FindStringSubmatch(url)
	if match != nil {
		c.Protocol = match[1]
		c.Host = match[2]
		c.Port = match[3]
	}
	c.uri = url
	c.skip = skipTls
	c.wait = wait

	return nil
}

func (c *KibanaConfig) Url() string {
	return c.uri
}

func (c *KibanaConfig) SkipTls() bool {
	return c.skip
}

func (c *KibanaConfig) WaitKibana() bool {
	return c.wait
}

// to catch unwanted params in config file
func checkOverflow(m map[string]interface{}, ctx string) error {
	if len(m) > 0 {
		var keys []string
		for k := range m {
			keys = append(keys, k)
		}
		return fmt.Errorf("unknown fields in %s: %s", ctx, strings.Join(keys, ", "))
	}
	return nil
}

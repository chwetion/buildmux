// Package config loads and validates buildmux YAML configuration.
package config

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"text/template"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Version   int                 `yaml:"version"`
	Platforms map[string]Platform `yaml:"platforms"`
	Manifest  ManifestSettings    `yaml:"manifest"`
}

type Platform struct {
	Endpoint string `yaml:"endpoint"`
	Tag      string `yaml:"tag"`
	TLS      *TLS   `yaml:"tls,omitempty"`
}

type TLS struct {
	CACert     string `yaml:"ca_cert"`
	Cert       string `yaml:"cert"`
	Key        string `yaml:"key"`
	ServerName string `yaml:"server_name,omitempty"`
}

type ManifestSettings struct {
	Insecure bool   `yaml:"insecure"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

func Load(path string) (*Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %q: %w", path, err)
	}
	var cfg Config
	dec := yaml.NewDecoder(bytes.NewReader(b))
	dec.KnownFields(true)
	if err := dec.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("parse config %q: %w", path, err)
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config %q: %w", path, err)
	}
	return &cfg, nil
}

func (c *Config) Validate() error {
	if c.Version != 1 {
		return fmt.Errorf("unsupported version %d (only 1 is supported)", c.Version)
	}
	if len(c.Platforms) == 0 {
		return errors.New("platforms: at least one platform must be defined")
	}
	for name, p := range c.Platforms {
		if p.Endpoint == "" {
			return fmt.Errorf("platforms[%q]: endpoint is required", name)
		}
		if p.Tag == "" {
			return fmt.Errorf("platforms[%q]: tag is required", name)
		}
		if _, err := template.New(name).Parse(p.Tag); err != nil {
			return fmt.Errorf("platforms[%q]: tag template: %w", name, err)
		}
		if p.TLS != nil {
			if p.TLS.CACert == "" || p.TLS.Cert == "" || p.TLS.Key == "" {
				return fmt.Errorf("platforms[%q]: tls requires ca_cert, cert, and key (all three)", name)
			}
		}
	}
	return nil
}

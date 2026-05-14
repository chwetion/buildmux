package config

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadValid(t *testing.T) {
	cfg, err := Load(filepath.Join("testdata", "valid.yaml"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Version != 1 {
		t.Errorf("version=%d want 1", cfg.Version)
	}
	if len(cfg.Platforms) != 2 {
		t.Errorf("platforms=%d want 2", len(cfg.Platforms))
	}
	p := cfg.Platforms["linux/amd64"]
	if p.Endpoint != "tcp://buildkit-amd64.internal:1234" {
		t.Errorf("endpoint mismatch: %q", p.Endpoint)
	}
	if p.TLS == nil || p.TLS.Key == "" {
		t.Errorf("tls block not loaded")
	}
}

func TestLoadTLSPartial(t *testing.T) {
	_, err := Load(filepath.Join("testdata", "tls_partial.yaml"))
	if err == nil {
		t.Fatal("expected error for incomplete TLS block")
	}
	if !strings.Contains(err.Error(), "tls") {
		t.Errorf("error should mention tls, got: %v", err)
	}
}

func TestValidate(t *testing.T) {
	cases := []struct {
		name    string
		cfg     Config
		wantErr string
	}{
		{
			name:    "bad version",
			cfg:     Config{Version: 2, Platforms: map[string]Platform{"linux/amd64": {Endpoint: "x", Tag: "{{.Name}}"}}},
			wantErr: "version",
		},
		{
			name:    "empty platforms",
			cfg:     Config{Version: 1, Platforms: map[string]Platform{}},
			wantErr: "platforms",
		},
		{
			name:    "missing endpoint",
			cfg:     Config{Version: 1, Platforms: map[string]Platform{"linux/amd64": {Tag: "{{.Name}}"}}},
			wantErr: "endpoint",
		},
		{
			name:    "missing tag",
			cfg:     Config{Version: 1, Platforms: map[string]Platform{"linux/amd64": {Endpoint: "x"}}},
			wantErr: "tag",
		},
		{
			name: "bad tag template",
			cfg: Config{Version: 1, Platforms: map[string]Platform{
				"linux/amd64": {Endpoint: "x", Tag: "{{.Name"},
			}},
			wantErr: "template",
		},
		{
			name: "tls all three present",
			cfg: Config{Version: 1, Platforms: map[string]Platform{
				"linux/amd64": {Endpoint: "x", Tag: "{{.Name}}", TLS: &TLS{CACert: "a", Cert: "c", Key: "k"}},
			}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cfg.Validate()
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("err=%v, want substring %q", err, tc.wantErr)
			}
		})
	}
}

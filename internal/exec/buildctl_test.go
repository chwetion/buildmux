package exec

import (
	"reflect"
	"testing"

	"github.com/chwetion/buildmux/internal/config"
)

func TestBuildctlInvocation(t *testing.T) {
	platformCfg := &config.Platform{
		Endpoint: "tcp://amd64:1234",
		Tag:      "{{.Name}}-amd64",
	}
	rest := []string{"--frontend", "dockerfile.v0", "--local", "context=."}
	finalOutput := "type=image,name=foo:v1-amd64,push=true"

	got := BuildctlInvocation("/usr/bin/buildctl", "linux/amd64", platformCfg, rest, finalOutput)
	wantArgs := []string{
		"--addr", "tcp://amd64:1234",
		"build",
		"--frontend", "dockerfile.v0",
		"--local", "context=.",
		"--opt", "platform=linux/amd64",
		"--output", "type=image,name=foo:v1-amd64,push=true",
	}
	if got.Binary != "/usr/bin/buildctl" {
		t.Errorf("binary got %q", got.Binary)
	}
	if !reflect.DeepEqual(got.Args, wantArgs) {
		t.Errorf("args got %v\nwant %v", got.Args, wantArgs)
	}
}

func TestBuildctlInvocationTLS(t *testing.T) {
	platformCfg := &config.Platform{
		Endpoint: "tcp://amd64:1234",
		Tag:      "{{.Name}}-amd64",
		TLS: &config.TLS{
			CACert:     "/etc/ca.pem",
			Cert:       "/etc/cert.pem",
			Key:        "/etc/key.pem",
			ServerName: "amd64.internal",
		},
	}
	got := BuildctlInvocation("buildctl", "linux/amd64", platformCfg, nil, "type=image,name=foo,push=true")
	want := []string{
		"--addr", "tcp://amd64:1234",
		"--tlscacert", "/etc/ca.pem",
		"--tlscert", "/etc/cert.pem",
		"--tlskey", "/etc/key.pem",
		"--tlsservername", "amd64.internal",
		"build",
		"--opt", "platform=linux/amd64",
		"--output", "type=image,name=foo,push=true",
	}
	if !reflect.DeepEqual(got.Args, want) {
		t.Errorf("args got %v\nwant %v", got.Args, want)
	}
}

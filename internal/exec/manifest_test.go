package exec

import (
	"reflect"
	"strings"
	"testing"

	"github.com/chwetion/buildmux/internal/config"
)

func TestBuildManifestSpec(t *testing.T) {
	entries := []ManifestEntry{
		{Platform: "linux/amd64", Image: "foo:v1-amd64"},
		{Platform: "linux/arm/v7", Image: "foo:v1-armv7"},
	}
	got, err := BuildManifestSpec("foo:v1", entries)
	if err != nil {
		t.Fatalf("BuildManifestSpec: %v", err)
	}
	for _, want := range []string{
		"image: foo:v1",
		"image: foo:v1-amd64",
		"image: foo:v1-armv7",
		"os: linux",
		"architecture: amd64",
		"architecture: arm",
		"variant: v7",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("spec missing %q\n--- spec ---\n%s", want, got)
		}
	}
}

func TestManifestToolInvocation(t *testing.T) {
	got := ManifestToolInvocation("manifest-tool", config.ManifestSettings{}, "/tmp/spec.yaml")
	want := []string{"push", "from-spec", "/tmp/spec.yaml"}
	if !reflect.DeepEqual(got.Args, want) {
		t.Errorf("args got %v want %v", got.Args, want)
	}
}

func TestManifestToolInvocationWithCreds(t *testing.T) {
	got := ManifestToolInvocation("manifest-tool", config.ManifestSettings{
		Username: "u",
		Password: "p",
		Insecure: true,
	}, "/tmp/spec.yaml")
	want := []string{
		"--username", "u",
		"--password", "p",
		"--insecure",
		"push", "from-spec", "/tmp/spec.yaml",
	}
	if !reflect.DeepEqual(got.Args, want) {
		t.Errorf("args got %v want %v", got.Args, want)
	}
}

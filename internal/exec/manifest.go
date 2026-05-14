package exec

import (
	"bytes"
	"fmt"

	"gopkg.in/yaml.v3"

	"github.com/chwetion/buildmux/internal/config"
)

type ManifestEntry struct {
	Platform string // e.g. "linux/amd64"
	Image    string // fully rewritten per-arch ref
}

type manifestSpec struct {
	Image     string             `yaml:"image"`
	Manifests []manifestSpecItem `yaml:"manifests"`
}

type manifestSpecItem struct {
	Image    string               `yaml:"image"`
	Platform manifestSpecPlatform `yaml:"platform"`
}

type manifestSpecPlatform struct {
	OS           string `yaml:"os"`
	Architecture string `yaml:"architecture"`
	Variant      string `yaml:"variant,omitempty"`
}

// BuildManifestSpec returns the YAML content for `manifest-tool push from-spec`.
func BuildManifestSpec(target string, entries []ManifestEntry) (string, error) {
	spec := manifestSpec{Image: target}
	for _, e := range entries {
		pp, err := config.SplitPlatform(e.Platform)
		if err != nil {
			return "", fmt.Errorf("manifest entry for %q: %w", e.Platform, err)
		}
		spec.Manifests = append(spec.Manifests, manifestSpecItem{
			Image: e.Image,
			Platform: manifestSpecPlatform{
				OS:           pp.OS,
				Architecture: pp.Arch,
				Variant:      pp.Variant,
			},
		})
	}
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(&spec); err != nil {
		return "", err
	}
	if err := enc.Close(); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func ManifestToolInvocation(binary string, settings config.ManifestSettings, specPath string) Invocation {
	args := make([]string, 0, 8)
	if settings.Username != "" {
		args = append(args, "--username", settings.Username)
	}
	if settings.Password != "" {
		args = append(args, "--password", settings.Password)
	}
	if settings.Insecure {
		args = append(args, "--insecure")
	}
	args = append(args, "push", "from-spec", specPath)
	return Invocation{Binary: binary, Args: args}
}

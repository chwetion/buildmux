//go:build integration

package build

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	bxexec "github.com/chwetion/buildmux/internal/exec"
)

func buildStub(t *testing.T, src, outName string) string {
	t.Helper()
	dir := t.TempDir()
	out := filepath.Join(dir, outName)
	cmd := exec.Command("go", "build", "-o", out, src)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("build stub %s: %v", src, err)
	}
	return out
}

func TestSmokeHappyPath(t *testing.T) {
	repoRoot, _ := filepath.Abs("../../..")
	buildctl := buildStub(t, filepath.Join(repoRoot, "testdata/stubs/fake-buildctl"), "fake-buildctl")
	mtool := buildStub(t, filepath.Join(repoRoot, "testdata/stubs/fake-manifest-tool"), "fake-manifest-tool")

	cfg := `
version: 1
platforms:
  linux/amd64:
    endpoint: tcp://amd64:1
    tag: "{{.Name}}-amd64"
  linux/arm64:
    endpoint: tcp://arm64:1
    tag: "{{.Name}}-arm64"
`
	cfgPath := filepath.Join(t.TempDir(), "buildmux.yaml")
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}

	opts := Options{
		ConfigPath:       cfgPath,
		BuildctlPath:     buildctl,
		ManifestToolPath: mtool,
		Args: []string{
			"--frontend", "dockerfile.v0",
			"--opt", "platform=linux/amd64,linux/arm64",
			"--output", "type=image,name=foo:v1,push=true",
		},
	}
	if err := Execute(context.Background(), opts, bxexec.RealRunner{}); err != nil {
		t.Fatalf("Execute: %v", err)
	}
}

func TestSmokeBuildctlFail(t *testing.T) {
	repoRoot, _ := filepath.Abs("../../..")
	buildctl := buildStub(t, filepath.Join(repoRoot, "testdata/stubs/fake-buildctl"), "fake-buildctl")
	mtool := buildStub(t, filepath.Join(repoRoot, "testdata/stubs/fake-manifest-tool"), "fake-manifest-tool")

	t.Setenv("FAKE_BUILDCTL_FAIL", "1")
	cfg := `
version: 1
platforms:
  linux/amd64:
    endpoint: tcp://amd64:1
    tag: "{{.Name}}-amd64"
`
	cfgPath := filepath.Join(t.TempDir(), "buildmux.yaml")
	_ = os.WriteFile(cfgPath, []byte(cfg), 0o644)

	opts := Options{
		ConfigPath:       cfgPath,
		BuildctlPath:     buildctl,
		ManifestToolPath: mtool,
		Args: []string{
			"--opt", "platform=linux/amd64",
			"--output", "type=image,name=foo:v1,push=true",
		},
	}
	err := Execute(context.Background(), opts, bxexec.RealRunner{})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "exit") {
		t.Logf("error: %v", err)
	}
}

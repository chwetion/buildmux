package build

import (
	"context"
	"io"
	"os"
	"strings"
	"sync"
	"testing"

	bxexec "github.com/chwetion/buildmux/internal/exec"
)

type call struct {
	binary string
	args   []string
}

type fakeRunner struct {
	mu        sync.Mutex
	calls     []call
	failOnIdx int // 1-based; 0 means none fail
}

func (f *fakeRunner) Run(ctx context.Context, inv bxexec.Invocation, stdout, stderr io.Writer) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, call{binary: inv.Binary, args: append([]string(nil), inv.Args...)})
	if f.failOnIdx > 0 && len(f.calls) == f.failOnIdx {
		return errBuildctlFailed
	}
	return nil
}

var errBuildctlFailed = stringError("buildctl failed")

type stringError string

func (e stringError) Error() string { return string(e) }

func TestExecuteHappyPath(t *testing.T) {
	cfgYAML := `
version: 1
platforms:
  linux/amd64:
    endpoint: tcp://amd64:1
    tag: "{{.Name}}-amd64"
  linux/arm64:
    endpoint: tcp://arm64:1
    tag: "{{.Name}}-arm64"
manifest:
  insecure: false
`
	dir := t.TempDir()
	cfgPath := writeFile(t, dir, "buildmux.yaml", cfgYAML)

	runner := &fakeRunner{}
	opts := Options{
		ConfigPath:       cfgPath,
		BuildctlPath:     "buildctl",
		ManifestToolPath: "manifest-tool",
		Args: []string{
			"--frontend", "dockerfile.v0",
			"--opt", "platform=linux/amd64,linux/arm64",
			"--output", "type=image,name=foo:v1,push=true",
		},
	}
	if err := Execute(context.Background(), opts, runner); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if len(runner.calls) != 3 {
		t.Fatalf("got %d calls, want 3", len(runner.calls))
	}

	// Two buildctl calls (order is non-deterministic), then one manifest-tool call.
	manifestCall := runner.calls[2]
	if manifestCall.binary != "manifest-tool" {
		t.Errorf("3rd call binary=%q want manifest-tool", manifestCall.binary)
	}
	if !containsAll(manifestCall.args, []string{"push", "from-spec"}) {
		t.Errorf("manifest call args missing from-spec: %v", manifestCall.args)
	}

	for _, c := range runner.calls[:2] {
		if c.binary != "buildctl" {
			t.Errorf("expected buildctl, got %q", c.binary)
		}
		addr := argAfter(c.args, "--addr")
		if addr != "tcp://amd64:1" && addr != "tcp://arm64:1" {
			t.Errorf("unexpected --addr %q", addr)
		}
		if !containsPair(c.args, "--opt", "platform=linux/amd64") &&
			!containsPair(c.args, "--opt", "platform=linux/arm64") {
			t.Errorf("buildctl args missing single-platform --opt: %v", c.args)
		}
		out := argAfter(c.args, "--output")
		if !strings.Contains(out, "name=foo:v1-amd64,") && !strings.Contains(out, "name=foo:v1-arm64,") {
			t.Errorf("--output not rewritten: %q", out)
		}
	}
}

func TestExecuteBuildctlFailSkipsManifest(t *testing.T) {
	cfgYAML := `
version: 1
platforms:
  linux/amd64:
    endpoint: tcp://amd64:1
    tag: "{{.Name}}-amd64"
manifest:
  insecure: false
`
	dir := t.TempDir()
	cfgPath := writeFile(t, dir, "buildmux.yaml", cfgYAML)

	runner := &fakeRunner{failOnIdx: 1}
	opts := Options{
		ConfigPath:       cfgPath,
		BuildctlPath:     "buildctl",
		ManifestToolPath: "manifest-tool",
		Args: []string{
			"--opt", "platform=linux/amd64",
			"--output", "type=image,name=foo:v1,push=true",
		},
	}
	if err := Execute(context.Background(), opts, runner); err == nil {
		t.Fatal("expected error")
	}
	for _, c := range runner.calls {
		if c.binary == "manifest-tool" {
			t.Errorf("manifest-tool should not be called when buildctl failed")
		}
	}
}

func TestExecuteMissingPlatformInConfig(t *testing.T) {
	cfgYAML := `
version: 1
platforms:
  linux/amd64:
    endpoint: tcp://amd64:1
    tag: "{{.Name}}-amd64"
`
	dir := t.TempDir()
	cfgPath := writeFile(t, dir, "buildmux.yaml", cfgYAML)

	runner := &fakeRunner{}
	opts := Options{
		ConfigPath:       cfgPath,
		BuildctlPath:     "buildctl",
		ManifestToolPath: "manifest-tool",
		Args: []string{
			"--opt", "platform=linux/amd64,linux/arm64",
			"--output", "type=image,name=foo:v1,push=true",
		},
	}
	err := Execute(context.Background(), opts, runner)
	if err == nil {
		t.Fatal("expected error for missing platform")
	}
	if !strings.Contains(err.Error(), "linux/arm64") {
		t.Errorf("error should name the missing platform, got: %v", err)
	}
	if len(runner.calls) != 0 {
		t.Errorf("no runner calls should be made when config is missing a platform, got %d", len(runner.calls))
	}
}

// helpers

func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := dir + "/" + name
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func argAfter(args []string, key string) string {
	for i := 0; i < len(args)-1; i++ {
		if args[i] == key {
			return args[i+1]
		}
	}
	return ""
}

func containsAll(args []string, need []string) bool {
	for _, n := range need {
		found := false
		for _, a := range args {
			if a == n {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func containsPair(args []string, k, v string) bool {
	for i := 0; i < len(args)-1; i++ {
		if args[i] == k && args[i+1] == v {
			return true
		}
	}
	return false
}

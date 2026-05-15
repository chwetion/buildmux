package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"

	"github.com/chwetion/buildmux/internal/cli/build"
	bxexec "github.com/chwetion/buildmux/internal/exec"
)

// Populated at build time via -ldflags "-X main.version=..." etc.
// goreleaser injects these on tagged releases; `go build` from source leaves them at the defaults.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

const usage = `buildmux: multiplex buildctl across per-platform buildkitd endpoints.

Usage:
  buildmux build [--config <path>] [--buildctl <path>] [--manifest-tool <path>] [--] [buildctl-args...]
  buildmux version

  All args after "build" are forwarded to buildctl, except that --opt platform=...
  and --output are intercepted and rewritten per-platform. Use "--" to disambiguate
  if buildctl args collide with buildmux's own flag names.

Flags:
  --config           path to YAML config (default: ./buildmux.yaml or $BUILDMUX_CONFIG)
  --buildctl         path to buildctl binary (default: looked up in PATH)
  --manifest-tool    path to manifest-tool binary (default: looked up in PATH)
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	switch os.Args[1] {
	case "build":
		os.Exit(runBuild(os.Args[2:]))
	case "version", "-v", "--version":
		fmt.Printf("buildmux %s (commit %s, built %s)\n", version, commit, date)
		return
	case "-h", "--help", "help":
		fmt.Print(usage)
		return
	default:
		fmt.Fprintf(os.Stderr, "unknown subcommand %q\n\n%s", os.Args[1], usage)
		os.Exit(2)
	}
}

func runBuild(argv []string) int {
	fs := flag.NewFlagSet("build", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	cfgPath := fs.String("config", defaultConfigPath(), "path to buildmux YAML config")
	buildctl := fs.String("buildctl", "", "path to buildctl binary (default: $PATH lookup)")
	mtool := fs.String("manifest-tool", "", "path to manifest-tool binary (default: $PATH lookup)")

	rest, err := parseLeadingFlags(fs, argv)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}

	resolvedBuildctl, err := resolveBinary(*buildctl, "buildctl")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 127
	}
	resolvedMtool, err := resolveBinary(*mtool, "manifest-tool")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 127
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	opts := build.Options{
		ConfigPath:       *cfgPath,
		BuildctlPath:     resolvedBuildctl,
		ManifestToolPath: resolvedMtool,
		Args:             rest,
	}
	if err := build.Execute(ctx, opts, bxexec.RealRunner{}); err != nil {
		fmt.Fprintf(os.Stderr, "buildmux: %v\n", err)
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return ee.ExitCode()
		}
		if errors.Is(err, context.Canceled) {
			return 130
		}
		return 2
	}
	return 0
}

// parseLeadingFlags consumes only flags that fs knows about from the start of
// argv. It stops at the first unknown flag, the first positional arg, or the
// "--" sentinel — whichever comes first — and returns the remaining tokens
// verbatim for forwarding to buildctl.
//
// All known flags in buildmux take a string value (no booleans), so we only
// need to handle "--name value" and "--name=value" forms.
func parseLeadingFlags(fs *flag.FlagSet, argv []string) ([]string, error) {
	i := 0
	for i < len(argv) {
		tok := argv[i]
		if tok == "--" {
			return argv[i+1:], nil
		}
		if !strings.HasPrefix(tok, "-") {
			return argv[i:], nil
		}
		name := strings.TrimLeft(tok, "-")
		if eq := strings.IndexByte(name, '='); eq >= 0 {
			val := name[eq+1:]
			name = name[:eq]
			f := fs.Lookup(name)
			if f == nil {
				return argv[i:], nil
			}
			if err := f.Value.Set(val); err != nil {
				return nil, fmt.Errorf("--%s: %w", name, err)
			}
			i++
			continue
		}
		f := fs.Lookup(name)
		if f == nil {
			return argv[i:], nil
		}
		if i+1 >= len(argv) {
			return nil, fmt.Errorf("--%s: value required", name)
		}
		if err := f.Value.Set(argv[i+1]); err != nil {
			return nil, fmt.Errorf("--%s: %w", name, err)
		}
		i += 2
	}
	return nil, nil
}

func defaultConfigPath() string {
	if v := os.Getenv("BUILDMUX_CONFIG"); v != "" {
		return v
	}
	return "buildmux.yaml"
}

func resolveBinary(override, name string) (string, error) {
	if override != "" {
		return override, nil
	}
	p, err := exec.LookPath(name)
	if err != nil {
		return "", fmt.Errorf("%s not found in PATH (use --%s to specify): %w", name, name, err)
	}
	return p, nil
}

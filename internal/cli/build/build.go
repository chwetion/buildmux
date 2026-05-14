// Package build implements the buildmux "build" subcommand orchestration.
package build

import (
	"context"
	"fmt"
	"os"

	"github.com/chwetion/buildmux/internal/cli/args"
	"github.com/chwetion/buildmux/internal/config"
	"github.com/chwetion/buildmux/internal/dispatch"
	bxexec "github.com/chwetion/buildmux/internal/exec"
	"github.com/chwetion/buildmux/internal/output"
)

type Options struct {
	ConfigPath       string
	BuildctlPath     string
	ManifestToolPath string
	Args             []string // trailing args (after `buildmux build`)
	Stdout           *os.File // optional; defaults to os.Stdout
	Stderr           *os.File // optional; defaults to os.Stderr
}

func Execute(ctx context.Context, opts Options, runner bxexec.Runner) error {
	stdout := opts.Stdout
	if stdout == nil {
		stdout = os.Stdout
	}
	stderr := opts.Stderr
	if stderr == nil {
		stderr = os.Stderr
	}

	cfg, err := config.Load(opts.ConfigPath)
	if err != nil {
		return err
	}
	parsed, err := args.Parse(opts.Args)
	if err != nil {
		return err
	}

	// Validate all requested platforms have a config entry.
	var missing []string
	for _, p := range parsed.Platforms {
		if _, ok := cfg.Platforms[p]; !ok {
			missing = append(missing, p)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("platforms not in config: %v", missing)
	}

	// Build per-platform jobs.
	type result struct {
		platform string
		image    string
	}
	results := make([]result, len(parsed.Platforms))
	jobs := make([]dispatch.Job, 0, len(parsed.Platforms))
	for i, p := range parsed.Platforms {
		i, p := i, p
		platformCfg := cfg.Platforms[p]
		renderedName, err := platformCfg.RenderTag(p, parsed.Output.Name)
		if err != nil {
			return fmt.Errorf("render tag for %q: %w", p, err)
		}
		newOut := output.Spec{
			Type:  parsed.Output.Type,
			Name:  renderedName,
			Push:  parsed.Output.Push,
			Other: parsed.Output.Other,
		}
		results[i] = result{platform: p, image: renderedName}

		inv := bxexec.BuildctlInvocation(opts.BuildctlPath, p, &platformCfg, parsed.Rest, newOut.String())
		prefix := makePrefix(p, i, isTTY(stdout))
		jobs = append(jobs, dispatch.Job{
			Name: p,
			Fn: func(ctx context.Context) error {
				ow := bxexec.NewPrefixWriter(stdout, prefix)
				ew := bxexec.NewPrefixWriter(stderr, prefix)
				defer ow.Close()
				defer ew.Close()
				return runner.Run(ctx, inv, ow, ew)
			},
		})
	}

	if err := dispatch.Run(ctx, jobs); err != nil {
		return err
	}

	// All builds succeeded; generate manifest spec and invoke manifest-tool.
	entries := make([]bxexec.ManifestEntry, 0, len(results))
	for _, r := range results {
		entries = append(entries, bxexec.ManifestEntry{Platform: r.platform, Image: r.image})
	}
	specYAML, err := bxexec.BuildManifestSpec(parsed.Output.Name, entries)
	if err != nil {
		return fmt.Errorf("build manifest spec: %w", err)
	}
	tmp, err := os.CreateTemp("", "buildmux-manifest-*.yaml")
	if err != nil {
		return fmt.Errorf("create temp manifest spec: %w", err)
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.WriteString(specYAML); err != nil {
		tmp.Close()
		return fmt.Errorf("write temp manifest spec: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp manifest spec: %w", err)
	}

	mInv := bxexec.ManifestToolInvocation(opts.ManifestToolPath, cfg.Manifest, tmp.Name())
	manifestPrefix := makePrefix("manifest-tool", len(parsed.Platforms), isTTY(stdout))
	mOut := bxexec.NewPrefixWriter(stdout, manifestPrefix)
	mErr := bxexec.NewPrefixWriter(stderr, manifestPrefix)
	defer mOut.Close()
	defer mErr.Close()
	return runner.Run(ctx, mInv, mOut, mErr)
}

// makePrefix renders the line-prefix for a given platform/index.
// When the destination is a TTY, the bracketed name is wrapped in an ANSI
// color escape sequence; otherwise it's plain text.
func makePrefix(name string, index int, color bool) string {
	if !color {
		return fmt.Sprintf("[%s] ", name)
	}
	colors := []string{"\x1b[36m", "\x1b[33m", "\x1b[35m", "\x1b[32m", "\x1b[34m", "\x1b[31m"}
	c := colors[index%len(colors)]
	return fmt.Sprintf("%s[%s]\x1b[0m ", c, name)
}

// isTTY reports whether f is a character device (terminal).
func isTTY(f *os.File) bool {
	if f == nil {
		return false
	}
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

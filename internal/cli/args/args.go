// Package args parses the trailing portion of `buildmux build [...]` into
// a structured form, hoisting --opt platform and --output out so buildmux
// can route per-platform and rewrite outputs.
package args

import (
	"errors"
	"fmt"
	"strings"

	"github.com/chwetion/buildmux/internal/output"
)

type ParsedArgs struct {
	Platforms []string
	Output    output.Spec
	Rest      []string
}

func Parse(in []string) (ParsedArgs, error) {
	var p ParsedArgs
	p.Rest = make([]string, 0, len(in))
	gotOutput := false

	i := 0
	for i < len(in) {
		tok := in[i]
		// --opt forms
		if tok == "--opt" && i+1 < len(in) {
			val := in[i+1]
			if isPlatformVal(val) {
				p.Platforms = append(p.Platforms, splitPlatformList(stripPlatformPrefix(val))...)
				i += 2
				continue
			}
			p.Rest = append(p.Rest, tok, val)
			i += 2
			continue
		}
		if strings.HasPrefix(tok, "--opt=") {
			val := strings.TrimPrefix(tok, "--opt=")
			if isPlatformVal(val) {
				p.Platforms = append(p.Platforms, splitPlatformList(stripPlatformPrefix(val))...)
				i++
				continue
			}
			p.Rest = append(p.Rest, tok)
			i++
			continue
		}
		// --output forms
		if tok == "--output" && i+1 < len(in) {
			if gotOutput {
				return p, errors.New("duplicate --output")
			}
			spec, err := output.Parse(in[i+1])
			if err != nil {
				return p, fmt.Errorf("--output: %w", err)
			}
			p.Output = spec
			gotOutput = true
			i += 2
			continue
		}
		if strings.HasPrefix(tok, "--output=") {
			if gotOutput {
				return p, errors.New("duplicate --output")
			}
			spec, err := output.Parse(strings.TrimPrefix(tok, "--output="))
			if err != nil {
				return p, fmt.Errorf("--output: %w", err)
			}
			p.Output = spec
			gotOutput = true
			i++
			continue
		}
		p.Rest = append(p.Rest, tok)
		i++
	}

	if len(p.Platforms) == 0 {
		return p, errors.New("--opt platform=<...> is required (one or more)")
	}
	if !gotOutput {
		return p, errors.New("--output type=image,name=<ref>,push=true is required")
	}
	if p.Output.Type != "image" {
		return p, fmt.Errorf("--output: only type=image is supported, got type=%q", p.Output.Type)
	}
	if !p.Output.Push {
		return p, errors.New("--output: only push=true is supported")
	}
	if p.Output.Name == "" {
		return p, errors.New("--output: name=<ref> is required")
	}
	return p, nil
}

func isPlatformVal(v string) bool {
	return strings.HasPrefix(v, "platform=")
}

func stripPlatformPrefix(v string) string {
	return strings.TrimPrefix(v, "platform=")
}

func splitPlatformList(v string) []string {
	out := strings.Split(v, ",")
	for i := range out {
		out[i] = strings.TrimSpace(out[i])
	}
	return out
}

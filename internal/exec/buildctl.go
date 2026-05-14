package exec

import "github.com/chwetion/buildmux/internal/config"

// BuildctlInvocation produces the buildctl invocation for a single platform.
// `rest` is the original buildctl args with --opt platform and --output already removed.
// `finalOutput` is the rewritten --output string (with per-arch name).
func BuildctlInvocation(binary, platform string, p *config.Platform, rest []string, finalOutput string) Invocation {
	args := make([]string, 0, 8+len(rest))
	args = append(args, "--addr", p.Endpoint)
	if p.TLS != nil {
		args = append(args,
			"--tlscacert", p.TLS.CACert,
			"--tlscert", p.TLS.Cert,
			"--tlskey", p.TLS.Key,
		)
		if p.TLS.ServerName != "" {
			args = append(args, "--tlsservername", p.TLS.ServerName)
		}
	}
	args = append(args, "build")
	args = append(args, rest...)
	args = append(args, "--opt", "platform="+platform)
	args = append(args, "--output", finalOutput)
	return Invocation{Binary: binary, Args: args}
}

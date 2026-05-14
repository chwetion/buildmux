package exec

import (
	"context"
	"io"
	"os/exec"
)

type Invocation struct {
	Binary string
	Args   []string
}

type Runner interface {
	Run(ctx context.Context, inv Invocation, stdout, stderr io.Writer) error
}

type RealRunner struct{}

func (RealRunner) Run(ctx context.Context, inv Invocation, stdout, stderr io.Writer) error {
	cmd := exec.CommandContext(ctx, inv.Binary, inv.Args...)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd.Run()
}

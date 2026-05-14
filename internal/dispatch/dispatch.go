// Package dispatch runs a set of jobs in parallel with fail-fast cancellation.
package dispatch

import (
	"context"

	"golang.org/x/sync/errgroup"
)

type Job struct {
	Name string
	Fn   func(ctx context.Context) error
}

func Run(parent context.Context, jobs []Job) error {
	g, ctx := errgroup.WithContext(parent)
	for _, j := range jobs {
		j := j
		g.Go(func() error {
			return j.Fn(ctx)
		})
	}
	return g.Wait()
}

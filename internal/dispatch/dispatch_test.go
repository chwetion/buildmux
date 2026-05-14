package dispatch

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestRunAllSucceed(t *testing.T) {
	var n atomic.Int32
	jobs := []Job{
		{Name: "a", Fn: func(ctx context.Context) error { n.Add(1); return nil }},
		{Name: "b", Fn: func(ctx context.Context) error { n.Add(1); return nil }},
	}
	if err := Run(context.Background(), jobs); err != nil {
		t.Fatalf("err=%v", err)
	}
	if n.Load() != 2 {
		t.Errorf("ran %d jobs, want 2", n.Load())
	}
}

func TestRunFirstFailureCancelsOthers(t *testing.T) {
	boom := errors.New("boom")
	var cancelled atomic.Bool
	jobs := []Job{
		{Name: "fail", Fn: func(ctx context.Context) error {
			return boom
		}},
		{Name: "slow", Fn: func(ctx context.Context) error {
			select {
			case <-ctx.Done():
				cancelled.Store(true)
				return ctx.Err()
			case <-time.After(2 * time.Second):
				return nil
			}
		}},
	}
	err := Run(context.Background(), jobs)
	if !errors.Is(err, boom) {
		t.Errorf("err=%v, want boom", err)
	}
	if !cancelled.Load() {
		t.Errorf("slow job was not cancelled")
	}
}

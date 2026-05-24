package sshtail

import (
	"context"
	"io"
)

// Executor is the seam that hides the SSH transport behind a thin
// io.ReadCloser contract. Production wiring (cmd/webox) implements
// this against `ssh.Pool`; tests inject a stub that emits canned
// bytes.
//
// Open MUST honour ctx.Done() in two phases:
//
//  1. While dialing/acquiring an SSH session — return ctx.Err().
//  2. While the returned reader is in use — close the reader so the
//     consumer's blocking Read unblocks with io.EOF or an error.
//
// The streamer always calls Close on the returned reader once it is
// done (whether through context cancel, reconnect, or natural EOF).
type Executor interface {
	Open(ctx context.Context, command string) (io.ReadCloser, error)
}

// ExecutorFunc adapts a plain function to the [Executor] interface.
type ExecutorFunc func(ctx context.Context, command string) (io.ReadCloser, error)

// Open satisfies [Executor].
func (f ExecutorFunc) Open(ctx context.Context, command string) (io.ReadCloser, error) {
	return f(ctx, command)
}

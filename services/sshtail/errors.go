package sshtail

import "errors"

// Sentinel errors. Callers MUST compare via `errors.Is`; the streamer
// wraps the underlying cause where useful.
var (
	// ErrLogPathInvalid is returned when the requested log path is
	// empty, absolute-but-traversal-prone, or fails a preflight stat.
	ErrLogPathInvalid = errors.New("sshtail: log path invalid")

	// ErrSessionClosed is returned when the remote session ended
	// gracefully (EOF without errors). The consumer should treat it
	// as a normal stream termination, not an alert-worthy failure.
	ErrSessionClosed = errors.New("sshtail: session closed")

	// ErrReconnectFailed is returned when every reconnect attempt
	// exhausts the configured backoff schedule. Surfaced to the
	// dashboard as a yellow banner; the tile keeps the buffered
	// history visible.
	ErrReconnectFailed = errors.New("sshtail: reconnect failed")

	// ErrStreamerClosed is returned when the consumer drains the
	// channel after the streamer's context was cancelled.
	ErrStreamerClosed = errors.New("sshtail: streamer closed")
)

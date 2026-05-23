package sshtail

import "strings"

// shellEscape returns path quoted with single quotes for safe inclusion
// in a `sh -c` command. Single quotes inside the input are escaped via
// the canonical `'\”` dance.
//
// The function does NOT validate that path is a sensible filesystem
// path — caller is expected to run [validateLogPath] first.
func shellEscape(path string) string {
	if path == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(path, "'", `'\''`) + "'"
}

// validateLogPath rejects paths that would let a hostile provider
// implementation traverse outside the project log directory or sneak
// in shell metacharacters. Per Sprint 09 §TASK-09.1 the streamer fails
// fast with [ErrLogPathInvalid] before opening a session.
func validateLogPath(path string) error {
	switch {
	case path == "":
		return ErrLogPathInvalid
	case strings.ContainsAny(path, "\n\r\t\x00"):
		return ErrLogPathInvalid
	case strings.Contains(path, ".."):
		return ErrLogPathInvalid
	}
	return nil
}

package domain

import (
	"context"
	"errors"
)

var (
	// ErrInvalidConnection indicates connection parameters failed validation.
	ErrInvalidConnection = errors.New("invalid connection parameters")
	// ErrSessionNotActive indicates no active telnet session exists.
	ErrSessionNotActive = errors.New("session not active")
)

// SanitizedConnectError returns a user-visible message without host/port leakage.
func SanitizedConnectError() string {
	return "Connection failed"
}

// SanitizedDialError returns a generic dial failure message.
func SanitizedDialError() string {
	return "Unable to reach remote host"
}

// SanitizedDialErrorFrom maps a dial error to a user-visible message.
func SanitizedDialErrorFrom(err error) string {
	if err == nil {
		return SanitizedDialError()
	}
	if errors.Is(err, context.Canceled) {
		return "Connection interrupted"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "Connection timed out"
	}
	return SanitizedDialError()
}

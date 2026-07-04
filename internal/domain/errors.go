package domain

import "errors"

var (
	// ErrInvalidConnection indicates connection parameters failed validation.
	ErrInvalidConnection = errors.New("invalid connection parameters")
	// ErrSessionNotActive indicates no active telnet session exists.
	ErrSessionNotActive = errors.New("session not active")
)

// SanitizedConnectError returns a user-visible message without host/port leakage.
func SanitizedConnectError() string {
	return "connection failed"
}

// SanitizedDialError returns a generic dial failure message.
func SanitizedDialError() string {
	return "unable to reach remote host"
}

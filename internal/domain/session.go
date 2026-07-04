package domain

// SessionID identifies an active terminal session.
type SessionID string

// SessionState is reported to the host via session.updateState.
type SessionState string

const (
	SessionConnecting SessionState = "connecting"
	SessionReady      SessionState = "ready"
	SessionError      SessionState = "error"
)

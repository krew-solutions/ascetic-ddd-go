package bus

import "errors"

var (
	// ErrUnknownScheme: the URI's scheme has no registered adapter, or the
	// URI has no scheme at all.
	ErrUnknownScheme = errors.New("bus: no adapter registered for scheme")
	// ErrAlreadyRegistered: the scheme is already bound to an adapter.
	ErrAlreadyRegistered = errors.New("bus: scheme is already registered")
	// ErrAlreadyInGroup: a second consumer joined the same (uri, group) on
	// an adapter that allows one, a configuration bug in a monolith.
	ErrAlreadyInGroup = errors.New("bus: a consumer already exists in this group")
	// ErrNoChannel: the URI names no channel after its scheme.
	ErrNoChannel = errors.New("bus: the uri names no channel")
	// ErrClosed: the transport has been closed.
	ErrClosed = errors.New("bus: the transport has been closed")
)

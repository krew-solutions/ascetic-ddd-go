package session

import "errors"

var ErrConcurrency = errors.New(
	"aggregate is modified concurrently",
)

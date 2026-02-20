package identitymap

import "errors"

var (
	ErrKeyNotFound    = errors.New("identitymap: key not found")
	ErrObjectNotFound = errors.New("identitymap: object not found")
)

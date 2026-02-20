package identitymap

type nonexistentObject struct{}

var nonexistent = &nonexistentObject{}

type isolationStrategy interface {
	add(key any, value any)
	addAbsent(key any)
	get(key any) (any, error)
	has(key any) bool
}

// readUncommittedStrategy — identity map is disabled.
type readUncommittedStrategy struct{}

func (s *readUncommittedStrategy) add(any, any)    {}
func (s *readUncommittedStrategy) addAbsent(any)   {}
func (s *readUncommittedStrategy) has(any) bool     { return false }
func (s *readUncommittedStrategy) get(any) (any, error) {
	return nil, ErrKeyNotFound
}

// readCommittedStrategy — identity map is disabled.
type readCommittedStrategy struct{}

func (s *readCommittedStrategy) add(any, any)    {}
func (s *readCommittedStrategy) addAbsent(any)   {}
func (s *readCommittedStrategy) has(any) bool     { return false }
func (s *readCommittedStrategy) get(any) (any, error) {
	return nil, ErrKeyNotFound
}

// repeatableReadsStrategy — caches existent objects only.
type repeatableReadsStrategy struct {
	cache *lruCache
}

func (s *repeatableReadsStrategy) add(key any, value any) {
	s.cache.add(key, value)
}

func (s *repeatableReadsStrategy) addAbsent(any) {}

func (s *repeatableReadsStrategy) get(key any) (any, error) {
	value, ok := s.cache.get(key)
	if !ok {
		return nil, ErrKeyNotFound
	}
	if _, isNonexistent := value.(*nonexistentObject); isNonexistent {
		return nil, ErrKeyNotFound
	}
	return value, nil
}

func (s *repeatableReadsStrategy) has(key any) bool {
	value, ok := s.cache.get(key)
	if !ok {
		return false
	}
	_, isNonexistent := value.(*nonexistentObject)
	return !isNonexistent
}

// serializableStrategy — caches both existent and nonexistent objects.
type serializableStrategy struct {
	cache *lruCache
}

func (s *serializableStrategy) add(key any, value any) {
	s.cache.add(key, value)
}

func (s *serializableStrategy) addAbsent(key any) {
	s.cache.add(key, nonexistent)
}

func (s *serializableStrategy) get(key any) (any, error) {
	value, ok := s.cache.get(key)
	if !ok {
		return nil, ErrKeyNotFound
	}
	if _, isNonexistent := value.(*nonexistentObject); isNonexistent {
		return nil, ErrObjectNotFound
	}
	return value, nil
}

func (s *serializableStrategy) has(key any) bool {
	_, ok := s.cache.get(key)
	return ok
}

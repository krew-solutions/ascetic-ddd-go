package identitymap

// IsolationLevel controls how the identity map caches objects.
type IsolationLevel int

const (
	ReadUncommitted IsolationLevel = iota // Identity map is disabled
	ReadCommitted                         // Identity map is disabled
	RepeatableReads                       // Prevents repeated queries for existent objects only
	Serializable                          // Prevents repeated queries for both existent and nonexistent objects
)

// IdentityMap tracks entity instances to ensure each entity is loaded only once per session.
type IdentityMap struct {
	cache    *lruCache
	strategy isolationStrategy
}

func New(cacheSize int, level IsolationLevel) *IdentityMap {
	cache := newLruCache(cacheSize)
	m := &IdentityMap{cache: cache}
	m.SetIsolationLevel(level)
	return m
}

func (m *IdentityMap) SetIsolationLevel(level IsolationLevel) {
	switch level {
	case ReadUncommitted:
		m.strategy = &readUncommittedStrategy{}
	case ReadCommitted:
		m.strategy = &readCommittedStrategy{}
	case RepeatableReads:
		m.strategy = &repeatableReadsStrategy{cache: m.cache}
	case Serializable:
		m.strategy = &serializableStrategy{cache: m.cache}
	default:
		m.strategy = &serializableStrategy{cache: m.cache}
	}
}

func (m *IdentityMap) SetSize(size int) {
	m.cache.setSize(size)
}

func (m *IdentityMap) Clear() {
	m.cache.clear()
}

// Add stores a found object in the identity map.
func Add[V any](m *IdentityMap, key IdentityKey[V], value V) {
	m.strategy.add(key, value)
}

// AddAbsent records that the key was queried but does not exist.
// Only effective with Serializable isolation level.
func AddAbsent[V any](m *IdentityMap, key IdentityKey[V]) {
	m.strategy.addAbsent(key)
}

// Get retrieves a previously stored object by its key.
func Get[V any](m *IdentityMap, key IdentityKey[V]) (V, error) {
	result, err := m.strategy.get(key)
	if err != nil {
		var zero V
		return zero, err
	}
	return result.(V), nil
}

// Has checks whether the key exists in the identity map.
func Has[V any](m *IdentityMap, key IdentityKey[V]) bool {
	return m.strategy.has(key)
}

// Remove removes an object from the identity map.
func Remove[V any](m *IdentityMap, key IdentityKey[V]) {
	m.cache.remove(key)
}

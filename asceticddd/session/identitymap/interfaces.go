package identitymap

// IdentityKey is a marker interface that associates a key with its value type.
// Embed IdentityKeyBase[V] into your key structs to implement this interface.
type IdentityKey[V any] interface {
	IsIdentityKey(*V)
}

// IdentityKeyBase is an embeddable struct that implements IdentityKey[V].
type IdentityKeyBase[V any] struct{}

func (IdentityKeyBase[V]) IsIdentityKey(*V) {}

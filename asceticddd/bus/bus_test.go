package bus_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/bus"
	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/bus/adapters/inmemory"
)

// Tests for the bus itself: the scheme registry and its error paths. The
// messaging behaviour lives in the adapters and is exercised by their tests.

func asText(message bus.Message) (string, error) {
	return string(message.Payload()), nil
}

func TestAnUnknownSchemeIsRefused(t *testing.T) {
	b := bus.New()
	require.NoError(t, b.Register("in-memory", inmemory.New()))

	_, err := bus.NewConsumer(b, "kafka://x", "g", asText)
	assert.ErrorIs(t, err, bus.ErrUnknownScheme)
	assert.ErrorContains(t, err, `"kafka"`)
}

func TestASchemeIsRegisteredOnce(t *testing.T) {
	b := bus.New()
	require.NoError(t, b.Register("in-memory", inmemory.New()))

	err := b.Register("in-memory", inmemory.New())
	assert.ErrorIs(t, err, bus.ErrAlreadyRegistered)
}

func TestAUriWithoutASchemeIsRefused(t *testing.T) {
	b := bus.New()
	require.NoError(t, b.Register("in-memory", inmemory.New()))

	_, err := bus.NewConsumer(b, "no-scheme-here", "g", asText)
	assert.ErrorIs(t, err, bus.ErrUnknownScheme)
	assert.ErrorContains(t, err, `"no-scheme-here"`)
}

// The registry is per bus, not global.
func TestBusesDoNotShareTheirRegistry(t *testing.T) {
	first := bus.New()
	require.NoError(t, first.Register("in-memory", inmemory.New()))
	second := bus.New()

	_, err := bus.NewConsumer(second, "in-memory://x", "g", asText)
	assert.True(t, errors.Is(err, bus.ErrUnknownScheme))
}

func TestUriParts(t *testing.T) {
	scheme, err := bus.SchemeOf("kafka://orders/order-7")
	require.NoError(t, err)
	assert.Equal(t, "kafka", scheme)

	channel, err := bus.ChannelOf("kafka://orders/order-7")
	require.NoError(t, err)
	assert.Equal(t, "orders", channel)

	key, ok := bus.KeyOf("kafka://orders/order-7")
	assert.True(t, ok)
	assert.Equal(t, "order-7", key)

	_, ok = bus.KeyOf("kafka://orders")
	assert.False(t, ok)
	_, ok = bus.KeyOf("kafka://orders/")
	assert.False(t, ok, "an empty key is no key")

	assert.Equal(t, "kafka://orders", bus.WithoutKey("kafka://orders/order-7"))
	assert.Equal(t, "kafka://orders", bus.WithoutKey("kafka://orders"))
	assert.Equal(t, "no-scheme", bus.WithoutKey("no-scheme"))

	_, err = bus.ChannelOf("kafka://")
	assert.ErrorIs(t, err, bus.ErrNoChannel)
	_, err = bus.ChannelOf("kafka:orders")
	assert.ErrorIs(t, err, bus.ErrUnknownScheme)
}

func TestMessageIsAValue(t *testing.T) {
	original := bus.NewMessage([]byte("x")).WithHeader("a", []byte("1"))
	extended := original.WithHeader("b", []byte("2")).WithKey([]byte("k"))

	assert.Len(t, original.Headers(), 1, "WithHeader leaves the receiver untouched")
	assert.Nil(t, original.Key())
	assert.Len(t, extended.Headers(), 2)
	assert.Equal(t, []byte("k"), extended.Key())

	value, ok := extended.Header("b")
	assert.True(t, ok)
	assert.Equal(t, []byte("2"), value)
	_, ok = extended.Header("c")
	assert.False(t, ok)

	clone := extended.Clone()
	clone.Payload()[0] = 'y'
	assert.Equal(t, []byte("x"), extended.Payload(), "a clone shares no bytes with the original")
}

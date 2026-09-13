package bus

import (
	"fmt"
	"strings"
)

// The shape of a URI on the bus is scheme://channel[/key]. The scheme picks
// the adapter, the channel is the topic or queue, and what follows the
// channel is a key: messages with one key stay in order where the transport
// partitions. "kafka://orders/order-7" is topic "orders", key "order-7";
// "in-memory://orders" is the same channel with no key.

// SchemeOf returns what comes before the first ':'.
func SchemeOf(uri string) (string, error) {
	scheme, _, ok := strings.Cut(uri, ":")
	if !ok {
		return "", fmt.Errorf("%w: %q", ErrUnknownScheme, uri)
	}
	return scheme, nil
}

// ChannelOf returns what follows "://", up to the first '/'.
func ChannelOf(uri string) (string, error) {
	_, rest, ok := strings.Cut(uri, "://")
	if !ok {
		return "", fmt.Errorf("%w: %q", ErrUnknownScheme, uri)
	}
	channel, _, _ := strings.Cut(rest, "/")
	if channel == "" {
		return "", fmt.Errorf("%w: %q", ErrNoChannel, uri)
	}
	return channel, nil
}

// KeyOf returns what follows the channel, if anything.
func KeyOf(uri string) (string, bool) {
	_, rest, ok := strings.Cut(uri, "://")
	if !ok {
		return "", false
	}
	_, key, ok := strings.Cut(rest, "/")
	if !ok || key == "" {
		return "", false
	}
	return key, true
}

// WithoutKey returns the URI without its key: the channel a consumer
// subscribes to.
func WithoutKey(uri string) string {
	scheme, rest, ok := strings.Cut(uri, "://")
	if !ok {
		return uri
	}
	channel, _, ok := strings.Cut(rest, "/")
	if !ok {
		return uri
	}
	return scheme + "://" + channel
}

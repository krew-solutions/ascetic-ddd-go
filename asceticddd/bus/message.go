package bus

import "bytes"

// Header is one named value carried beside the payload. Headers are flat,
// name to bytes, as Kafka's are; anything nested travels as JSON text.
type Header struct {
	Name  string
	Value []byte
}

// Message is what crosses the wire: an opaque payload, an optional key and
// headers. The bus never looks inside the payload: producers encode their
// values into it and consumers decode it back, each with a function of its
// own, so two consumers of one topic may read the same bytes as different
// types.
//
// The key is for transports that partition: messages with equal keys stay
// in order relative to each other. A Message is a value; the With methods
// return a modified copy and leave the receiver untouched.
type Message struct {
	key     []byte
	payload []byte
	headers []Header
}

// NewMessage is a message carrying payload and no key.
func NewMessage(payload []byte) Message {
	return Message{payload: payload}
}

// WithKey is the same message, keyed.
func (m Message) WithKey(key []byte) Message {
	m.key = key
	return m
}

// WithHeader is the same message with one more header.
func (m Message) WithHeader(name string, value []byte) Message {
	headers := make([]Header, len(m.headers), len(m.headers)+1)
	copy(headers, m.headers)
	m.headers = append(headers, Header{Name: name, Value: value})
	return m
}

// Header returns the first header of that name.
func (m Message) Header(name string) ([]byte, bool) {
	for _, h := range m.headers {
		if h.Name == name {
			return h.Value, true
		}
	}
	return nil, false
}

// Headers returns every header, in order.
func (m Message) Headers() []Header {
	return append([]Header(nil), m.headers...)
}

// Key returns the key, or nil when the message has none.
func (m Message) Key() []byte {
	return m.key
}

// Payload returns the payload.
func (m Message) Payload() []byte {
	return m.payload
}

// Clone copies the message deeply, so that a handler cannot alter what
// another handler of the same message receives.
func (m Message) Clone() Message {
	c := Message{key: bytes.Clone(m.key), payload: bytes.Clone(m.payload)}
	if m.headers != nil {
		c.headers = make([]Header, len(m.headers))
		for i, h := range m.headers {
			c.headers[i] = Header{Name: h.Name, Value: bytes.Clone(h.Value)}
		}
	}
	return c
}

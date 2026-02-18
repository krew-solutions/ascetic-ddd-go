package signals

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCompositeSignal_AttachPropagatesToAllDelegates(t *testing.T) {
	s1 := NewSignal[sampleEvent]()
	s2 := NewSignal[sampleEvent]()
	composite := NewCompositeSignal[sampleEvent](s1, s2)
	callCount := 0
	composite.Attach(func(e sampleEvent) { callCount++ }, "obs")
	s1.Notify(sampleEvent{1})
	s2.Notify(sampleEvent{1})
	assert.Equal(t, 2, callCount)
}

func TestCompositeSignal_DetachPropagatesToAllDelegates(t *testing.T) {
	s1 := NewSignal[sampleEvent]()
	s2 := NewSignal[sampleEvent]()
	composite := NewCompositeSignal[sampleEvent](s1, s2)
	called := false
	observer := Observer[sampleEvent](func(e sampleEvent) { called = true })
	composite.Attach(observer, "obs")
	composite.Detach(observer, "obs")
	s1.Notify(sampleEvent{1})
	s2.Notify(sampleEvent{1})
	assert.False(t, called)
}

func TestCompositeSignal_NotifyPropagatesToAllDelegates(t *testing.T) {
	s1 := NewSignal[sampleEvent]()
	s2 := NewSignal[sampleEvent]()
	composite := NewCompositeSignal[sampleEvent](s1, s2)
	callCount := 0
	composite.Attach(func(e sampleEvent) { callCount++ }, "obs")
	composite.Notify(sampleEvent{1})
	assert.Equal(t, 2, callCount)
}

func TestCompositeSignal_DisposableDetachesFromAllDelegates(t *testing.T) {
	s1 := NewSignal[sampleEvent]()
	s2 := NewSignal[sampleEvent]()
	composite := NewCompositeSignal[sampleEvent](s1, s2)
	called := false
	d := composite.Attach(func(e sampleEvent) { called = true }, "obs")
	d.Dispose()
	s1.Notify(sampleEvent{1})
	s2.Notify(sampleEvent{1})
	assert.False(t, called)
}

func TestCompositeSignal_NotifyNoDelegates(t *testing.T) {
	composite := NewCompositeSignal[sampleEvent]()
	composite.Notify(sampleEvent{1}) // should not panic
}

func TestCompositeSignal_NotifySingleDelegate(t *testing.T) {
	s := NewSignal[sampleEvent]()
	composite := NewCompositeSignal[sampleEvent](s)
	callCount := 0
	composite.Attach(func(e sampleEvent) { callCount++ }, "obs")
	composite.Notify(sampleEvent{1})
	assert.Equal(t, 1, callCount)
}

func TestCompositeSignal_AttachWithoutID(t *testing.T) {
	s1 := NewSignal[sampleEvent]()
	s2 := NewSignal[sampleEvent]()
	composite := NewCompositeSignal[sampleEvent](s1, s2)
	callCount := 0
	observer := Observer[sampleEvent](func(e sampleEvent) { callCount++ })
	composite.Attach(observer)
	composite.Notify(sampleEvent{1})
	assert.Equal(t, 2, callCount)
}

func TestCompositeSignal_DetachWithoutID(t *testing.T) {
	s1 := NewSignal[sampleEvent]()
	s2 := NewSignal[sampleEvent]()
	composite := NewCompositeSignal[sampleEvent](s1, s2)
	called := false
	observer := Observer[sampleEvent](func(e sampleEvent) { called = true })
	composite.Attach(observer)
	composite.Detach(observer)
	s1.Notify(sampleEvent{1})
	s2.Notify(sampleEvent{1})
	assert.False(t, called)
}

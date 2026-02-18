package signals

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

type sampleEvent struct {
	payload int
}

func TestSignal_AttachAndNotify(t *testing.T) {
	s := NewSignal[sampleEvent]()
	var called sampleEvent
	s.Attach(func(e sampleEvent) { called = e }, "obs")
	s.Notify(sampleEvent{1})
	assert.Equal(t, sampleEvent{1}, called)
}

func TestSignal_NotifyMultipleObservers(t *testing.T) {
	s := NewSignal[sampleEvent]()
	var calls []int
	s.Attach(func(e sampleEvent) { calls = append(calls, 1) }, "obs1")
	s.Attach(func(e sampleEvent) { calls = append(calls, 2) }, "obs2")
	s.Notify(sampleEvent{1})
	assert.Equal(t, []int{1, 2}, calls)
}

func TestSignal_NotifyPreservesOrder(t *testing.T) {
	s := NewSignal[sampleEvent]()
	var order []int
	s.Attach(func(e sampleEvent) { order = append(order, 1) }, "obs1")
	s.Attach(func(e sampleEvent) { order = append(order, 2) }, "obs2")
	s.Notify(sampleEvent{1})
	assert.Equal(t, []int{1, 2}, order)
}

func TestSignal_Detach(t *testing.T) {
	s := NewSignal[sampleEvent]()
	called := false
	observer := Observer[sampleEvent](func(e sampleEvent) { called = true })
	s.Attach(observer, "obs")
	s.Detach(observer, "obs")
	s.Notify(sampleEvent{1})
	assert.False(t, called)
}

func TestSignal_DetachNonexistentIsSilent(t *testing.T) {
	s := NewSignal[sampleEvent]()
	observer := Observer[sampleEvent](func(e sampleEvent) {})
	s.Detach(observer, "nonexistent") // should not panic
}

func TestSignal_AttachDuplicateIsIdempotent(t *testing.T) {
	s := NewSignal[sampleEvent]()
	callCount := 0
	observer := Observer[sampleEvent](func(e sampleEvent) { callCount++ })
	s.Attach(observer, "obs")
	s.Attach(observer, "obs")
	s.Notify(sampleEvent{1})
	assert.Equal(t, 1, callCount)
}

func TestSignal_AttachDuplicateObserverIDKeepsFirst(t *testing.T) {
	s := NewSignal[sampleEvent]()
	var which int
	s.Attach(func(e sampleEvent) { which = 1 }, "same")
	s.Attach(func(e sampleEvent) { which = 2 }, "same")
	s.Notify(sampleEvent{1})
	assert.Equal(t, 1, which)
}

func TestSignal_NotifyNoObservers(t *testing.T) {
	s := NewSignal[sampleEvent]()
	s.Notify(sampleEvent{1}) // should not panic
}

func TestSignal_DisposableDetaches(t *testing.T) {
	s := NewSignal[sampleEvent]()
	called := false
	d := s.Attach(func(e sampleEvent) { called = true }, "obs")
	d.Dispose()
	s.Notify(sampleEvent{1})
	assert.False(t, called)
}

func TestSignal_AttachWithoutID(t *testing.T) {
	s := NewSignal[sampleEvent]()
	var called sampleEvent
	observer := Observer[sampleEvent](func(e sampleEvent) { called = e })
	s.Attach(observer)
	s.Notify(sampleEvent{42})
	assert.Equal(t, sampleEvent{42}, called)
}

func TestSignal_DetachWithoutID(t *testing.T) {
	s := NewSignal[sampleEvent]()
	called := false
	observer := Observer[sampleEvent](func(e sampleEvent) { called = true })
	s.Attach(observer)
	s.Detach(observer)
	s.Notify(sampleEvent{1})
	assert.False(t, called)
}

func TestSignal_AttachDuplicateWithoutIDIsIdempotent(t *testing.T) {
	s := NewSignal[sampleEvent]()
	callCount := 0
	observer := Observer[sampleEvent](func(e sampleEvent) { callCount++ })
	s.Attach(observer)
	s.Attach(observer)
	s.Notify(sampleEvent{1})
	assert.Equal(t, 1, callCount)
}

func TestSignal_DisposableDetachesWithoutID(t *testing.T) {
	s := NewSignal[sampleEvent]()
	called := false
	d := s.Attach(func(e sampleEvent) { called = true })
	d.Dispose()
	s.Notify(sampleEvent{1})
	assert.False(t, called)
}

func TestMakeIdForFunction(t *testing.T) {
	observer := Observer[sampleEvent](func(e sampleEvent) {})
	result := makeId(observer)
	assert.Equal(t, reflect.ValueOf(observer).Pointer(), result)
}

func TestSignal_DifferentObserversWithoutIDAreSeparate(t *testing.T) {
	s := NewSignal[sampleEvent]()
	var calls []int
	obs1 := Observer[sampleEvent](func(e sampleEvent) { calls = append(calls, 1) })
	obs2 := Observer[sampleEvent](func(e sampleEvent) { calls = append(calls, 2) })
	s.Attach(obs1)
	s.Attach(obs2)
	s.Notify(sampleEvent{1})
	assert.Equal(t, []int{1, 2}, calls)
}

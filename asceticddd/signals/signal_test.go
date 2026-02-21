package signals

import (
	"errors"
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
	s.Attach(func(e sampleEvent) error { called = e; return nil }, "obs")
	s.Notify(sampleEvent{1})
	assert.Equal(t, sampleEvent{1}, called)
}

func TestSignal_NotifyMultipleObservers(t *testing.T) {
	s := NewSignal[sampleEvent]()
	var calls []int
	s.Attach(func(e sampleEvent) error { calls = append(calls, 1); return nil }, "obs1")
	s.Attach(func(e sampleEvent) error { calls = append(calls, 2); return nil }, "obs2")
	s.Notify(sampleEvent{1})
	assert.Equal(t, []int{1, 2}, calls)
}

func TestSignal_NotifyPreservesOrder(t *testing.T) {
	s := NewSignal[sampleEvent]()
	var order []int
	s.Attach(func(e sampleEvent) error { order = append(order, 1); return nil }, "obs1")
	s.Attach(func(e sampleEvent) error { order = append(order, 2); return nil }, "obs2")
	s.Notify(sampleEvent{1})
	assert.Equal(t, []int{1, 2}, order)
}

func TestSignal_Detach(t *testing.T) {
	s := NewSignal[sampleEvent]()
	called := false
	observer := Observer[sampleEvent](func(e sampleEvent) error { called = true; return nil })
	s.Attach(observer, "obs")
	s.Detach(observer, "obs")
	s.Notify(sampleEvent{1})
	assert.False(t, called)
}

func TestSignal_DetachNonexistentIsSilent(t *testing.T) {
	s := NewSignal[sampleEvent]()
	observer := Observer[sampleEvent](func(e sampleEvent) error { return nil })
	s.Detach(observer, "nonexistent") // should not panic
}

func TestSignal_AttachDuplicateIsIdempotent(t *testing.T) {
	s := NewSignal[sampleEvent]()
	callCount := 0
	observer := Observer[sampleEvent](func(e sampleEvent) error { callCount++; return nil })
	s.Attach(observer, "obs")
	s.Attach(observer, "obs")
	s.Notify(sampleEvent{1})
	assert.Equal(t, 1, callCount)
}

func TestSignal_AttachDuplicateObserverIDKeepsFirst(t *testing.T) {
	s := NewSignal[sampleEvent]()
	var which int
	s.Attach(func(e sampleEvent) error { which = 1; return nil }, "same")
	s.Attach(func(e sampleEvent) error { which = 2; return nil }, "same")
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
	d := s.Attach(func(e sampleEvent) error { called = true; return nil }, "obs")
	d.Dispose()
	s.Notify(sampleEvent{1})
	assert.False(t, called)
}

func TestSignal_AttachWithoutID(t *testing.T) {
	s := NewSignal[sampleEvent]()
	var called sampleEvent
	observer := Observer[sampleEvent](func(e sampleEvent) error { called = e; return nil })
	s.Attach(observer)
	s.Notify(sampleEvent{42})
	assert.Equal(t, sampleEvent{42}, called)
}

func TestSignal_DetachWithoutID(t *testing.T) {
	s := NewSignal[sampleEvent]()
	called := false
	observer := Observer[sampleEvent](func(e sampleEvent) error { called = true; return nil })
	s.Attach(observer)
	s.Detach(observer)
	s.Notify(sampleEvent{1})
	assert.False(t, called)
}

func TestSignal_AttachDuplicateWithoutIDIsIdempotent(t *testing.T) {
	s := NewSignal[sampleEvent]()
	callCount := 0
	observer := Observer[sampleEvent](func(e sampleEvent) error { callCount++; return nil })
	s.Attach(observer)
	s.Attach(observer)
	s.Notify(sampleEvent{1})
	assert.Equal(t, 1, callCount)
}

func TestSignal_DisposableDetachesWithoutID(t *testing.T) {
	s := NewSignal[sampleEvent]()
	called := false
	d := s.Attach(func(e sampleEvent) error { called = true; return nil })
	d.Dispose()
	s.Notify(sampleEvent{1})
	assert.False(t, called)
}

func TestMakeIdForFunction(t *testing.T) {
	observer := Observer[sampleEvent](func(e sampleEvent) error { return nil })
	result := makeId(observer)
	assert.Equal(t, reflect.ValueOf(observer).Pointer(), result)
}

func TestSignal_DifferentObserversWithoutIDAreSeparate(t *testing.T) {
	s := NewSignal[sampleEvent]()
	var calls []int
	obs1 := Observer[sampleEvent](func(e sampleEvent) error { calls = append(calls, 1); return nil })
	obs2 := Observer[sampleEvent](func(e sampleEvent) error { calls = append(calls, 2); return nil })
	s.Attach(obs1)
	s.Attach(obs2)
	s.Notify(sampleEvent{1})
	assert.Equal(t, []int{1, 2}, calls)
}

func TestSignal_NotifyReturnsObserverError(t *testing.T) {
	s := NewSignal[sampleEvent]()
	expectedErr := errors.New("observer failed")
	s.Attach(func(e sampleEvent) error { return expectedErr }, "obs")
	err := s.Notify(sampleEvent{1})
	assert.Equal(t, expectedErr, err)
}

func TestSignal_NotifyStopsOnFirstError(t *testing.T) {
	s := NewSignal[sampleEvent]()
	var calls []int
	s.Attach(func(e sampleEvent) error { calls = append(calls, 1); return errors.New("fail") }, "obs1")
	s.Attach(func(e sampleEvent) error { calls = append(calls, 2); return nil }, "obs2")
	s.Notify(sampleEvent{1})
	assert.Equal(t, []int{1}, calls)
}

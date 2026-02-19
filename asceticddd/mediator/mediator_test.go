package mediator

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

type session struct{}

type sampleEvent struct {
	payload int
}

type anotherEvent struct {
	payload string
}

type sampleCommand struct {
	RequestBase[int]
	payload int
}

type anotherCommand struct {
	RequestBase[string]
	payload string
}

// --- Publish ---

func TestPublishCallsSubscriber(t *testing.T) {
	m := NewMediator[*session]()
	s := &session{}
	var calledSession any
	var calledEvent any

	handler := func(sess *session, event sampleEvent) error {
		calledSession = sess
		calledEvent = event
		return nil
	}

	Subscribe(m, handler)
	event := sampleEvent{payload: 2}
	err := Publish(m, s, event)

	assert.NoError(t, err)
	assert.Same(t, s, calledSession)
	assert.Equal(t, event, calledEvent)
}

func TestPublishCallsMultipleSubscribers(t *testing.T) {
	m := NewMediator[*session]()
	s := &session{}
	var called1, called2 bool

	handler1 := func(sess *session, event sampleEvent) error {
		called1 = true
		return nil
	}
	handler2 := func(sess *session, event sampleEvent) error {
		called2 = true
		return nil
	}

	Subscribe(m, handler1)
	Subscribe(m, handler2)
	err := Publish(m, s, sampleEvent{payload: 3})

	assert.NoError(t, err)
	assert.True(t, called1)
	assert.True(t, called2)
}

func TestPublishDoesNotCallSubscriberOfOtherEventType(t *testing.T) {
	m := NewMediator[*session]()
	s := &session{}
	var called bool

	handler := func(sess *session, event anotherEvent) error {
		called = true
		return nil
	}

	Subscribe(m, handler)
	err := Publish(m, s, sampleEvent{payload: 1})

	assert.NoError(t, err)
	assert.False(t, called)
}

func TestPublishWithNoSubscribers(t *testing.T) {
	m := NewMediator[*session]()
	s := &session{}

	err := Publish(m, s, sampleEvent{payload: 1})

	assert.NoError(t, err)
}

// --- Unsubscribe ---

func TestUnsubscribeRemovesHandler(t *testing.T) {
	m := NewMediator[*session]()
	s := &session{}
	var called1, called2 bool

	handler := func(sess *session, event sampleEvent) error {
		called1 = true
		return nil
	}
	handler2 := func(sess *session, event sampleEvent) error {
		called2 = true
		return nil
	}

	Subscribe(m, handler)
	Subscribe(m, handler2)
	Unsubscribe(m, handler)

	err := Publish(m, s, sampleEvent{payload: 2})

	assert.NoError(t, err)
	assert.False(t, called1)
	assert.True(t, called2)
}

func TestUnsubscribeNonexistentHandlerIsNoop(t *testing.T) {
	m := NewMediator[*session]()

	handler := func(sess *session, event sampleEvent) error {
		return nil
	}

	Unsubscribe(m, handler)
}

func TestDisposeUnsubscribes(t *testing.T) {
	m := NewMediator[*session]()
	s := &session{}
	var called1, called2 bool

	handler := func(sess *session, event sampleEvent) error {
		called1 = true
		return nil
	}
	handler2 := func(sess *session, event sampleEvent) error {
		called2 = true
		return nil
	}

	d := Subscribe(m, handler)
	Subscribe(m, handler2)
	d.Dispose()

	err := Publish(m, s, sampleEvent{payload: 2})

	assert.NoError(t, err)
	assert.False(t, called1)
	assert.True(t, called2)
}

// --- Send ---

func TestSendCallsHandlerAndReturnsResult(t *testing.T) {
	m := NewMediator[*session]()
	s := &session{}
	var calledSession any
	var calledRequest any

	handler := func(sess *session, req sampleCommand) (int, error) {
		calledSession = sess
		calledRequest = req
		return 5, nil
	}

	Register(m, handler)
	command := sampleCommand{payload: 2}
	result, err := Send(m, s, command)

	assert.NoError(t, err)
	assert.Equal(t, 5, result) // result is int, not any
	assert.Same(t, s, calledSession)
	assert.Equal(t, command, calledRequest)
}

func TestSendReturnsZeroWhenNoHandler(t *testing.T) {
	m := NewMediator[*session]()
	s := &session{}

	result, err := Send(m, s, sampleCommand{payload: 1})

	assert.NoError(t, err)
	assert.Equal(t, 0, result) // zero value of int
}

func TestSendDispatchesByRequestType(t *testing.T) {
	m := NewMediator[*session]()
	s := &session{}

	handler1 := func(sess *session, req sampleCommand) (int, error) {
		return 10, nil
	}
	handler2 := func(sess *session, req anotherCommand) (string, error) {
		return "hello", nil
	}

	Register(m, handler1)
	Register(m, handler2)

	result1, err1 := Send(m, s, sampleCommand{payload: 1})
	result2, err2 := Send(m, s, anotherCommand{payload: "x"})

	assert.NoError(t, err1)
	assert.NoError(t, err2)
	assert.Equal(t, 10, result1)    // int
	assert.Equal(t, "hello", result2) // string
}

// --- Unregister ---

func TestUnregisterRemovesHandler(t *testing.T) {
	m := NewMediator[*session]()
	s := &session{}
	var called bool

	handler := func(sess *session, req sampleCommand) (int, error) {
		called = true
		return 0, nil
	}

	Register(m, handler)
	err := Unregister[*session, sampleCommand](m)

	assert.NoError(t, err)

	result, err := Send(m, s, sampleCommand{payload: 2})

	assert.NoError(t, err)
	assert.Equal(t, 0, result)
	assert.False(t, called)
}

func TestUnregisterNonexistentReturnsError(t *testing.T) {
	m := NewMediator[*session]()

	err := Unregister[*session, sampleCommand](m)

	assert.Error(t, err)
	assert.True(t, errors.Is(err, ErrHandlerNotRegistered))
}

func TestDisposeUnregisters(t *testing.T) {
	m := NewMediator[*session]()
	s := &session{}
	var called bool

	handler := func(sess *session, req sampleCommand) (int, error) {
		called = true
		return 0, nil
	}

	d := Register(m, handler)
	d.Dispose()

	result, err := Send(m, s, sampleCommand{payload: 2})

	assert.NoError(t, err)
	assert.Equal(t, 0, result)
	assert.False(t, called)
}

// --- Pipeline ---

func TestTypedPipelineWrapsHandler(t *testing.T) {
	m := NewMediator[*session]()
	s := &session{}
	var callLog []string

	handler := func(sess *session, req sampleCommand) (int, error) {
		callLog = append(callLog, "handler")
		return req.payload * 2, nil
	}

	pipeline := func(sess *session, req sampleCommand, next RequestHandler[*session, sampleCommand, int]) (int, error) {
		callLog = append(callLog, "before")
		result, err := next(sess, req)
		callLog = append(callLog, "after")
		return result, err
	}

	Register(m, handler)
	AddPipeline(m, pipeline)

	result, err := Send(m, s, sampleCommand{payload: 3})

	assert.NoError(t, err)
	assert.Equal(t, 6, result) // int, not any
	assert.Equal(t, []string{"before", "handler", "after"}, callLog)
}

func TestBroadcastPipelineWrapsAllRequestTypes(t *testing.T) {
	m := NewMediator[*session]()
	s := &session{}
	var callLog []string

	handler1 := func(sess *session, req sampleCommand) (int, error) {
		callLog = append(callLog, "handler1")
		return req.payload, nil
	}
	handler2 := func(sess *session, req anotherCommand) (string, error) {
		callLog = append(callLog, "handler2")
		return req.payload, nil
	}

	pipeline := func(sess *session, request any, next func(*session, any) (any, error)) (any, error) {
		callLog = append(callLog, "broadcast")
		return next(sess, request)
	}

	Register(m, handler1)
	Register(m, handler2)
	AddBroadcastPipeline(m,pipeline)

	Send(m, s, sampleCommand{payload: 1})
	Send(m, s, anotherCommand{payload: "x"})

	assert.Equal(t, []string{
		"broadcast", "handler1",
		"broadcast", "handler2",
	}, callLog)
}

func TestMultiplePipelinesExecuteInOrder(t *testing.T) {
	m := NewMediator[*session]()
	s := &session{}
	var callLog []string

	handler := func(sess *session, req sampleCommand) (int, error) {
		callLog = append(callLog, "handler")
		return 0, nil
	}

	pipelineA := func(sess *session, req sampleCommand, next RequestHandler[*session, sampleCommand, int]) (int, error) {
		callLog = append(callLog, "A-before")
		result, err := next(sess, req)
		callLog = append(callLog, "A-after")
		return result, err
	}
	pipelineB := func(sess *session, req sampleCommand, next RequestHandler[*session, sampleCommand, int]) (int, error) {
		callLog = append(callLog, "B-before")
		result, err := next(sess, req)
		callLog = append(callLog, "B-after")
		return result, err
	}

	Register(m, handler)
	AddPipeline(m, pipelineA)
	AddPipeline(m, pipelineB)

	Send(m, s, sampleCommand{payload: 1})

	// First added pipeline is outermost
	assert.Equal(t, []string{
		"A-before", "B-before", "handler", "B-after", "A-after",
	}, callLog)
}

func TestBroadcastPipelineRunsBeforeTypedPipeline(t *testing.T) {
	m := NewMediator[*session]()
	s := &session{}
	var callLog []string

	handler := func(sess *session, req sampleCommand) (int, error) {
		callLog = append(callLog, "handler")
		return 0, nil
	}

	broadcast := func(sess *session, request any, next func(*session, any) (any, error)) (any, error) {
		callLog = append(callLog, "broadcast")
		return next(sess, request)
	}
	typed := func(sess *session, req sampleCommand, next RequestHandler[*session, sampleCommand, int]) (int, error) {
		callLog = append(callLog, "typed")
		return next(sess, req)
	}

	Register(m, handler)
	AddBroadcastPipeline(m,broadcast)
	AddPipeline(m, typed)

	Send(m, s, sampleCommand{payload: 1})

	assert.Equal(t, []string{"broadcast", "typed", "handler"}, callLog)
}

func TestPipelineCanModifyResult(t *testing.T) {
	m := NewMediator[*session]()
	s := &session{}

	handler := func(sess *session, req sampleCommand) (int, error) {
		return req.payload, nil
	}

	pipeline := func(sess *session, req sampleCommand, next RequestHandler[*session, sampleCommand, int]) (int, error) {
		result, err := next(sess, req)
		if err != nil {
			return 0, err
		}
		return result + 100, nil
	}

	Register(m, handler)
	AddPipeline(m, pipeline)

	result, err := Send(m, s, sampleCommand{payload: 5})

	assert.NoError(t, err)
	assert.Equal(t, 105, result) // int, not any
}

func TestPipelineNotAppliedToOtherRequestType(t *testing.T) {
	m := NewMediator[*session]()
	s := &session{}
	var callLog []string

	handler := func(sess *session, req anotherCommand) (string, error) {
		return req.payload, nil
	}

	pipeline := func(sess *session, req sampleCommand, next RequestHandler[*session, sampleCommand, int]) (int, error) {
		callLog = append(callLog, "pipeline")
		return next(sess, req)
	}

	Register(m, func(sess *session, req sampleCommand) (int, error) { return 0, nil })
	Register(m, handler)
	AddPipeline(m, pipeline)

	Send(m, s, anotherCommand{payload: "x"})

	assert.Empty(t, callLog)
}

func TestSendWithoutHandlerSkipsPipelines(t *testing.T) {
	m := NewMediator[*session]()
	s := &session{}
	var callLog []string

	pipeline := func(sess *session, req sampleCommand, next RequestHandler[*session, sampleCommand, int]) (int, error) {
		callLog = append(callLog, "pipeline")
		return next(sess, req)
	}

	AddPipeline(m, pipeline)

	result, err := Send(m, s, sampleCommand{payload: 1})

	assert.NoError(t, err)
	assert.Equal(t, 0, result)
	assert.Empty(t, callLog)
}

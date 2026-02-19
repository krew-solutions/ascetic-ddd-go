package mediator

import (
	"fmt"
	"reflect"

	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/disposable"
)

var ErrHandlerNotRegistered = fmt.Errorf("mediator: handler not registered")

type subscriberEntry[S any] struct {
	key     uintptr
	handler func(S, any) error
}

type internalPipeline[S any] = func(S, any, func(S, any) (any, error)) (any, error)

func NewMediator[S any]() *MediatorImp[S] {
	return &MediatorImp[S]{
		subscribers: make(map[reflect.Type][]subscriberEntry[S]),
		handlers:    make(map[reflect.Type]func(S, any) (any, error)),
		pipelines:   make(map[reflect.Type][]internalPipeline[S]),
	}
}

type MediatorImp[S any] struct {
	subscribers        map[reflect.Type][]subscriberEntry[S]
	handlers           map[reflect.Type]func(S, any) (any, error)
	broadcastPipelines []internalPipeline[S]
	pipelines          map[reflect.Type][]internalPipeline[S]
}

func (m *MediatorImp[S]) send(session S, request any) (any, error) {
	handler, ok := m.handlers[reflect.TypeOf(request)]
	if !ok {
		return nil, nil
	}
	return m.executePipelines(session, request, handler)
}

func (m *MediatorImp[S]) executePipelines(session S, request any, handler func(S, any) (any, error)) (any, error) {
	currentHandler := handler
	typedPipelines := m.pipelines[reflect.TypeOf(request)]
	allPipelines := make([]internalPipeline[S], 0, len(m.broadcastPipelines)+len(typedPipelines))
	allPipelines = append(allPipelines, m.broadcastPipelines...)
	allPipelines = append(allPipelines, typedPipelines...)

	for i := len(allPipelines) - 1; i >= 0; i-- {
		currentHandler = wrapPipeline(allPipelines[i], currentHandler)
	}

	return currentHandler(session, request)
}

func wrapPipeline[S any](pipeline internalPipeline[S], next func(S, any) (any, error)) func(S, any) (any, error) {
	return func(session S, request any) (any, error) {
		return pipeline(session, request, next)
	}
}

// --- Typed free functions ---

// Send sends a request that implements Request[Res] and returns a typed result.
func Send[S, Res any](m *MediatorImp[S], session S, request Request[Res]) (Res, error) {
	result, err := m.send(session, request)
	if err != nil {
		var zero Res
		return zero, err
	}
	if result == nil {
		var zero Res
		return zero, nil
	}
	return result.(Res), nil
}

// Publish publishes an event to all subscribers of the event's type.
func Publish[S, E any](m *MediatorImp[S], session S, event E) error {
	for _, entry := range m.subscribers[reflect.TypeOf(event)] {
		if err := entry.handler(session, event); err != nil {
			return err
		}
	}
	return nil
}

// Register registers a typed request handler for requests of type Req.
func Register[S, Req, Res any](m *MediatorImp[S], handler RequestHandler[S, Req, Res]) disposable.Disposable {
	reqType := reflect.TypeFor[Req]()
	m.handlers[reqType] = func(session S, request any) (any, error) {
		return handler(session, request.(Req))
	}
	return disposable.NewDisposable(func() {
		Unregister[S, Req](m)
	})
}

// Unregister removes a previously registered request handler for requests of type Req.
func Unregister[S, Req any](m *MediatorImp[S]) error {
	reqType := reflect.TypeFor[Req]()
	if _, ok := m.handlers[reqType]; !ok {
		return fmt.Errorf("%w for type %v", ErrHandlerNotRegistered, reqType)
	}
	delete(m.handlers, reqType)
	return nil
}

// Subscribe subscribes a typed event handler for events of type E.
func Subscribe[S, E any](m *MediatorImp[S], handler EventHandler[S, E]) disposable.Disposable {
	eventType := reflect.TypeFor[E]()
	key := reflect.ValueOf(handler).Pointer()
	m.subscribers[eventType] = append(m.subscribers[eventType], subscriberEntry[S]{
		key: key,
		handler: func(session S, event any) error {
			return handler(session, event.(E))
		},
	})
	return disposable.NewDisposable(func() {
		Unsubscribe(m, handler)
	})
}

// Unsubscribe removes a previously subscribed typed event handler.
func Unsubscribe[S, E any](m *MediatorImp[S], handler EventHandler[S, E]) {
	eventType := reflect.TypeFor[E]()
	key := reflect.ValueOf(handler).Pointer()
	entries := m.subscribers[eventType]
	for i, e := range entries {
		if e.key == key {
			m.subscribers[eventType] = append(entries[:i], entries[i+1:]...)
			return
		}
	}
}

// AddPipeline adds a typed pipeline for requests of type Req.
func AddPipeline[S, Req, Res any](m *MediatorImp[S], pipeline PipelineHandler[S, Req, Res]) {
	reqType := reflect.TypeFor[Req]()
	m.pipelines[reqType] = append(m.pipelines[reqType], func(session S, request any, next func(S, any) (any, error)) (any, error) {
		typedNext := func(s S, r Req) (Res, error) {
			result, err := next(s, r)
			if err != nil {
				var zero Res
				return zero, err
			}
			if result == nil {
				var zero Res
				return zero, nil
			}
			return result.(Res), nil
		}
		return pipeline(session, request.(Req), typedNext)
	})
}

// AddBroadcastPipeline adds a pipeline that wraps all request types.
func AddBroadcastPipeline[S any](m *MediatorImp[S], pipeline BroadcastPipelineHandler[S]) {
	m.broadcastPipelines = append(m.broadcastPipelines, pipeline)
}

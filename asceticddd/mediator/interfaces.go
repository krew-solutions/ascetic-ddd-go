package mediator

// Request is a marker interface that associates a request type with its result type.
// Embed RequestBase[Res] into your request structs to implement this interface.
type Request[Res any] interface {
	IsRequest(*Res)
}

// RequestBase is an embeddable struct that implements Request[Res].
type RequestBase[Res any] struct{}

func (RequestBase[Res]) IsRequest(*Res) {}

// RequestHandler handles a request of type Req and returns a result of type Res.
type RequestHandler[S, Req, Res any] = func(session S, request Req) (Res, error)

// EventHandler handles an event of type E.
type EventHandler[S, E any] = func(session S, event E) error

// PipelineHandler wraps request execution with cross-cutting concerns.
type PipelineHandler[S, Req, Res any] = func(session S, request Req, next RequestHandler[S, Req, Res]) (Res, error)

// BroadcastPipelineHandler wraps all request types with cross-cutting concerns.
type BroadcastPipelineHandler[S any] = func(session S, request any, next func(S, any) (any, error)) (any, error)

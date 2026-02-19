# mediator

A fully type-safe mediator (MediatR) implementation for Go.

Unlike other Go mediator libraries that rely on `any` everywhere, this implementation uses generic type aliases (Go 1.24+) and a phantom type marker (`RequestBase[Res]`) to achieve full type inference — no type assertions, no `reflect.TypeOf`, no `any` in user code.

## Usage

### Define requests

Embed `RequestBase[Res]` to associate a request type with its result type:

```go
type CreateUser struct {
    mediator.RequestBase[UserId]
    Name string
}

type GetUser struct {
    mediator.RequestBase[User]
    Id UserId
}
```

### Define events

Events are plain structs, no marker needed:

```go
type UserCreated struct {
    Id   UserId
    Name string
}
```

### Create a mediator

```go
m := mediator.NewMediator[Session]()
```

The type parameter `S` accepts any type: an interface, a pointer, or a value.

### Register handlers

All type parameters are inferred from the handler signature:

```go
mediator.Register(m, func(sess Session, req CreateUser) (UserId, error) {
    id := NewUserId()
    // ...
    return id, nil
})

mediator.Register(m, func(sess Session, req GetUser) (User, error) {
    // ...
    return user, nil
})
```

### Send requests

The result type is inferred from `RequestBase[Res]`:

```go
userId, err := mediator.Send(m, session, CreateUser{Name: "Alice"})
// userId is UserId, not any

user, err := mediator.Send(m, session, GetUser{Id: userId})
// user is User, not any
```

### Subscribe to events

```go
handler := func(sess Session, event UserCreated) error {
    log.Printf("user created: %s", event.Name)
    return nil
}

mediator.Subscribe(m, handler)
```

### Publish events

```go
err := mediator.Publish(m, session, UserCreated{Id: id, Name: "Alice"})
```

### Unregister / Unsubscribe

`Register` and `Subscribe` return a `Disposable` for convenient cleanup:

```go
d := mediator.Register(m, handler)
d.Dispose() // removes the handler
```

Or explicitly:

```go
mediator.Unregister[Session, CreateUser](m)
mediator.Unsubscribe(m, handler)
```

## Pipelines

Pipelines are middleware for request processing. They wrap handler execution and can inspect/modify the request and result.

### Typed pipeline

Applied to a specific request type:

```go
mediator.AddPipeline(m, func(
    sess Session,
    req CreateUser,
    next mediator.RequestHandler[Session, CreateUser, UserId],
) (UserId, error) {
    log.Println("before")
    result, err := next(sess, req)
    log.Println("after")
    return result, err
})
```

### Broadcast pipeline

Applied to all request types (uses `any` — unavoidable since the request type is not known at compile time):

```go
mediator.AddBroadcastPipeline(m, func(
    sess Session,
    request any,
    next func(Session, any) (any, error),
) (any, error) {
    start := time.Now()
    result, err := next(sess, request)
    log.Printf("request took %v", time.Since(start))
    return result, err
})
```

### Pipeline ordering

- Broadcast pipelines run before typed pipelines.
- Within each group, the first added pipeline is the outermost (executes first on the way in, last on the way out).

## API reference

| Function | Description |
|---|---|
| `Send(m, session, request)` | Send a request, return typed result |
| `Publish(m, session, event)` | Publish an event to all subscribers |
| `Register(m, handler)` | Register a request handler |
| `Unregister[S, Req](m)` | Remove a request handler |
| `Subscribe(m, handler)` | Subscribe an event handler |
| `Unsubscribe(m, handler)` | Remove an event handler |
| `AddPipeline(m, pipeline)` | Add a typed pipeline |
| `AddBroadcastPipeline(m, pipeline)` | Add a broadcast pipeline |

All type parameters are inferred from arguments, except `Unregister` which requires explicit type parameters.

## Requirements

Go 1.24+ (generic type aliases).

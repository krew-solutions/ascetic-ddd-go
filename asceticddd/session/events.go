package session

import (
	"strconv"
	"time"
)

type SessionScopeStartedEvent struct {
	Session Session
}

type SessionScopeEndedEvent struct {
	Session Session
}

type QueryStartedEvent struct {
	Query   string
	Params  []any
	Sender  any
	Session DbSession
}

type QueryEndedEvent struct {
	Query        string
	Params       []any
	Sender       any
	Session      DbSession
	ResponseTime time.Duration
}

type RequestViewModel struct {
	TimeStart    time.Time
	Label        string
	Status       *int
	ResponseTime *time.Duration
}

func (r RequestViewModel) String() string {
	if r.Status != nil {
		return r.Label + "." + strconv.Itoa(*r.Status)
	}
	return r.Label
}

type RequestStartedEvent struct {
	Session     Session
	Sender      any
	RequestView *RequestViewModel
}

type RequestEndedEvent struct {
	Session     Session
	Sender      any
	RequestView *RequestViewModel
}

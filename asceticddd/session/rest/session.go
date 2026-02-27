package rest

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/session"
	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/session/identitymap"
	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/signals"
)

const defaultCacheSize = 100

var hostname string

func init() {
	hostname, _ = os.Hostname()
}

func ExtractHttpClient(s session.Session) *http.Client {
	return s.(session.RestSession).HttpClient()
}

type observableTransport struct {
	base    http.RoundTripper
	session *Session
}

func (t *observableTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	label := fmt.Sprintf(
		"ascetic-ddd.%s.%s.%s.%s",
		hostname, req.Method, req.URL.Host, req.URL.Path,
	)
	requestView := &session.RequestViewModel{
		TimeStart: time.Now(),
		Label:     label,
	}

	if err := t.session.onRequestStarted.Notify(session.RequestStartedEvent{
		Session:     t.session,
		Sender:      t.session,
		RequestView: requestView,
	}); err != nil {
		return nil, err
	}

	resp, err := t.base.RoundTrip(req)

	responseTime := time.Since(requestView.TimeStart)
	requestView.ResponseTime = &responseTime
	if resp != nil {
		status := resp.StatusCode
		requestView.Status = &status
	}

	if endErr := t.session.onRequestEnded.Notify(session.RequestEndedEvent{
		Session:     t.session,
		Sender:      t.session,
		RequestView: requestView,
	}); endErr != nil && err == nil {
		err = endErr
	}

	return resp, err
}

type Session struct {
	ctx              context.Context
	httpClient       *http.Client
	parent           session.Session
	identityMap      *identitymap.IdentityMap
	onStarted        signals.Signal[session.SessionScopeStartedEvent]
	onEnded          signals.Signal[session.SessionScopeEndedEvent]
	onRequestStarted signals.Signal[session.RequestStartedEvent]
	onRequestEnded   signals.Signal[session.RequestEndedEvent]
}

func NewSession(ctx context.Context, transport http.RoundTripper) *Session {
	s := &Session{
		ctx:              ctx,
		parent:           nil,
		identityMap:      identitymap.New(defaultCacheSize, identitymap.ReadUncommitted),
		onStarted:        signals.NewSignal[session.SessionScopeStartedEvent](),
		onEnded:          signals.NewSignal[session.SessionScopeEndedEvent](),
		onRequestStarted: signals.NewSignal[session.RequestStartedEvent](),
		onRequestEnded:   signals.NewSignal[session.RequestEndedEvent](),
	}
	s.httpClient = &http.Client{
		Transport: &observableTransport{base: transport, session: s},
	}
	return s
}

func (s *Session) Context() context.Context {
	return s.ctx
}

func (s *Session) HttpClient() *http.Client {
	return s.httpClient
}

func (s *Session) IdentityMap() *identitymap.IdentityMap {
	return s.identityMap
}

func (s *Session) OnAtomicStarted() signals.Signal[session.SessionScopeStartedEvent] {
	return s.onStarted
}

func (s *Session) OnAtomicEnded() signals.Signal[session.SessionScopeEndedEvent] {
	return s.onEnded
}

func (s *Session) OnRequestStarted() signals.Signal[session.RequestStartedEvent] {
	return s.onRequestStarted
}

func (s *Session) OnRequestEnded() signals.Signal[session.RequestEndedEvent] {
	return s.onRequestEnded
}

func (s *Session) Atomic(callback session.SessionCallback) error {
	atomicSession := s.makeAtomicSession()

	if err := s.onStarted.Notify(session.SessionScopeStartedEvent{Session: atomicSession}); err != nil {
		return err
	}

	err := callback(atomicSession)

	if s.parent == nil {
		atomicSession.identityMap.Clear()
	}

	if endedErr := s.onEnded.Notify(session.SessionScopeEndedEvent{Session: atomicSession}); err == nil {
		err = endedErr
	}

	return err
}

func (s *Session) makeAtomicSession() *AtomicSession {
	return NewAtomicSession(s.ctx, s.httpClient.Transport.(*observableTransport).base, s)
}

type AtomicSession struct {
	Session
}

func NewAtomicSession(ctx context.Context, transport http.RoundTripper, parent *Session) *AtomicSession {
	s := &AtomicSession{}
	s.ctx = ctx
	s.parent = parent
	s.identityMap = identitymap.New(defaultCacheSize, identitymap.Serializable)
	s.onStarted = signals.NewSignal[session.SessionScopeStartedEvent]()
	s.onEnded = signals.NewSignal[session.SessionScopeEndedEvent]()
	s.onRequestStarted = signals.NewSignal[session.RequestStartedEvent]()
	s.onRequestEnded = signals.NewSignal[session.RequestEndedEvent]()
	s.httpClient = &http.Client{
		Transport: &observableTransport{base: transport, session: &s.Session},
	}
	return s
}

func (s *AtomicSession) Atomic(callback session.SessionCallback) error {
	atomicSession := s.makeNestedAtomicSession()

	if err := s.onStarted.Notify(session.SessionScopeStartedEvent{Session: atomicSession}); err != nil {
		return err
	}

	err := callback(atomicSession)

	if endedErr := s.onEnded.Notify(session.SessionScopeEndedEvent{Session: atomicSession}); err == nil {
		err = endedErr
	}

	return err
}

func (s *AtomicSession) makeNestedAtomicSession() *AtomicSession {
	nested := &AtomicSession{}
	nested.ctx = s.ctx
	nested.parent = s
	nested.identityMap = s.identityMap
	nested.onStarted = signals.NewSignal[session.SessionScopeStartedEvent]()
	nested.onEnded = signals.NewSignal[session.SessionScopeEndedEvent]()
	nested.onRequestStarted = signals.NewSignal[session.RequestStartedEvent]()
	nested.onRequestEnded = signals.NewSignal[session.RequestEndedEvent]()
	nested.httpClient = &http.Client{
		Transport: &observableTransport{
			base:    s.httpClient.Transport.(*observableTransport).base,
			session: &nested.Session,
		},
	}
	return nested
}

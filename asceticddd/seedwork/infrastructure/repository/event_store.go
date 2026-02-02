package repository

import (
	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/seedwork/domain/aggregate"
	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/seedwork/domain/uuid"
	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/session"
)

type EventSourcedQueryEvaluator interface {
	session.QueryEvaluator
	SetStreamType(string)
}

type EventQueryFactory func(aggregate.PersistentDomainEvent) EventSourcedQueryEvaluator

func NewEventStore(streamType string, eventQuery EventQueryFactory) *EventStore {
	return &EventStore{
		streamType: streamType,
		eventQuery: eventQuery,
	}
}

type EventStore struct {
	streamType string
	eventQuery EventQueryFactory
}

func (r EventStore) NewStreamId(
	tenantId uint,
	streamId string,
) (StreamId, error) {
	return NewStreamId(tenantId, r.streamType, streamId)
}

func (r *EventStore) Save(
	s session.DbSession,
	agg aggregate.DomainEventAccessor[aggregate.PersistentDomainEvent],
	eventMeta aggregate.EventMeta,
) error {
	pendingEvents := agg.PendingDomainEvents()
	agg.ClearPendingDomainEvents()

	for _, iEvent := range pendingEvents {
		eventMeta = eventMeta.Spawn(uuid.NewUuid())
		iEvent.SetEventMeta(eventMeta)
		q := r.eventQuery(iEvent)
		q.SetStreamType(r.streamType)
		_, err := q.Evaluate(s)
		if err != nil {
			return err
		}
	}
	return nil
}

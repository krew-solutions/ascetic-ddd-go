package repository

import (
	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/seedwork/domain/aggregate"
	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/seedwork/domain/uuid"
	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/session"
)

type CodecFactory func(session.Session, StreamId) (Codec, error)

type EventSourcedQueryEvaluator interface {
	Evaluate(CodecFactory, session.Session) (session.Result, error)
	SetStreamType(string)
}

type EventQueryFactory func(aggregate.PersistentDomainEvent) EventSourcedQueryEvaluator

func NewEventStore(dekStore DekStore, streamType string, eventQuery EventQueryFactory) *EventStore {
	return &EventStore{
		dekStore:   dekStore,
		streamType: streamType,
		eventQuery: eventQuery,
	}
}

type EventStore struct {
	dekStore   DekStore
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
	s session.Session,
	agg aggregate.DomainEventAccessor[aggregate.PersistentDomainEvent],
	eventMeta aggregate.EventMeta,
) error {
	pendingEvents := agg.PendingDomainEvents()
	agg.ClearPendingDomainEvents()
	codecFactory := r.makeCodecFactory()

	for _, iEvent := range pendingEvents {
		eventMeta = eventMeta.Spawn(uuid.NewUuid())
		iEvent.SetEventMeta(eventMeta)
		q := r.eventQuery(iEvent)
		q.SetStreamType(r.streamType)
		_, err := q.Evaluate(codecFactory, s)
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *EventStore) makeCodecFactory() CodecFactory {
	cache := make(map[StreamId]Codec)
	return func(s session.Session, streamId StreamId) (Codec, error) {
		if codec, ok := cache[streamId]; ok {
			return codec, nil
		}
		cipher, err := r.dekStore.GetOrCreate(s, streamId)
		if err != nil {
			return nil, err
		}
		codec := NewEncryptionCodec(cipher, NewZlibCodec(NewJsonCodec()))
		cache[streamId] = codec
		return codec, nil
	}
}

func (r *EventStore) makeReadCodecFactory() CodecFactory {
	cache := make(map[StreamId]Codec)
	return func(s session.Session, streamId StreamId) (Codec, error) {
		if codec, ok := cache[streamId]; ok {
			return codec, nil
		}
		cipher, err := r.dekStore.GetAll(s, streamId)
		if err != nil {
			return nil, err
		}
		codec := NewEncryptionCodec(cipher, NewZlibCodec(NewJsonCodec()))
		cache[streamId] = codec
		return codec, nil
	}
}

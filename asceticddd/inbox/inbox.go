package inbox

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/session"
)

type PgInbox struct {
	sessionPool           session.SessionPool
	table                 string
	sequence              string
	partitionKeyStrategy  PartitionKeyStrategy
}

func NewInbox(
	sessionPool session.SessionPool,
	table string,
	sequence string,
	partitionKeyStrategy PartitionKeyStrategy,
) *PgInbox {
	if table == "" {
		table = "inbox"
	}
	if sequence == "" {
		sequence = "inbox_received_position_seq"
	}
	if partitionKeyStrategy == nil {
		partitionKeyStrategy = &UriPartitionKeyStrategy{}
	}
	return &PgInbox{
		sessionPool:          sessionPool,
		table:                table,
		sequence:             sequence,
		partitionKeyStrategy: partitionKeyStrategy,
	}
}

func (i *PgInbox) Publish(message *InboxMessage) error {
	ctx := context.Background()
	return i.sessionPool.Session(ctx, func(s session.Session) error {
		return s.Atomic(func(txSession session.Session) error {
			return i.insertMessage(txSession, message)
		})
	})
}

func (i *PgInbox) Dispatch(subscriber Subscriber, workerID int, numWorkers int) (bool, error) {
	ctx := context.Background()

	var message *InboxMessage
	err := i.sessionPool.Session(ctx, func(s session.Session) error {
		return s.Atomic(func(txSession session.Session) error {
			var err error
			message, err = i.fetchNextProcessable(txSession, 0, workerID, numWorkers)
			if err != nil {
				return err
			}

			if message == nil {
				return nil
			}

			if err := subscriber(txSession, message); err != nil {
				return err
			}

			return i.markProcessed(txSession, message)
		})
	})

	if err != nil {
		return false, err
	}

	return message != nil, nil
}

func (i *PgInbox) Run(ctx context.Context, subscriber Subscriber, processID int, numProcesses int, concurrency int, pollInterval float64) error {
	effectiveTotal := numProcesses * concurrency

	workerLoop := func(localID int) error {
		effectiveID := processID*concurrency + localID
		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			hasMessages, err := i.Dispatch(subscriber, effectiveID, effectiveTotal)
			if err != nil {
				return err
			}
			if !hasMessages {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(time.Duration(pollInterval * float64(time.Second))):
				}
			}
		}
	}

	if concurrency == 1 {
		return workerLoop(0)
	}

	errCh := make(chan error, concurrency)
	for id := 0; id < concurrency; id++ {
		go func(workerID int) {
			errCh <- workerLoop(workerID)
		}(id)
	}

	return <-errCh
}

func (i *PgInbox) Messages(ctx context.Context, workerID int, numWorkers int, pollInterval float64) <-chan *SessionMessage {
	messageCh := make(chan *SessionMessage)

	go func() {
		defer close(messageCh)

		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			var message *InboxMessage

			bgCtx := context.Background()
			err := i.sessionPool.Session(bgCtx, func(s session.Session) error {
				return s.Atomic(func(txSession session.Session) error {
					var err error
					message, err = i.fetchNextProcessable(txSession, 0, workerID, numWorkers)
					if err != nil {
						return err
					}

					if message == nil {
						return nil
					}

					// Yield message to caller
					select {
					case <-ctx.Done():
						return ctx.Err()
					case messageCh <- &SessionMessage{Session: txSession, Message: message}:
					}

					// Mark as processed after yield
					return i.markProcessed(txSession, message)
				})
			})

			if err != nil {
				if err == context.Canceled || err == context.DeadlineExceeded {
					return
				}
				continue
			}

			if message == nil {
				select {
				case <-ctx.Done():
					return
				case <-time.After(time.Duration(pollInterval * float64(time.Second))):
				}
			}
		}
	}()

	return messageCh
}

func (i *PgInbox) insertMessage(s session.Session, message *InboxMessage) error {
	sql := fmt.Sprintf(`
		INSERT INTO %s (
			tenant_id, stream_type, stream_id, stream_position,
			uri, payload, metadata
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7
		)
		ON CONFLICT (tenant_id, stream_type, stream_id, stream_position) DO NOTHING
	`, i.table)

	streamIDBytes, err := json.Marshal(message.StreamId)
	if err != nil {
		return err
	}

	payloadBytes, err := json.Marshal(message.Payload)
	if err != nil {
		return err
	}

	var metadataBytes []byte
	if message.Metadata != nil {
		metadataBytes, err = json.Marshal(message.Metadata)
		if err != nil {
			return err
		}
	}

	_, err = s.(session.DbSession).Connection().Exec(
		sql,
		message.TenantId,
		message.StreamType,
		streamIDBytes,
		message.StreamPosition,
		message.Uri,
		payloadBytes,
		metadataBytes,
	)
	return err
}

func (i *PgInbox) fetchNextProcessable(
	s session.Session,
	startOffset int,
	workerID int,
	numWorkers int,
) (*InboxMessage, error) {
	offset := startOffset
	for {
		message, err := i.fetchUnprocessedMessage(s, offset, workerID, numWorkers)
		if err != nil {
			return nil, err
		}
		if message == nil {
			return nil, nil
		}

		satisfied, err := i.areDependenciesSatisfied(s, message)
		if err != nil {
			return nil, err
		}
		if satisfied {
			return message, nil
		}

		offset++
	}
}

func (i *PgInbox) fetchUnprocessedMessage(
	s session.Session,
	offset int,
	workerID int,
	numWorkers int,
) (*InboxMessage, error) {
	partitionFilter := ""
	if numWorkers > 1 {
		partitionExpr := i.partitionKeyStrategy.GetSqlExpression()
		partitionFilter = fmt.Sprintf(
			"AND hashtext(%s) %% %d = %d",
			partitionExpr,
			numWorkers,
			workerID,
		)
	}

	sql := fmt.Sprintf(`
		SELECT
			tenant_id, stream_type, stream_id, stream_position,
			uri, payload, metadata,
			received_position, processed_position
		FROM %s
		WHERE processed_position IS NULL
		%s
		ORDER BY received_position ASC
		LIMIT 1 OFFSET $1
		FOR UPDATE SKIP LOCKED
	`, i.table, partitionFilter)

	row := s.(session.DbSession).Connection().QueryRow(sql, offset)

	var tenantID string
	var streamType string
	var streamIDBytes []byte
	var streamPosition int
	var uri string
	var payloadBytes []byte
	var metadataBytes []byte
	var receivedPosition int64
	var processedPosition *int64

	err := row.Scan(
		&tenantID,
		&streamType,
		&streamIDBytes,
		&streamPosition,
		&uri,
		&payloadBytes,
		&metadataBytes,
		&receivedPosition,
		&processedPosition,
	)

	if err != nil {
		// No rows found
		if err.Error() == "no rows in result set" {
			return nil, nil
		}
		return nil, err
	}

	var streamID map[string]any
	if err := json.Unmarshal(streamIDBytes, &streamID); err != nil {
		return nil, err
	}

	var payload map[string]any
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		return nil, err
	}

	var metadata map[string]any
	if len(metadataBytes) > 0 {
		if err := json.Unmarshal(metadataBytes, &metadata); err != nil {
			return nil, err
		}
	}

	return &InboxMessage{
		TenantId:          tenantID,
		StreamType:        streamType,
		StreamId:          streamID,
		StreamPosition:    streamPosition,
		Uri:               uri,
		Payload:           payload,
		Metadata:          metadata,
		ReceivedPosition:  &receivedPosition,
		ProcessedPosition: processedPosition,
	}, nil
}

func (i *PgInbox) areDependenciesSatisfied(s session.Session, message *InboxMessage) (bool, error) {
	dependencies := message.CausalDependencies()
	if len(dependencies) == 0 {
		return true, nil
	}

	for _, dep := range dependencies {
		processed, err := i.isDependencyProcessed(s, dep)
		if err != nil {
			return false, err
		}
		if !processed {
			return false, nil
		}
	}

	return true, nil
}

func (i *PgInbox) isDependencyProcessed(s session.Session, dependency map[string]any) (bool, error) {
	sql := fmt.Sprintf(`
		SELECT 1 FROM %s
		WHERE tenant_id = $1
		  AND stream_type = $2
		  AND stream_id = $3
		  AND stream_position = $4
		  AND processed_position IS NOT NULL
		LIMIT 1
	`, i.table)

	streamIDBytes, err := json.Marshal(dependency["stream_id"])
	if err != nil {
		return false, err
	}

	row := s.(session.DbSession).Connection().QueryRow(
		sql,
		dependency["tenant_id"],
		dependency["stream_type"],
		streamIDBytes,
		dependency["stream_position"],
	)

	var exists int
	err = row.Scan(&exists)
	if err != nil {
		// No rows found
		if err.Error() == "no rows in result set" {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

func (i *PgInbox) markProcessed(s session.Session, message *InboxMessage) error {
	sql := fmt.Sprintf(`
		UPDATE %s
		SET processed_position = nextval('%s')
		WHERE tenant_id = $1
		  AND stream_type = $2
		  AND stream_id = $3
		  AND stream_position = $4
	`, i.table, i.sequence)

	streamIDBytes, err := json.Marshal(message.StreamId)
	if err != nil {
		return err
	}

	_, err = s.(session.DbSession).Connection().Exec(
		sql,
		message.TenantId,
		message.StreamType,
		streamIDBytes,
		message.StreamPosition,
	)
	return err
}

func (i *PgInbox) Setup() error {
	ctx := context.Background()
	return i.sessionPool.Session(ctx, func(s session.Session) error {
		return s.Atomic(func(txSession session.Session) error {
			if err := i.createSequence(txSession); err != nil {
				return err
			}
			return i.createTable(txSession)
		})
	})
}

func (i *PgInbox) createSequence(s session.Session) error {
	sql := fmt.Sprintf("CREATE SEQUENCE IF NOT EXISTS %s", i.sequence)
	_, err := s.(session.DbSession).Connection().Exec(sql)
	return err
}

func (i *PgInbox) createTable(s session.Session) error {
	sql := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			tenant_id varchar(128) NOT NULL,
			stream_type varchar(128) NOT NULL,
			stream_id jsonb NOT NULL,
			stream_position integer NOT NULL,
			uri varchar(60) NOT NULL,
			payload jsonb NOT NULL,
			metadata jsonb NULL,
			received_position bigint NOT NULL UNIQUE DEFAULT nextval('%s'),
			processed_position bigint NULL,
			CONSTRAINT %s_pk PRIMARY KEY (tenant_id, stream_type, stream_id, stream_position)
		)
	`, i.table, i.sequence, i.table)

	_, err := s.(session.DbSession).Connection().Exec(sql)
	return err
}

func (i *PgInbox) Cleanup() error {
	return nil
}

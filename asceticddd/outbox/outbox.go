package outbox

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/session"
)

type PgOutbox struct {
	sessionPool  session.SessionPool
	outboxTable  string
	offsetsTable string
	batchSize    int
}

func NewOutbox(
	sessionPool session.SessionPool,
	outboxTable string,
	offsetsTable string,
	batchSize int,
) *PgOutbox {
	if outboxTable == "" {
		outboxTable = "outbox"
	}
	if offsetsTable == "" {
		offsetsTable = "outbox_offsets"
	}
	if batchSize == 0 {
		batchSize = 100
	}
	return &PgOutbox{
		sessionPool:  sessionPool,
		outboxTable:  outboxTable,
		offsetsTable: offsetsTable,
		batchSize:    batchSize,
	}
}

func (o *PgOutbox) Publish(s session.DbSession, message *OutboxMessage) error {
	sql := fmt.Sprintf(`
		INSERT INTO %s (uri, payload, metadata, transaction_id)
		VALUES ($1, $2, $3, pg_current_xact_id())
	`, o.outboxTable)

	payload, err := json.Marshal(message.Payload)
	if err != nil {
		return err
	}

	metadata, err := json.Marshal(message.Metadata)
	if err != nil {
		return err
	}

	_, err = s.Connection().Exec(sql, message.URI, payload, metadata)
	return err
}

func (o *PgOutbox) Dispatch(subscriber Subscriber, consumerGroup string, uri string, workerID int, numWorkers int) (bool, error) {
	effectiveConsumerGroup := consumerGroup
	if numWorkers > 1 {
		effectiveConsumerGroup = fmt.Sprintf("%s:%d", consumerGroup, workerID)
	}

	ctx := context.Background()

	err := o.sessionPool.Session(ctx, func(s session.Session) error {
		return o.ensureConsumerGroup(s.(session.DbSession), effectiveConsumerGroup, uri)
	})
	if err != nil {
		return false, err
	}

	var messages []*OutboxMessage
	err = o.sessionPool.Session(ctx, func(s session.Session) error {
		return s.Atomic(func(txSession session.Session) error {
			var err error
			messages, err = o.fetchMessages(txSession.(session.DbSession), effectiveConsumerGroup, uri, workerID, numWorkers)
			if err != nil {
				return err
			}

			if len(messages) == 0 {
				return nil
			}

			for _, msg := range messages {
				if err := subscriber(msg); err != nil {
					return err
				}
			}

			last := messages[len(messages)-1]
			return o.ackMessage(txSession.(session.DbSession), effectiveConsumerGroup, uri, *last.TransactionID, *last.Position)
		})
	})

	if err != nil {
		return false, err
	}

	return len(messages) > 0, nil
}

func (o *PgOutbox) Run(ctx context.Context, subscriber Subscriber, consumerGroup string, uri string, processID int, numProcesses int, concurrency int, pollInterval float64) error {
	effectiveTotal := numProcesses * concurrency

	workerLoop := func(localID int) error {
		effectiveID := processID*concurrency + localID
		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			hasMessages, err := o.Dispatch(subscriber, consumerGroup, uri, effectiveID, effectiveTotal)
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
	for i := 0; i < concurrency; i++ {
		go func(id int) {
			errCh <- workerLoop(id)
		}(i)
	}

	return <-errCh
}

func (o *PgOutbox) GetPosition(s session.DbSession, consumerGroup string, uri string) (int64, int64, error) {
	sql := fmt.Sprintf(`
		SELECT last_processed_transaction_id, offset_acked
		FROM %s
		WHERE consumer_group = $1 AND uri = $2
	`, o.offsetsTable)

	row := s.Connection().QueryRow(sql, consumerGroup, uri)
	var transactionID int64
	var offset int64
	err := row.Scan(&transactionID, &offset)
	if err != nil {
		return 0, 0, nil
	}
	return transactionID, offset, nil
}

func (o *PgOutbox) SetPosition(s session.DbSession, consumerGroup string, uri string, transactionID int64, offset int64) error {
	sql := fmt.Sprintf(`
		INSERT INTO %s (consumer_group, uri, offset_acked, last_processed_transaction_id, updated_at)
		VALUES ($1, $2, $3, $4, CURRENT_TIMESTAMP)
		ON CONFLICT (consumer_group, uri) DO UPDATE SET
			offset_acked = EXCLUDED.offset_acked,
			last_processed_transaction_id = EXCLUDED.last_processed_transaction_id,
			updated_at = EXCLUDED.updated_at
	`, o.offsetsTable)

	_, err := s.Connection().Exec(sql, consumerGroup, uri, offset, fmt.Sprintf("%d", transactionID))
	return err
}

func (o *PgOutbox) Setup() error {
	ctx := context.Background()
	return o.sessionPool.Session(ctx, func(s session.Session) error {
		return s.Atomic(func(txSession session.Session) error {
			dbSession := txSession.(session.DbSession)
			if err := o.createOutboxTable(dbSession); err != nil {
				return err
			}
			return o.createOffsetsTable(dbSession)
		})
	})
}

func (o *PgOutbox) Cleanup() error {
	return nil
}

func (o *PgOutbox) ensureConsumerGroup(s session.DbSession, consumerGroup string, uri string) error {
	sql := fmt.Sprintf(`
		INSERT INTO %s (consumer_group, uri, offset_acked, last_processed_transaction_id)
		VALUES ($1, $2, 0, '0')
		ON CONFLICT DO NOTHING
	`, o.offsetsTable)

	_, err := s.Connection().Exec(sql, consumerGroup, uri)
	return err
}

func (o *PgOutbox) fetchMessages(s session.DbSession, consumerGroup string, uri string, workerID int, numWorkers int) ([]*OutboxMessage, error) {
	args := []any{consumerGroup, uri}
	paramNum := 3

	uriFilter := ""
	if uri != "" {
		uriFilter = fmt.Sprintf("AND (uri = $%d OR uri LIKE $%d)", paramNum, paramNum+1)
		args = append(args, uri, uri+"/%")
		paramNum += 2
	}

	partitionFilter := ""
	if numWorkers > 1 {
		partitionFilter = fmt.Sprintf("AND hashtext(uri) %% $%d = $%d", paramNum, paramNum+1)
		args = append(args, numWorkers, workerID)
	}

	sql := fmt.Sprintf(`
		SELECT * FROM (
			WITH last_processed AS (
				SELECT offset_acked, last_processed_transaction_id
				FROM %s
				WHERE consumer_group = $1 AND uri = $2
				FOR UPDATE
			)
			SELECT "position", transaction_id, uri, payload, metadata, created_at
			FROM %s
			WHERE (
				(transaction_id = (SELECT last_processed_transaction_id FROM last_processed)
				 AND "position" > (SELECT offset_acked FROM last_processed))
				OR
				(transaction_id > (SELECT last_processed_transaction_id FROM last_processed))
			)
			AND transaction_id < pg_snapshot_xmin(pg_current_snapshot())
			%s
			%s
		) AS messages
		ORDER BY transaction_id ASC, "position" ASC
		LIMIT %d
	`, o.offsetsTable, o.outboxTable, uriFilter, partitionFilter, o.batchSize)

	rows, err := s.Connection().Query(sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []*OutboxMessage
	for rows.Next() {
		var position int64
		var transactionID int64
		var uri string
		var payloadBytes []byte
		var metadataBytes []byte
		var createdAt time.Time

		err := rows.Scan(&position, &transactionID, &uri, &payloadBytes, &metadataBytes, &createdAt)
		if err != nil {
			return nil, err
		}

		var payload map[string]any
		if err := json.Unmarshal(payloadBytes, &payload); err != nil {
			return nil, err
		}

		var metadata map[string]any
		if err := json.Unmarshal(metadataBytes, &metadata); err != nil {
			return nil, err
		}

		createdAtStr := createdAt.Format(time.RFC3339)
		messages = append(messages, &OutboxMessage{
			URI:           uri,
			Payload:       payload,
			Metadata:      metadata,
			CreatedAt:     &createdAtStr,
			Position:      &position,
			TransactionID: &transactionID,
		})
	}

	return messages, rows.Err()
}

func (o *PgOutbox) ackMessage(s session.DbSession, consumerGroup string, uri string, transactionID int64, position int64) error {
	sql := fmt.Sprintf(`
		INSERT INTO %s (consumer_group, uri, offset_acked, last_processed_transaction_id, updated_at)
		VALUES ($1, $2, $3, $4, CURRENT_TIMESTAMP)
		ON CONFLICT (consumer_group, uri) DO UPDATE SET
			offset_acked = EXCLUDED.offset_acked,
			last_processed_transaction_id = EXCLUDED.last_processed_transaction_id,
			updated_at = EXCLUDED.updated_at
	`, o.offsetsTable)

	_, err := s.Connection().Exec(sql, consumerGroup, uri, position, fmt.Sprintf("%d", transactionID))
	return err
}

func (o *PgOutbox) createOutboxTable(s session.DbSession) error {
	sql := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			"position" BIGSERIAL,
			"uri" VARCHAR(255) NOT NULL,
			"payload" JSONB NOT NULL,
			"metadata" JSONB NOT NULL,
			"created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			"transaction_id" xid8 NOT NULL,
			PRIMARY KEY ("transaction_id", "position")
		)
	`, o.outboxTable)

	if _, err := s.Connection().Exec(sql); err != nil {
		return err
	}

	sqls := []string{
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS %s_position_idx ON %s ("position")`, o.outboxTable, o.outboxTable),
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS %s_uri_idx ON %s ("uri")`, o.outboxTable, o.outboxTable),
		fmt.Sprintf(`CREATE UNIQUE INDEX IF NOT EXISTS %s_event_id_uniq ON %s (((metadata->>'event_id')::uuid))`, o.outboxTable, o.outboxTable),
	}

	for _, sql := range sqls {
		if _, err := s.Connection().Exec(sql); err != nil {
			return err
		}
	}

	return nil
}

func (o *PgOutbox) createOffsetsTable(s session.DbSession) error {
	sql := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			"consumer_group" VARCHAR(255) NOT NULL,
			"uri" VARCHAR(255) NOT NULL DEFAULT '',
			"offset_acked" BIGINT NOT NULL DEFAULT 0,
			"last_processed_transaction_id" xid8 NOT NULL DEFAULT '0',
			"updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY ("consumer_group", "uri")
		)
	`, o.offsetsTable)

	_, err := s.Connection().Exec(sql)
	return err
}

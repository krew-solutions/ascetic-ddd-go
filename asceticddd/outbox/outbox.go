package outbox

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/session"
)

type PgOutbox struct {
	sessionPool  session.SessionPool
	outboxTable  string
	offsetsTable string
	batchSize    int
	pollInterval time.Duration
	logger       *slog.Logger
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
		pollInterval: time.Second,
		logger:       slog.Default(),
	}
}

func (o *PgOutbox) Publish(s session.Session, message *OutboxMessage) error {
	sql := fmt.Sprintf(`
		INSERT INTO %s (uri, payload, metadata, transaction_id)
		VALUES ($1, $2, $3, pg_current_xact_id())
	`, o.outboxTable)

	metadata, err := json.Marshal(message.Metadata)
	if err != nil {
		return err
	}

	_, err = s.(session.DbSession).Connection().Exec(sql, message.Uri, message.Payload, metadata)
	return err
}

func (o *PgOutbox) Dispatch(ctx context.Context, subscriber Subscriber, consumerGroup string, uri string, workerId int, numWorkers int) (bool, error) {
	effectiveConsumerGroup := consumerGroup
	if numWorkers > 1 {
		effectiveConsumerGroup = fmt.Sprintf("%s:%d", consumerGroup, workerId)
	}

	err := o.sessionPool.Session(ctx, func(s session.Session) error {
		return o.ensureConsumerGroup(s, effectiveConsumerGroup, uri)
	})
	if err != nil {
		return false, err
	}

	var messages []*OutboxMessage
	err = o.sessionPool.Session(ctx, func(s session.Session) error {
		return s.Atomic(func(txSession session.Session) error {
			var err error
			messages, err = o.fetchMessages(txSession, effectiveConsumerGroup, uri, workerId, numWorkers)
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
			return o.ackMessage(txSession, effectiveConsumerGroup, uri, *last.TransactionId, *last.Position)
		})
	})

	if err != nil {
		return false, err
	}

	return len(messages) > 0, nil
}

func (o *PgOutbox) Run(ctx context.Context, subscriber Subscriber, consumerGroup string, uri string, processId int, numProcesses int, concurrency int, pollInterval float64) error {
	if concurrency < 1 {
		concurrency = 1
	}
	if numProcesses < 1 {
		numProcesses = 1
	}
	effectiveTotal := numProcesses * concurrency

	// A failing worker stops the others cooperatively, between batches.
	loopCtx, stop := context.WithCancel(ctx)
	defer stop()

	var (
		wg       sync.WaitGroup
		once     sync.Once
		firstErr error
	)
	for localId := 0; localId < concurrency; localId++ {
		wg.Add(1)
		go func(effectiveId int) {
			defer wg.Done()
			if err := o.workerLoop(loopCtx, subscriber, consumerGroup, uri, effectiveId, effectiveTotal, pollInterval); err != nil {
				once.Do(func() { firstErr = err })
				stop()
			}
		}(processId*concurrency + localId)
	}
	wg.Wait()

	if firstErr != nil {
		return firstErr
	}
	return ctx.Err()
}

// workerLoop dispatches as one worker until ctx is cancelled or a dispatch fails.
func (o *PgOutbox) workerLoop(ctx context.Context, subscriber Subscriber, consumerGroup string, uri string, workerId int, numWorkers int, pollInterval float64) error {
	// Shutdown is cooperative: the transaction carries ctx's values but not
	// its cancellation, so a batch being dispatched is finished and committed.
	dispatchCtx := context.WithoutCancel(ctx)
	for ctx.Err() == nil {
		hasMessages, err := o.Dispatch(dispatchCtx, subscriber, consumerGroup, uri, workerId, numWorkers)
		if err != nil {
			return err
		}
		if !hasMessages {
			select {
			case <-ctx.Done():
			case <-time.After(time.Duration(pollInterval * float64(time.Second))):
			}
		}
	}
	return nil
}

func (o *PgOutbox) GetPosition(s session.Session, consumerGroup string, uri string) (int64, int64, error) {
	sql := fmt.Sprintf(`
		SELECT last_processed_transaction_id, offset_acked
		FROM %s
		WHERE consumer_group = $1 AND uri = $2
	`, o.offsetsTable)

	row := s.(session.DbSession).Connection().QueryRow(sql, consumerGroup, uri)
	var transactionId int64
	var offset int64
	err := row.Scan(&transactionId, &offset)
	if err != nil {
		return 0, 0, nil
	}
	return transactionId, offset, nil
}

func (o *PgOutbox) SetPosition(s session.Session, consumerGroup string, uri string, transactionId int64, offset int64) error {
	sql := fmt.Sprintf(`
		INSERT INTO %s (consumer_group, uri, offset_acked, last_processed_transaction_id, updated_at)
		VALUES ($1, $2, $3, $4, CURRENT_TIMESTAMP)
		ON CONFLICT (consumer_group, uri) DO UPDATE SET
			offset_acked = EXCLUDED.offset_acked,
			last_processed_transaction_id = EXCLUDED.last_processed_transaction_id,
			updated_at = EXCLUDED.updated_at
	`, o.offsetsTable)

	_, err := s.(session.DbSession).Connection().Exec(sql, consumerGroup, uri, offset, fmt.Sprintf("%d", transactionId))
	return err
}

func (o *PgOutbox) Setup(s session.Session) error {
	if err := o.createOutboxTable(s); err != nil {
		return err
	}
	return o.createOffsetsTable(s)
}

func (o *PgOutbox) Cleanup(s session.Session) error {
	return nil
}

func (o *PgOutbox) ensureConsumerGroup(s session.Session, consumerGroup string, uri string) error {
	sql := fmt.Sprintf(`
		INSERT INTO %s (consumer_group, uri, offset_acked, last_processed_transaction_id)
		VALUES ($1, $2, 0, '0')
		ON CONFLICT DO NOTHING
	`, o.offsetsTable)

	_, err := s.(session.DbSession).Connection().Exec(sql, consumerGroup, uri)
	return err
}

func (o *PgOutbox) fetchMessages(s session.Session, consumerGroup string, uri string, workerId int, numWorkers int) ([]*OutboxMessage, error) {
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
		// hashtext() is signed and % keeps the sign of the dividend, so a
		// negative hash would match no worker; the sign bit is cleared.
		partitionFilter = fmt.Sprintf("AND (hashtext(uri) & 2147483647) %% $%d = $%d", paramNum, paramNum+1)
		args = append(args, numWorkers, workerId)
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

	rows, err := s.(session.DbSession).Connection().Query(sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []*OutboxMessage
	for rows.Next() {
		var position int64
		var transactionId int64
		var uri string
		var payloadBytes []byte
		var metadataBytes []byte
		var createdAt time.Time

		err := rows.Scan(&position, &transactionId, &uri, &payloadBytes, &metadataBytes, &createdAt)
		if err != nil {
			return nil, err
		}

		var metadata map[string]any
		if err := json.Unmarshal(metadataBytes, &metadata); err != nil {
			return nil, err
		}

		createdAtStr := createdAt.Format(time.RFC3339)
		messages = append(messages, &OutboxMessage{
			Uri:           uri,
			Payload:       payloadBytes,
			Metadata:      metadata,
			CreatedAt:     &createdAtStr,
			Position:      &position,
			TransactionId: &transactionId,
		})
	}

	return messages, rows.Err()
}

func (o *PgOutbox) ackMessage(s session.Session, consumerGroup string, uri string, transactionId int64, position int64) error {
	sql := fmt.Sprintf(`
		INSERT INTO %s (consumer_group, uri, offset_acked, last_processed_transaction_id, updated_at)
		VALUES ($1, $2, $3, $4, CURRENT_TIMESTAMP)
		ON CONFLICT (consumer_group, uri) DO UPDATE SET
			offset_acked = EXCLUDED.offset_acked,
			last_processed_transaction_id = EXCLUDED.last_processed_transaction_id,
			updated_at = EXCLUDED.updated_at
	`, o.offsetsTable)

	_, err := s.(session.DbSession).Connection().Exec(sql, consumerGroup, uri, position, fmt.Sprintf("%d", transactionId))
	return err
}

func (o *PgOutbox) createOutboxTable(s session.Session) error {
	sql := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			"position" BIGSERIAL,
			"uri" VARCHAR(255) NOT NULL,
			"payload" BYTEA NOT NULL,
			"metadata" JSONB NOT NULL,
			"created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			"transaction_id" xid8 NOT NULL,
			PRIMARY KEY ("transaction_id", "position")
		)
	`, o.outboxTable)

	conn := s.(session.DbSession).Connection()
	if _, err := conn.Exec(sql); err != nil {
		return err
	}

	sqls := []string{
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS %s_position_idx ON %s ("position")`, o.outboxTable, o.outboxTable),
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS %s_uri_idx ON %s ("uri")`, o.outboxTable, o.outboxTable),
		fmt.Sprintf(`CREATE UNIQUE INDEX IF NOT EXISTS %s_message_id_uniq ON %s (((metadata->>'message_id')::uuid))`, o.outboxTable, o.outboxTable),
	}

	for _, sql := range sqls {
		if _, err := conn.Exec(sql); err != nil {
			return err
		}
	}

	return nil
}

func (o *PgOutbox) createOffsetsTable(s session.Session) error {
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

	_, err := s.(session.DbSession).Connection().Exec(sql)
	return err
}

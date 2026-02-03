package outbox

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/session"
	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/utils/testutils"
)

const (
	testOutboxTable  = "outbox_test"
	testOffsetsTable = "outbox_offsets_test"
)

func setupOutbox(t *testing.T) (*PgOutbox, session.SessionPool) {
	pool, err := testutils.NewPgxSessionPool()
	require.NoError(t, err)

	outbox := NewOutbox(pool, testOutboxTable, testOffsetsTable, 100)
	err = outbox.Setup()
	require.NoError(t, err)

	truncateTables(t, pool)

	return outbox, pool
}

func truncateTables(t *testing.T, pool session.SessionPool) {
	ctx := context.Background()
	err := pool.Session(ctx, func(s session.Session) error {
		dbSession := s.(session.DbSession)
		_, err := dbSession.Connection().Exec("TRUNCATE TABLE " + testOutboxTable)
		if err != nil {
			return err
		}
		_, err = dbSession.Connection().Exec("TRUNCATE TABLE " + testOffsetsTable)
		return err
	})
	require.NoError(t, err)
}

func dropTables(t *testing.T, pool session.SessionPool) {
	ctx := context.Background()
	_ = pool.Session(ctx, func(s session.Session) error {
		dbSession := s.(session.DbSession)
		_, _ = dbSession.Connection().Exec("DROP TABLE IF EXISTS " + testOutboxTable)
		_, _ = dbSession.Connection().Exec("DROP TABLE IF EXISTS " + testOffsetsTable)
		return nil
	})
}

func TestPublishAndDispatch(t *testing.T) {
	outbox, pool := setupOutbox(t)
	defer dropTables(t, pool)

	ctx := context.Background()

	err := pool.Session(ctx, func(s session.Session) error {
		return s.Atomic(func(txSession session.Session) error {
			message := &OutboxMessage{
				URI: "kafka://orders",
				Payload: map[string]any{
					"type":     "OrderCreated",
					"order_id": "123",
					"amount":   100,
				},
				Metadata: map[string]any{
					"event_id": "550e8400-e29b-41d4-a716-446655440001",
				},
			}
			return outbox.Publish(txSession.(session.DbSession), message)
		})
	})
	require.NoError(t, err)

	var publishedMessages []*OutboxMessage
	subscriber := func(msg *OutboxMessage) error {
		publishedMessages = append(publishedMessages, msg)
		return nil
	}

	result, err := outbox.Dispatch(subscriber, "", "", 0, 1)
	require.NoError(t, err)

	assert.True(t, result)
	assert.Len(t, publishedMessages, 1)
	assert.Equal(t, "kafka://orders", publishedMessages[0].URI)
	assert.Equal(t, "123", publishedMessages[0].Payload["order_id"])
}

func TestDispatchReturnsFalseWhenEmpty(t *testing.T) {
	outbox, pool := setupOutbox(t)
	defer dropTables(t, pool)

	var publishedMessages []*OutboxMessage
	subscriber := func(msg *OutboxMessage) error {
		publishedMessages = append(publishedMessages, msg)
		return nil
	}

	result, err := outbox.Dispatch(subscriber, "", "", 0, 1)
	require.NoError(t, err)

	assert.False(t, result)
	assert.Len(t, publishedMessages, 0)
}

func TestMultipleConsumerGroups(t *testing.T) {
	outbox, pool := setupOutbox(t)
	defer dropTables(t, pool)

	ctx := context.Background()

	err := pool.Session(ctx, func(s session.Session) error {
		return s.Atomic(func(txSession session.Session) error {
			message := &OutboxMessage{
				URI: "kafka://orders",
				Payload: map[string]any{
					"type":     "OrderCreated",
					"order_id": "123",
				},
				Metadata: map[string]any{
					"event_id": "550e8400-e29b-41d4-a716-446655440003",
				},
			}
			return outbox.Publish(txSession.(session.DbSession), message)
		})
	})
	require.NoError(t, err)

	var publishedMessages []*OutboxMessage
	subscriber := func(msg *OutboxMessage) error {
		publishedMessages = append(publishedMessages, msg)
		return nil
	}

	result1, err := outbox.Dispatch(subscriber, "group-1", "", 0, 1)
	require.NoError(t, err)
	assert.True(t, result1)
	assert.Len(t, publishedMessages, 1)

	result2, err := outbox.Dispatch(subscriber, "group-2", "", 0, 1)
	require.NoError(t, err)
	assert.True(t, result2)
	assert.Len(t, publishedMessages, 2)

	assert.Equal(t, "kafka://orders", publishedMessages[0].URI)
	assert.Equal(t, "kafka://orders", publishedMessages[1].URI)
}

func TestGetAndSetPosition(t *testing.T) {
	outbox, pool := setupOutbox(t)
	defer dropTables(t, pool)

	ctx := context.Background()

	err := pool.Session(ctx, func(s session.Session) error {
		dbSession := s.(session.DbSession)

		txID, offset, err := outbox.GetPosition(dbSession, "test-group", "")
		if err != nil {
			return err
		}
		assert.Equal(t, int64(0), txID)
		assert.Equal(t, int64(0), offset)

		err = outbox.SetPosition(dbSession, "test-group", "", 100, 50)
		if err != nil {
			return err
		}

		txID, offset, err = outbox.GetPosition(dbSession, "test-group", "")
		if err != nil {
			return err
		}
		assert.Equal(t, int64(100), txID)
		assert.Equal(t, int64(50), offset)

		return nil
	})
	require.NoError(t, err)
}

func TestGetAndSetPositionWithURI(t *testing.T) {
	outbox, pool := setupOutbox(t)
	defer dropTables(t, pool)

	ctx := context.Background()

	err := pool.Session(ctx, func(s session.Session) error {
		dbSession := s.(session.DbSession)

		err := outbox.SetPosition(dbSession, "test-group", "kafka://orders", 100, 50)
		if err != nil {
			return err
		}

		err = outbox.SetPosition(dbSession, "test-group", "kafka://users", 200, 30)
		if err != nil {
			return err
		}

		txIDOrders, offsetOrders, err := outbox.GetPosition(dbSession, "test-group", "kafka://orders")
		if err != nil {
			return err
		}
		assert.Equal(t, int64(100), txIDOrders)
		assert.Equal(t, int64(50), offsetOrders)

		txIDUsers, offsetUsers, err := outbox.GetPosition(dbSession, "test-group", "kafka://users")
		if err != nil {
			return err
		}
		assert.Equal(t, int64(200), txIDUsers)
		assert.Equal(t, int64(30), offsetUsers)

		return nil
	})
	require.NoError(t, err)
}

func TestDispatchUpdatesPosition(t *testing.T) {
	outbox, pool := setupOutbox(t)
	defer dropTables(t, pool)

	ctx := context.Background()

	err := pool.Session(ctx, func(s session.Session) error {
		return s.Atomic(func(txSession session.Session) error {
			message := &OutboxMessage{
				URI: "kafka://orders",
				Payload: map[string]any{
					"type":     "OrderCreated",
					"order_id": "123",
				},
				Metadata: map[string]any{
					"event_id": "550e8400-e29b-41d4-a716-446655440002",
				},
			}
			return outbox.Publish(txSession.(session.DbSession), message)
		})
	})
	require.NoError(t, err)

	var publishedMessages []*OutboxMessage
	subscriber := func(msg *OutboxMessage) error {
		publishedMessages = append(publishedMessages, msg)
		return nil
	}

	result1, err := outbox.Dispatch(subscriber, "test-group", "", 0, 1)
	require.NoError(t, err)
	assert.True(t, result1)

	result2, err := outbox.Dispatch(subscriber, "test-group", "", 0, 1)
	require.NoError(t, err)
	assert.False(t, result2)
}

func TestOrderingByPosition(t *testing.T) {
	outbox, pool := setupOutbox(t)
	defer dropTables(t, pool)

	ctx := context.Background()

	for i := 0; i < 3; i++ {
		err := pool.Session(ctx, func(s session.Session) error {
			return s.Atomic(func(txSession session.Session) error {
				message := &OutboxMessage{
					URI: "kafka://orders",
					Payload: map[string]any{
						"type":  "OrderCreated",
						"order": i,
					},
					Metadata: map[string]any{
						"event_id": fmt.Sprintf("550e8400-e29b-41d4-a716-44665544000%d", i),
					},
				}
				return outbox.Publish(txSession.(session.DbSession), message)
			})
		})
		require.NoError(t, err)
	}

	var publishedMessages []*OutboxMessage
	subscriber := func(msg *OutboxMessage) error {
		publishedMessages = append(publishedMessages, msg)
		return nil
	}

	for {
		hasMessages, err := outbox.Dispatch(subscriber, "", "", 0, 1)
		require.NoError(t, err)
		if !hasMessages {
			break
		}
	}

	assert.Len(t, publishedMessages, 3)
	for i, msg := range publishedMessages {
		assert.Equal(t, float64(i), msg.Payload["order"])
	}
}

func TestBatchDispatch(t *testing.T) {
	outbox, pool := setupOutbox(t)
	defer dropTables(t, pool)

	ctx := context.Background()

	err := pool.Session(ctx, func(s session.Session) error {
		return s.Atomic(func(txSession session.Session) error {
			for i := 0; i < 5; i++ {
				message := &OutboxMessage{
					URI: "kafka://orders",
					Payload: map[string]any{
						"type":  "OrderCreated",
						"order": i,
					},
					Metadata: map[string]any{
						"event_id": fmt.Sprintf("550e8400-e29b-41d4-a716-44665544010%d", i),
					},
				}
				err := outbox.Publish(txSession.(session.DbSession), message)
				if err != nil {
					return err
				}
			}
			return nil
		})
	})
	require.NoError(t, err)

	var publishedMessages []*OutboxMessage
	subscriber := func(msg *OutboxMessage) error {
		publishedMessages = append(publishedMessages, msg)
		return nil
	}

	result, err := outbox.Dispatch(subscriber, "", "", 0, 1)
	require.NoError(t, err)

	assert.True(t, result)
	assert.Len(t, publishedMessages, 5)
}

func TestDispatchWithURIFilter(t *testing.T) {
	outbox, pool := setupOutbox(t)
	defer dropTables(t, pool)

	ctx := context.Background()

	err := pool.Session(ctx, func(s session.Session) error {
		return s.Atomic(func(txSession session.Session) error {
			messages := []*OutboxMessage{
				{
					URI: "kafka://orders",
					Payload: map[string]any{
						"type":     "OrderCreated",
						"order_id": "1",
					},
					Metadata: map[string]any{
						"event_id": "550e8400-e29b-41d4-a716-446655440080",
					},
				},
				{
					URI: "kafka://users",
					Payload: map[string]any{
						"type":    "UserCreated",
						"user_id": "1",
					},
					Metadata: map[string]any{
						"event_id": "550e8400-e29b-41d4-a716-446655440081",
					},
				},
				{
					URI: "kafka://orders",
					Payload: map[string]any{
						"type":     "OrderShipped",
						"order_id": "1",
					},
					Metadata: map[string]any{
						"event_id": "550e8400-e29b-41d4-a716-446655440082",
					},
				},
			}
			for _, msg := range messages {
				err := outbox.Publish(txSession.(session.DbSession), msg)
				if err != nil {
					return err
				}
			}
			return nil
		})
	})
	require.NoError(t, err)

	var publishedMessages []*OutboxMessage
	subscriber := func(msg *OutboxMessage) error {
		publishedMessages = append(publishedMessages, msg)
		return nil
	}

	result1, err := outbox.Dispatch(subscriber, "orders-consumer", "kafka://orders", 0, 1)
	require.NoError(t, err)
	assert.True(t, result1)
	assert.Len(t, publishedMessages, 2)
	for _, msg := range publishedMessages {
		assert.Equal(t, "kafka://orders", msg.URI)
	}

	result2, err := outbox.Dispatch(subscriber, "orders-consumer", "kafka://orders", 0, 1)
	require.NoError(t, err)
	assert.False(t, result2)

	result3, err := outbox.Dispatch(subscriber, "orders-consumer", "kafka://users", 0, 1)
	require.NoError(t, err)
	assert.True(t, result3)
	assert.Len(t, publishedMessages, 3)
	assert.Equal(t, "kafka://users", publishedMessages[2].URI)
}

func TestMultipleURIsIndependentPositions(t *testing.T) {
	outbox, pool := setupOutbox(t)
	defer dropTables(t, pool)

	ctx := context.Background()

	for i := 0; i < 3; i++ {
		err := pool.Session(ctx, func(s session.Session) error {
			return s.Atomic(func(txSession session.Session) error {
				orderMsg := &OutboxMessage{
					URI: "kafka://orders",
					Payload: map[string]any{
						"type":  "OrderCreated",
						"order": i,
					},
					Metadata: map[string]any{
						"event_id": fmt.Sprintf("550e8400-e29b-41d4-a716-44665544009%d", i),
					},
				}
				if err := outbox.Publish(txSession.(session.DbSession), orderMsg); err != nil {
					return err
				}

				userMsg := &OutboxMessage{
					URI: "kafka://users",
					Payload: map[string]any{
						"type": "UserCreated",
						"user": i,
					},
					Metadata: map[string]any{
						"event_id": fmt.Sprintf("550e8400-e29b-41d4-a716-44665544019%d", i),
					},
				}
				return outbox.Publish(txSession.(session.DbSession), userMsg)
			})
		})
		require.NoError(t, err)
	}

	var ordersMessages []*OutboxMessage
	ordersSubscriber := func(msg *OutboxMessage) error {
		ordersMessages = append(ordersMessages, msg)
		return nil
	}

	for {
		hasMessages, err := outbox.Dispatch(ordersSubscriber, "group1", "kafka://orders", 0, 1)
		require.NoError(t, err)
		if !hasMessages {
			break
		}
	}

	var usersMessages []*OutboxMessage
	usersSubscriber := func(msg *OutboxMessage) error {
		usersMessages = append(usersMessages, msg)
		return nil
	}

	for {
		hasMessages, err := outbox.Dispatch(usersSubscriber, "group1", "kafka://users", 0, 1)
		require.NoError(t, err)
		if !hasMessages {
			break
		}
	}

	assert.Len(t, ordersMessages, 3)
	assert.Len(t, usersMessages, 3)
	for _, msg := range ordersMessages {
		assert.Equal(t, "kafka://orders", msg.URI)
	}
	for _, msg := range usersMessages {
		assert.Equal(t, "kafka://users", msg.URI)
	}
}

func TestVisibilityRule(t *testing.T) {
	outbox, pool := setupOutbox(t)
	defer dropTables(t, pool)

	ctx := context.Background()

	var publishedMessages []*OutboxMessage
	subscriber := func(msg *OutboxMessage) error {
		publishedMessages = append(publishedMessages, msg)
		return nil
	}

	err := pool.Session(ctx, func(s session.Session) error {
		return s.Atomic(func(txSession session.Session) error {
			dbSession := txSession.(session.DbSession)
			_, err := dbSession.Connection().Exec(fmt.Sprintf(`
				INSERT INTO %s (uri, payload, metadata, transaction_id)
				VALUES ('kafka://orders', '{"type": "OrderCreated", "order": 1}'::jsonb,
						'{"event_id": "550e8400-e29b-41d4-a716-446655440050"}'::jsonb,
						pg_current_xact_id())
			`, testOutboxTable))
			if err != nil {
				return err
			}

			result, err := outbox.Dispatch(subscriber, "", "", 0, 1)
			require.NoError(t, err)
			assert.False(t, result)

			return fmt.Errorf("rollback transaction")
		})
	})
	require.Error(t, err)

	result, err := outbox.Dispatch(subscriber, "", "", 0, 1)
	require.NoError(t, err)
	assert.False(t, result)
	assert.Len(t, publishedMessages, 0)
}

func TestIdempotencyViaEventID(t *testing.T) {
	outbox, pool := setupOutbox(t)
	defer dropTables(t, pool)

	ctx := context.Background()

	err := pool.Session(ctx, func(s session.Session) error {
		return s.Atomic(func(txSession session.Session) error {
			message := &OutboxMessage{
				URI: "kafka://orders",
				Payload: map[string]any{
					"type":     "OrderCreated",
					"order_id": "123",
				},
				Metadata: map[string]any{
					"event_id": "550e8400-e29b-41d4-a716-446655440060",
				},
			}
			return outbox.Publish(txSession.(session.DbSession), message)
		})
	})
	require.NoError(t, err)

	err = pool.Session(ctx, func(s session.Session) error {
		return s.Atomic(func(txSession session.Session) error {
			message := &OutboxMessage{
				URI: "kafka://orders",
				Payload: map[string]any{
					"type":     "OrderCreated",
					"order_id": "456",
				},
				Metadata: map[string]any{
					"event_id": "550e8400-e29b-41d4-a716-446655440060",
				},
			}
			return outbox.Publish(txSession.(session.DbSession), message)
		})
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate")
}

func TestForUpdatePreventsDuplicateProcessing(t *testing.T) {
	outbox, pool := setupOutbox(t)
	defer dropTables(t, pool)

	ctx := context.Background()

	err := pool.Session(ctx, func(s session.Session) error {
		return s.Atomic(func(txSession session.Session) error {
			message := &OutboxMessage{
				URI: "kafka://orders",
				Payload: map[string]any{
					"type":     "OrderCreated",
					"order_id": "123",
				},
				Metadata: map[string]any{
					"event_id": "550e8400-e29b-41d4-a716-446655440070",
				},
			}
			return outbox.Publish(txSession.(session.DbSession), message)
		})
	})
	require.NoError(t, err)

	var publishedMessages []*OutboxMessage
	subscriber := func(msg *OutboxMessage) error {
		publishedMessages = append(publishedMessages, msg)
		return nil
	}

	type result struct {
		success bool
		err     error
	}

	results := make(chan result, 3)

	for i := 0; i < 3; i++ {
		go func() {
			success, err := outbox.Dispatch(subscriber, "test-group", "", 0, 1)
			results <- result{success, err}
		}()
	}

	successCount := 0
	for i := 0; i < 3; i++ {
		r := <-results
		require.NoError(t, r.err)
		if r.success {
			successCount++
		}
	}

	assert.Equal(t, 1, successCount)
	assert.Len(t, publishedMessages, 1)
}

func TestRunWithSingleWorker(t *testing.T) {
	outbox, pool := setupOutbox(t)
	defer dropTables(t, pool)

	ctx := context.Background()

	for i := 0; i < 3; i++ {
		err := pool.Session(ctx, func(s session.Session) error {
			return s.Atomic(func(txSession session.Session) error {
				message := &OutboxMessage{
					URI: "kafka://orders",
					Payload: map[string]any{
						"type":  "OrderCreated",
						"order": i,
					},
					Metadata: map[string]any{
						"event_id": fmt.Sprintf("550e8400-e29b-41d4-a716-44665544030%d", i),
					},
				}
				return outbox.Publish(txSession.(session.DbSession), message)
			})
		})
		require.NoError(t, err)
	}

	var publishedMessages []*OutboxMessage
	subscriber := func(msg *OutboxMessage) error {
		publishedMessages = append(publishedMessages, msg)
		return nil
	}

	runCtx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	err := outbox.Run(runCtx, subscriber, "", "", 0, 1, 1, 0.01)
	if err != nil && err != context.DeadlineExceeded {
		t.Fatalf("Run failed: %v", err)
	}

	assert.Len(t, publishedMessages, 3)
}

func TestRunWithMultipleWorkers(t *testing.T) {
	outbox, pool := setupOutbox(t)
	defer dropTables(t, pool)

	ctx := context.Background()

	for i := 0; i < 10; i++ {
		err := pool.Session(ctx, func(s session.Session) error {
			return s.Atomic(func(txSession session.Session) error {
				message := &OutboxMessage{
					URI: "kafka://orders",
					Payload: map[string]any{
						"type":  "OrderCreated",
						"order": i,
					},
					Metadata: map[string]any{
						"event_id": fmt.Sprintf("550e8400-e29b-41d4-a716-44665544040%d", i),
					},
				}
				return outbox.Publish(txSession.(session.DbSession), message)
			})
		})
		require.NoError(t, err)
	}

	var publishedMessages []*OutboxMessage
	subscriber := func(msg *OutboxMessage) error {
		publishedMessages = append(publishedMessages, msg)
		return nil
	}

	runCtx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	err := outbox.Run(runCtx, subscriber, "", "", 0, 1, 3, 0.01)
	if err != nil && err != context.DeadlineExceeded {
		t.Fatalf("Run failed: %v", err)
	}

	assert.Len(t, publishedMessages, 10)
}

func TestMessagesChannelAPI(t *testing.T) {
	outbox, pool := setupOutbox(t)
	defer dropTables(t, pool)

	ctx := context.Background()

	for i := 0; i < 5; i++ {
		err := pool.Session(ctx, func(s session.Session) error {
			return s.Atomic(func(txSession session.Session) error {
				message := &OutboxMessage{
					URI: "kafka://orders",
					Payload: map[string]any{
						"type":  "OrderCreated",
						"order": i,
					},
					Metadata: map[string]any{
						"event_id": fmt.Sprintf("550e8400-e29b-41d4-a716-44665544050%d", i),
					},
				}
				return outbox.Publish(txSession.(session.DbSession), message)
			})
		})
		require.NoError(t, err)
	}

	messageCtx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	var receivedMessages []*OutboxMessage

	for message := range outbox.Messages(messageCtx, "", "", 0, 1, 0.01) {
		receivedMessages = append(receivedMessages, message)
		if len(receivedMessages) >= 5 {
			cancel()
		}
	}

	assert.Len(t, receivedMessages, 5)
	for i, msg := range receivedMessages {
		assert.Equal(t, "kafka://orders", msg.URI)
		assert.Equal(t, float64(i), msg.Payload["order"])
	}
}

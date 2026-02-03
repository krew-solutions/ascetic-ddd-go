package outbox

import (
	"context"

	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/session"
)

type Subscriber func(*OutboxMessage) error

type Outbox interface {
	Publish(s session.DbSession, message *OutboxMessage) error
	Dispatch(subscriber Subscriber, consumerGroup string, uri string, workerID int, numWorkers int) (bool, error)
	Run(ctx context.Context, subscriber Subscriber, consumerGroup string, uri string, processID int, numProcesses int, concurrency int, pollInterval float64) error
	Messages(ctx context.Context, consumerGroup string, uri string, workerID int, numWorkers int, pollInterval float64) <-chan *OutboxMessage
	GetPosition(s session.DbSession, consumerGroup string, uri string) (int64, int64, error)
	SetPosition(s session.DbSession, consumerGroup string, uri string, transactionID int64, offset int64) error
	Setup() error
	Cleanup() error
}

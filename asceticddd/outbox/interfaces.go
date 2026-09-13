package outbox

import (
	"context"

	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/session"
)

type Subscriber func(*OutboxMessage) error

type Outbox interface {
	Publish(s session.Session, message *OutboxMessage) error
	Dispatch(ctx context.Context, subscriber Subscriber, consumerGroup string, uri string, workerId int, numWorkers int) (bool, error)
	Run(ctx context.Context, subscriber Subscriber, consumerGroup string, uri string, processId int, numProcesses int, concurrency int, pollInterval float64) error
	GetPosition(s session.Session, consumerGroup string, uri string) (int64, int64, error)
	SetPosition(s session.Session, consumerGroup string, uri string, transactionId int64, offset int64) error
	Setup(s session.Session) error
	Cleanup(s session.Session) error
}

package inbox

import (
	"context"

	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/session"
)

// Subscriber is a callback to process an inbox message.
// Receives a database session and the message to process.
type Subscriber func(session.Session, *InboxMessage) error

// Inbox defines the interface for the Inbox pattern.
//
// The Inbox pattern ensures:
// - Idempotency of incoming integration messages
// - Causal consistency by checking causal dependencies
// - Reliable message processing with at-least-once delivery
type Inbox interface {
	// Publish receives and stores an incoming message.
	//
	// The message is stored in the inbox table. If a message with the same
	// primary key (tenant_id, stream_type, stream_id, stream_position) already
	// exists, it is ignored (idempotency).
	//
	// ctx bounds the session the message is stored in.
	Publish(ctx context.Context, message *InboxMessage) error

	// Dispatch processes the next unprocessed message.
	//
	// Selects the first message with processed_position IS NULL,
	// ordered by received_position ASC, filtered by partition.
	//
	// Before processing, checks that all causal dependencies are satisfied
	// (exist in the inbox and have processed_position IS NOT NULL).
	//
	// If dependencies are not satisfied, skips to the next message.
	//
	// Args:
	//   ctx: Bounds the transaction; cancelling it aborts the transaction.
	//   subscriber: Callback to process the message.
	//   workerId: This worker's ID (0 to numWorkers-1).
	//   numWorkers: Total number of workers for partitioning.
	//
	// Returns true if a message was processed, false if no processable messages.
	Dispatch(ctx context.Context, subscriber Subscriber, workerId int, numWorkers int) (bool, error)

	// Run starts message processing with partitioned workers.
	//
	// Each goroutine processes its own partitions:
	//   effectiveId = processId * concurrency + localId
	//   effectiveTotal = numProcesses * concurrency
	//
	// Args:
	//   ctx: Context for cancellation and timeouts.
	//   subscriber: Callback to process each message.
	//   processId: This process's ID (0 to numProcesses-1).
	//   numProcesses: Total number of processes.
	//   concurrency: Number of goroutines within this process.
	//   pollInterval: Seconds to wait when no messages available.
	//
	// Returns when ctx is cancelled or a worker fails. A failing worker stops
	// the others between messages, and the first error is returned.
	//
	// Shutdown is cooperative: Dispatch is called with a context that carries
	// ctx's values but not its cancellation, so a message being processed is
	// finished and committed before the worker stops.
	Run(ctx context.Context, subscriber Subscriber, processId int, numProcesses int, concurrency int, pollInterval float64) error

	// Setup initializes the inbox (creates tables and sequences if needed).
	Setup(s session.Session) error

	// Cleanup releases resources.
	Cleanup(s session.Session) error
}

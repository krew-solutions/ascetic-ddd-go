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
	Publish(message *InboxMessage) error

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
	//   subscriber: Callback to process the message.
	//   workerID: This worker's ID (0 to numWorkers-1).
	//   numWorkers: Total number of workers for partitioning.
	//
	// Returns true if a message was processed, false if no processable messages.
	Dispatch(subscriber Subscriber, workerID int, numWorkers int) (bool, error)

	// Run starts message processing with partitioned workers.
	//
	// Each goroutine processes its own partitions:
	//   effectiveID = processID * concurrency + localID
	//   effectiveTotal = numProcesses * concurrency
	//
	// Args:
	//   ctx: Context for cancellation and timeouts.
	//   subscriber: Callback to process each message.
	//   processID: This process's ID (0 to numProcesses-1).
	//   numProcesses: Total number of processes.
	//   concurrency: Number of goroutines within this process.
	//   pollInterval: Seconds to wait when no messages available.
	Run(ctx context.Context, subscriber Subscriber, processID int, numProcesses int, concurrency int, pollInterval float64) error

	// Messages returns a channel for continuous message processing.
	//
	// Usage:
	//   for msg := range inbox.Messages(ctx, 0, 1, 1.0) {
	//       // Process message
	//       handleMessage(msg.Session, msg.Message)
	//   }
	//
	// Returns channel that yields SessionMessage pairs for each processable message.
	// The channel is automatically closed when context is cancelled.
	Messages(ctx context.Context, workerID int, numWorkers int, pollInterval float64) <-chan *SessionMessage

	// Setup initializes the inbox (creates tables and sequences if needed).
	Setup() error

	// Cleanup releases resources.
	Cleanup() error
}

// SessionMessage pairs a database session with an inbox message.
// Used by Messages() channel API.
type SessionMessage struct {
	Session session.Session
	Message *InboxMessage
}

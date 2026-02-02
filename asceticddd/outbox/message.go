package outbox

type OutboxMessage struct {
	URI           string
	Payload       map[string]any
	Metadata      map[string]any
	CreatedAt     *string
	Position      *int64
	TransactionID *int64
}

package outbox

type OutboxMessage struct {
	URI           string
	Payload       []byte
	Metadata      map[string]any
	CreatedAt     *string
	Position      *int64
	TransactionID *int64
}

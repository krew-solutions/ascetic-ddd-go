package outbox

type OutboxMessage struct {
	Uri           string
	Payload       []byte
	Metadata      map[string]any
	CreatedAt     *string
	Position      *int64
	TransactionId *int64
}

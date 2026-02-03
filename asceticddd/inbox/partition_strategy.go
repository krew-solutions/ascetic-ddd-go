package inbox

// PartitionKeyStrategy defines strategy for computing partition key SQL expression.
type PartitionKeyStrategy interface {
	// GetSQLExpression returns SQL expression for partition key.
	//
	// The expression is used with hashtext() for worker distribution:
	// WHERE hashtext(<expression>) % num_workers = worker_id
	GetSQLExpression() string
}

// StreamPartitionKeyStrategy partitions by stream identity.
//
// Use this strategy when messages have causal dependencies within a stream.
// All messages for the same (tenant_id, stream_type, stream_id) go to the
// same worker, preserving causal order.
//
// SQL expression: tenant_id || ':' || stream_type || ':' || stream_id::text
type StreamPartitionKeyStrategy struct{}

func (s *StreamPartitionKeyStrategy) GetSQLExpression() string {
	return "tenant_id || ':' || stream_type || ':' || stream_id::text"
}

// UriPartitionKeyStrategy partitions by URI.
//
// Use this strategy when ordering is based on topic/partition from the broker.
// All messages with the same URI go to the same worker.
//
// The URI may contain a partition key suffix (e.g., "kafka://orders/order-123")
// similar to Outbox pattern.
//
// SQL expression: uri
type UriPartitionKeyStrategy struct{}

func (s *UriPartitionKeyStrategy) GetSQLExpression() string {
	return "uri"
}

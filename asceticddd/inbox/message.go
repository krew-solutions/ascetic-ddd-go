package inbox

// InboxMessage represents an incoming integration message for the Inbox pattern.
//
// Attributes:
//
//	TenantID: Tenant identifier. Extracted from payload. Use "1" if tenant is not used.
//	StreamType: Type of the event stream. Can be one of:
//	    - bounded_context_name.aggregate_name extracted from payload
//	    - topic/channel name
//	StreamID: Identifier of the stream (stored as jsonb). Can be one of:
//	    - aggregate.id.internal_id extracted from payload for composite aggregate.id
//	    - aggregate.id extracted from payload for primitive aggregate.id
//	    - partition key of topic/channel
//	StreamPosition: Position in the stream (monotonically increasing). Can be one of:
//	    - aggregate.version from payload
//	    - position/offset of topic/channel
//	URI: Routing URI (e.g., 'kafka://orders', 'amqp://exchange/key'). Can be one of:
//	    - bus_type://topic_or_channel_name
//	    - bus_type://topic_or_channel_name/partition_key
//	Payload: Event payload data (must contain 'type' for deserialization).
//	Metadata: Optional event metadata (may contain event_id, causal_dependencies, etc.).
//	ReceivedPosition: Position when message was received (auto-assigned by DB).
//	ProcessedPosition: Position when message was processed (nil if not processed).
type InboxMessage struct {
	TenantID          string
	StreamType        string
	StreamID          map[string]any
	StreamPosition    int
	URI               string
	Payload           map[string]any
	Metadata          map[string]any
	ReceivedPosition  *int64
	ProcessedPosition *int64
}

// CausalDependencies returns causal dependencies from metadata.
//
// Returns list of dependency descriptors, each containing:
// - tenant_id, stream_type, stream_id, stream_position
func (m *InboxMessage) CausalDependencies() []map[string]any {
	if m.Metadata == nil {
		return []map[string]any{}
	}
	deps, ok := m.Metadata["causal_dependencies"].([]any)
	if !ok {
		return []map[string]any{}
	}
	result := make([]map[string]any, 0, len(deps))
	for _, dep := range deps {
		if depMap, ok := dep.(map[string]any); ok {
			result = append(result, depMap)
		}
	}
	return result
}

// EventID returns event_id from metadata if present.
func (m *InboxMessage) EventID() *string {
	if m.Metadata == nil {
		return nil
	}
	eventID, ok := m.Metadata["event_id"].(string)
	if !ok {
		return nil
	}
	return &eventID
}

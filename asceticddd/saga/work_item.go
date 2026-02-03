package saga

import "encoding/json"

// WorkItem is a unit of work to be processed by a specific activity type.
// Contains the queue address for routing and arguments for the activity.
// This structure is fully serializable for distributed saga orchestration.
type WorkItem struct {
	// QueueAddress is the address of the queue where this work item should be processed.
	QueueAddress string `json:"queueAddress"`
	// Arguments contains the arguments for the activity.
	Arguments WorkItemArguments `json:"arguments"`

	// Deprecated: activityType is kept for backward compatibility with tests.
	// In production, only QueueAddress is serialized and used for routing.
	activityType ActivityType
}

// NewWorkItem creates a new work item with the specified activity type and arguments.
// The queue address is extracted from the activity and stored for routing.
func NewWorkItem(activityType ActivityType, arguments WorkItemArguments) WorkItem {
	activity := activityType()
	return WorkItem{
		QueueAddress: activity.WorkItemQueueAddress(),
		Arguments:    arguments,
		activityType: activityType, // Kept for backward compatibility
	}
}

// ActivityType returns the type of activity that will process this work item.
// Deprecated: Use QueueAddress field for routing in distributed scenarios.
func (w WorkItem) ActivityType() ActivityType {
	return w.activityType
}

// MarshalJSON implements json.Marshaler interface.
// Serializes only the queue address and arguments (not the activityType function).
func (w WorkItem) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		QueueAddress string                 `json:"queueAddress"`
		Arguments    map[string]interface{} `json:"arguments"`
	}{
		QueueAddress: w.QueueAddress,
		Arguments:    w.Arguments,
	})
}

// UnmarshalJSON implements json.Unmarshaler interface.
// Deserializes the queue address and arguments.
// Note: activityType will be nil after deserialization - it must be resolved
// by the receiving service based on the queue address.
func (w *WorkItem) UnmarshalJSON(data []byte) error {
	var raw struct {
		QueueAddress string                 `json:"queueAddress"`
		Arguments    map[string]interface{} `json:"arguments"`
	}

	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	w.QueueAddress = raw.QueueAddress
	w.Arguments = raw.Arguments
	w.activityType = nil // Cannot deserialize function type

	return nil
}

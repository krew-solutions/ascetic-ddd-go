package saga

import "encoding/json"

// WorkLog is a record of completed work from an activity.
// Stores the compensation queue address and result, enabling compensation
// to be performed later if the saga needs to be rolled back.
// This structure is fully serializable for distributed saga orchestration.
type WorkLog struct {
	// CompensationQueueAddress is the address of the queue for compensating this work.
	CompensationQueueAddress string `json:"compensationQueueAddress"`
	// Result contains the result dictionary from the activity's work.
	Result WorkResult `json:"result"`

	// Deprecated: activityType is kept for backward compatibility with tests.
	// In production, only CompensationQueueAddress is serialized and used for routing.
	activityType ActivityType
}

// NewWorkLog creates a new work log with the specified activity and result.
// The compensation queue address is extracted from the activity and stored for routing.
func NewWorkLog(activity Activity, result WorkResult) WorkLog {
	return WorkLog{
		CompensationQueueAddress: activity.CompensationQueueAddress(),
		Result:                   result,
		activityType:             activity.ActivityType(), // Kept for backward compatibility
	}
}

// ActivityType returns the type of activity that performed this work.
// Deprecated: Use CompensationQueueAddress field for routing in distributed scenarios.
func (w WorkLog) ActivityType() ActivityType {
	return w.activityType
}

// MarshalJSON implements json.Marshaler interface.
// Serializes only the compensation queue address and result (not the activityType function).
func (w WorkLog) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		CompensationQueueAddress string                 `json:"compensationQueueAddress"`
		Result                   map[string]interface{} `json:"result"`
	}{
		CompensationQueueAddress: w.CompensationQueueAddress,
		Result:                   w.Result,
	})
}

// UnmarshalJSON implements json.Unmarshaler interface.
// Deserializes the compensation queue address and result.
// Note: activityType will be nil after deserialization - it must be resolved
// by the receiving service based on the queue address.
func (w *WorkLog) UnmarshalJSON(data []byte) error {
	var raw struct {
		CompensationQueueAddress string                 `json:"compensationQueueAddress"`
		Result                   map[string]interface{} `json:"result"`
	}

	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	w.CompensationQueueAddress = raw.CompensationQueueAddress
	w.Result = raw.Result
	w.activityType = nil // Cannot deserialize function type

	return nil
}

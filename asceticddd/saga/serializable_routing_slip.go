package saga

// SerializableRoutingSlip represents a serializable version of RoutingSlip.
// It can be marshaled to/from JSON or other formats for transmission over a message bus.
type SerializableRoutingSlip struct {
	CompletedWorkLogs []SerializableWorkLog  `json:"completedWorkLogs"`
	NextWorkItems     []SerializableWorkItem `json:"nextWorkItems"`
}

// SerializableWorkItem represents a serializable version of WorkItem.
type SerializableWorkItem struct {
	ActivityTypeName string            `json:"activityTypeName"`
	Arguments        WorkItemArguments `json:"arguments"`
}

// SerializableWorkLog represents a serializable version of WorkLog.
type SerializableWorkLog struct {
	ActivityTypeName string     `json:"activityTypeName"`
	Result           WorkResult `json:"result"`
}

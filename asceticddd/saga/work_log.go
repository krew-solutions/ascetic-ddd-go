package saga

// WorkLog is a record of completed work from an activity.
// Stores the activity type and its result, enabling compensation
// to be performed later if the saga needs to be rolled back.
type WorkLog struct {
	activityType ActivityType
	result       WorkResult
}

// NewWorkLog creates a new work log with the specified activity and result.
func NewWorkLog(activity Activity, result WorkResult) WorkLog {
	return WorkLog{
		activityType: activity.ActivityType(),
		result:       result,
	}
}

// Result returns the result dictionary from the activity's work.
func (w WorkLog) Result() WorkResult {
	return w.result
}

// ActivityType returns the type of activity that performed this work.
func (w WorkLog) ActivityType() ActivityType {
	return w.activityType
}

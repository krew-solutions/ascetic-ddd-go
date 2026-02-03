package saga

// WorkItem is a unit of work to be processed by a specific activity type.
// Contains the arguments needed by the activity.
type WorkItem struct {
	activityType ActivityType
	arguments    WorkItemArguments
}

// NewWorkItem creates a new work item with the specified activity type and arguments.
func NewWorkItem(activityType ActivityType, arguments WorkItemArguments) WorkItem {
	return WorkItem{
		activityType: activityType,
		arguments:    arguments,
	}
}

// ActivityType returns the type of activity that will process this work item.
func (w WorkItem) ActivityType() ActivityType {
	return w.activityType
}

// Arguments returns the arguments for the activity.
func (w WorkItem) Arguments() WorkItemArguments {
	return w.arguments
}

package saga

import "fmt"

// ToSerializable converts RoutingSlip to a serializable form using the provided resolver.
func (rs *RoutingSlip) ToSerializable(resolver ActivityTypeResolver) (*SerializableRoutingSlip, error) {
	srs := &SerializableRoutingSlip{
		CompletedWorkLogs: make([]SerializableWorkLog, len(rs.completedWorkLogs)),
		NextWorkItems:     make([]SerializableWorkItem, len(rs.nextWorkItems)),
	}

	// Serialize completed work logs
	for i, log := range rs.completedWorkLogs {
		name, err := resolver.GetName(log.ActivityType())
		if err != nil {
			return nil, fmt.Errorf("cannot serialize work log %d: %w", i, err)
		}
		srs.CompletedWorkLogs[i] = SerializableWorkLog{
			ActivityTypeName: name,
			Result:           log.Result(),
		}
	}

	// Serialize pending work items
	for i, item := range rs.nextWorkItems {
		name, err := resolver.GetName(item.ActivityType())
		if err != nil {
			return nil, fmt.Errorf("cannot serialize work item %d: %w", i, err)
		}
		srs.NextWorkItems[i] = SerializableWorkItem{
			ActivityTypeName: name,
			Arguments:        item.Arguments(),
		}
	}

	return srs, nil
}

// FromSerializable restores a RoutingSlip from its serializable form using the provided resolver.
func FromSerializable(srs *SerializableRoutingSlip, resolver ActivityTypeResolver) (*RoutingSlip, error) {
	rs := &RoutingSlip{
		completedWorkLogs: make([]WorkLog, 0, len(srs.CompletedWorkLogs)),
		nextWorkItems:     make([]WorkItem, 0, len(srs.NextWorkItems)),
	}

	// Restore completed work logs
	for i, slog := range srs.CompletedWorkLogs {
		activityType, err := resolver.Resolve(slog.ActivityTypeName)
		if err != nil {
			return nil, fmt.Errorf("cannot deserialize work log %d: %w", i, err)
		}

		activity := activityType()
		log := NewWorkLog(activity, slog.Result)
		rs.completedWorkLogs = append(rs.completedWorkLogs, log)
	}

	// Restore pending work items
	for i, sitem := range srs.NextWorkItems {
		activityType, err := resolver.Resolve(sitem.ActivityTypeName)
		if err != nil {
			return nil, fmt.Errorf("cannot deserialize work item %d: %w", i, err)
		}

		item := NewWorkItem(activityType, sitem.Arguments)
		rs.nextWorkItems = append(rs.nextWorkItems, item)
	}

	return rs, nil
}

package saga

import "context"

// ProcessNextForTest is a test helper that processes the next work item.
// If registry is nil, it will attempt to use the deprecated activityType field from WorkItem.
// This is only for testing - in production, use ActivityHost.
func ProcessNextForTest(ctx context.Context, slip *RoutingSlip, registry map[string]ActivityType) (bool, error) {
	// Auto-build registry from WorkItems if not provided (for backward compatibility)
	if registry == nil {
		registry = buildRegistryFromSlip(slip)
	}

	activityFactory := func(queueAddress string) Activity {
		factory, ok := registry[queueAddress]
		if !ok {
			return nil
		}
		return factory()
	}
	return slip.ProcessNextWith(ctx, activityFactory)
}

// UndoLastForTest is a test helper that undoes the last completed work.
// If registry is nil, it will attempt to use the deprecated activityType field from WorkLog.
// This is only for testing - in production, use ActivityHost.
func UndoLastForTest(ctx context.Context, slip *RoutingSlip, registry map[string]ActivityType) (bool, error) {
	// Auto-build registry from WorkLogs if not provided (for backward compatibility)
	if registry == nil {
		registry = buildRegistryFromSlip(slip)
	}

	activityFactory := func(queueAddress string) Activity {
		factory, ok := registry[queueAddress]
		if !ok {
			return nil
		}
		return factory()
	}
	return slip.UndoLastWith(ctx, activityFactory)
}

// buildRegistryFromSlip builds a registry from the routing slip's work items.
// This uses the deprecated activityType field for backward compatibility with tests.
func buildRegistryFromSlip(slip *RoutingSlip) map[string]ActivityType {
	registry := make(map[string]ActivityType)

	// Build from pending work items
	for _, item := range slip.nextWorkItems {
		if item.activityType != nil {
			activity := item.activityType()
			registry[activity.WorkItemQueueAddress()] = item.activityType
			registry[activity.CompensationQueueAddress()] = item.activityType
		}
	}

	// Build from completed work logs
	for _, log := range slip.completedWorkLogs {
		if log.activityType != nil {
			activity := log.activityType()
			registry[activity.WorkItemQueueAddress()] = log.activityType
			registry[activity.CompensationQueueAddress()] = log.activityType
		}
	}

	return registry
}

// BuildActivityRegistry is a test helper that builds a registry from activity types.
// The registry maps queue addresses (both work and compensation) to activity factories.
func BuildActivityRegistry(activityTypes ...ActivityType) map[string]ActivityType {
	registry := make(map[string]ActivityType)
	for _, factory := range activityTypes {
		activity := factory()
		registry[activity.WorkItemQueueAddress()] = factory
		registry[activity.CompensationQueueAddress()] = factory
	}
	return registry
}

package saga

import (
	"context"
	"testing"
)

// TestCompleteImplementationCanBeInstantiated verifies a complete Activity implementation works
func TestCompleteImplementationCanBeInstantiated(t *testing.T) {
	// Create an activity that implements all interface methods
	var activity *testActivity
	activity = &testActivity{
		doWorkFunc: func(ctx context.Context, workItem WorkItem) (*WorkLog, error) {
			result := NewWorkLog(activity, WorkResult{"done": true})
			return &result, nil
		},
		compensateFunc: func(ctx context.Context, workLog WorkLog, routingSlip *RoutingSlip) (bool, error) {
			return true, nil
		},
		workItemQueueAddressFunc: func() string {
			return "sb://./complete"
		},
		compensationQueueAddressFunc: func() string {
			return "sb://./completeCompensation"
		},
		activityTypeFunc: func() ActivityType {
			return func() Activity { return activity }
		},
	}

	// Verify it implements Activity interface
	var _ Activity = activity

	// Verify methods work
	if activity.WorkItemQueueAddress() != "sb://./complete" {
		t.Errorf("Expected work queue address 'sb://./complete', got '%s'", activity.WorkItemQueueAddress())
	}

	if activity.CompensationQueueAddress() != "sb://./completeCompensation" {
		t.Errorf("Expected compensation address 'sb://./completeCompensation', got '%s'", activity.CompensationQueueAddress())
	}
}

func TestDoWorkReceivesWorkItem(t *testing.T) {
	var receivedItem WorkItem

	var activity *testActivity
	activity = &testActivity{
		doWorkFunc: func(ctx context.Context, workItem WorkItem) (*WorkLog, error) {
			receivedItem = workItem
			result := NewWorkLog(activity, WorkResult{})
			return &result, nil
		},
		compensateFunc: func(ctx context.Context, workLog WorkLog, routingSlip *RoutingSlip) (bool, error) {
			return true, nil
		},
		workItemQueueAddressFunc: func() string {
			return "sb://./test"
		},
		compensationQueueAddressFunc: func() string {
			return "sb://./testCompensation"
		},
		activityTypeFunc: func() ActivityType {
			return func() Activity { return activity }
		},
	}

	activityType := func() Activity { return activity }
	workItem := NewWorkItem(activityType, WorkItemArguments{"key": "value"})

	ctx := context.Background()
	_, err := activity.DoWork(ctx, workItem)
	if err != nil {
		t.Fatalf("DoWork failed: %v", err)
	}

	if receivedItem.Arguments["key"] != "value" {
		t.Errorf("Expected arguments[key]='value', got '%v'", receivedItem.Arguments["key"])
	}
}

func TestCompensateReceivesWorkLogAndRoutingSlip(t *testing.T) {
	var receivedLog WorkLog
	var receivedSlip *RoutingSlip

	var activity *testActivity
	activity = &testActivity{
		doWorkFunc: func(ctx context.Context, workItem WorkItem) (*WorkLog, error) {
			result := NewWorkLog(activity, WorkResult{"id": 123})
			return &result, nil
		},
		compensateFunc: func(ctx context.Context, workLog WorkLog, routingSlip *RoutingSlip) (bool, error) {
			receivedLog = workLog
			receivedSlip = routingSlip
			return true, nil
		},
		workItemQueueAddressFunc: func() string {
			return "sb://./test"
		},
		compensationQueueAddressFunc: func() string {
			return "sb://./testCompensation"
		},
		activityTypeFunc: func() ActivityType {
			return func() Activity { return activity }
		},
	}

	activityType := func() Activity { return activity }
	workItem := NewWorkItem(activityType, WorkItemArguments{})

	ctx := context.Background()
	workLog, err := activity.DoWork(ctx, workItem)
	if err != nil {
		t.Fatalf("DoWork failed: %v", err)
	}

	routingSlip := NewRoutingSlip(nil)

	_, err = activity.Compensate(ctx, *workLog, routingSlip)
	if err != nil {
		t.Fatalf("Compensate failed: %v", err)
	}

	if receivedLog.Result["id"] != workLog.Result["id"] {
		t.Error("Expected to receive the same work log")
	}

	if receivedSlip != routingSlip {
		t.Error("Expected to receive the same routing slip")
	}
}

// testActivity is a test implementation of Activity interface
type testActivity struct {
	doWorkFunc                   func(context.Context, WorkItem) (*WorkLog, error)
	compensateFunc               func(context.Context, WorkLog, *RoutingSlip) (bool, error)
	workItemQueueAddressFunc     func() string
	compensationQueueAddressFunc func() string
	activityTypeFunc             func() ActivityType
}

func (a *testActivity) DoWork(ctx context.Context, workItem WorkItem) (*WorkLog, error) {
	if a.doWorkFunc != nil {
		return a.doWorkFunc(ctx, workItem)
	}
	result := NewWorkLog(a, WorkResult{})
	return &result, nil
}

func (a *testActivity) Compensate(ctx context.Context, workLog WorkLog, routingSlip *RoutingSlip) (bool, error) {
	if a.compensateFunc != nil {
		return a.compensateFunc(ctx, workLog, routingSlip)
	}
	return true, nil
}

func (a *testActivity) WorkItemQueueAddress() string {
	if a.workItemQueueAddressFunc != nil {
		return a.workItemQueueAddressFunc()
	}
	return "sb://./test"
}

func (a *testActivity) CompensationQueueAddress() string {
	if a.compensationQueueAddressFunc != nil {
		return a.compensationQueueAddressFunc()
	}
	return "sb://./testCompensation"
}

func (a *testActivity) ActivityType() ActivityType {
	if a.activityTypeFunc != nil {
		return a.activityTypeFunc()
	}
	return func() Activity { return a }
}

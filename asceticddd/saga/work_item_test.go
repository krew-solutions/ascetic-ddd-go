package saga

import (
	"context"
	"testing"
)

type stubActivity struct{}

func newStubActivity() Activity {
	return &stubActivity{}
}

func (s *stubActivity) DoWork(ctx context.Context, workItem WorkItem) (*WorkLog, error) {
	workLog := NewWorkLog(s, WorkResult{"id": 123})
	return &workLog, nil
}

func (s *stubActivity) Compensate(ctx context.Context, workLog WorkLog, routingSlip *RoutingSlip) (bool, error) {
	return true, nil
}

func (s *stubActivity) WorkItemQueueAddress() string {
	return "sb://./stub"
}

func (s *stubActivity) CompensationQueueAddress() string {
	return "sb://./stubCompensation"
}

func (s *stubActivity) ActivityType() ActivityType {
	return newStubActivity
}

func TestWorkItem_CreateWorkItem(t *testing.T) {
	args := WorkItemArguments{"vehicleType": "SUV"}
	workItem := NewWorkItem(newStubActivity, args)

	if workItem.Arguments["vehicleType"] != "SUV" {
		t.Errorf("Expected vehicleType 'SUV', got %v", workItem.Arguments["vehicleType"])
	}
}

func TestWorkItem_ArgumentsAreAccessible(t *testing.T) {
	args := WorkItemArguments{"a": 1, "b": 2, "c": 3}
	workItem := NewWorkItem(newStubActivity, args)

	if workItem.Arguments["a"] != 1 {
		t.Errorf("Expected a=1, got %v", workItem.Arguments["a"])
	}
	if workItem.Arguments["b"] != 2 {
		t.Errorf("Expected b=2, got %v", workItem.Arguments["b"])
	}
	if workItem.Arguments["c"] != 3 {
		t.Errorf("Expected c=3, got %v", workItem.Arguments["c"])
	}
}

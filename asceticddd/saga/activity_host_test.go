package saga

import (
	"context"
	"errors"
	"testing"
)

type activity1 struct {
	callCount       *int
	compensateCount *int
}

func newActivity1(callCount, compensateCount *int) ActivityType {
	return func() Activity {
		return &activity1{
			callCount:       callCount,
			compensateCount: compensateCount,
		}
	}
}

func (a *activity1) DoWork(ctx context.Context, workItem WorkItem) (*WorkLog, error) {
	*a.callCount++
	workLog := NewWorkLog(a, WorkResult{"id": *a.callCount})
	return &workLog, nil
}

func (a *activity1) Compensate(ctx context.Context, workLog WorkLog, routingSlip *RoutingSlip) (bool, error) {
	*a.compensateCount++
	return true, nil
}

func (a *activity1) WorkItemQueueAddress() string {
	return "sb://./activity1"
}

func (a *activity1) CompensationQueueAddress() string {
	return "sb://./activity1Compensation"
}

func (a *activity1) ActivityType() ActivityType {
	return newActivity1(a.callCount, a.compensateCount)
}

type activity2 struct {
	callCount       *int
	compensateCount *int
}

func newActivity2(callCount, compensateCount *int) ActivityType {
	return func() Activity {
		return &activity2{
			callCount:       callCount,
			compensateCount: compensateCount,
		}
	}
}

func (a *activity2) DoWork(ctx context.Context, workItem WorkItem) (*WorkLog, error) {
	*a.callCount++
	workLog := NewWorkLog(a, WorkResult{"id": *a.callCount})
	return &workLog, nil
}

func (a *activity2) Compensate(ctx context.Context, workLog WorkLog, routingSlip *RoutingSlip) (bool, error) {
	*a.compensateCount++
	return true, nil
}

func (a *activity2) WorkItemQueueAddress() string {
	return "sb://./activity2"
}

func (a *activity2) CompensationQueueAddress() string {
	return "sb://./activity2Compensation"
}

func (a *activity2) ActivityType() ActivityType {
	return newActivity2(a.callCount, a.compensateCount)
}

type failingActivityHost struct{}

func newFailingActivityHost() Activity {
	return &failingActivityHost{}
}

func (f *failingActivityHost) DoWork(ctx context.Context, workItem WorkItem) (*WorkLog, error) {
	return nil, errors.New("intentional failure")
}

func (f *failingActivityHost) Compensate(ctx context.Context, workLog WorkLog, routingSlip *RoutingSlip) (bool, error) {
	return true, nil
}

func (f *failingActivityHost) WorkItemQueueAddress() string {
	return "sb://./failing"
}

func (f *failingActivityHost) CompensationQueueAddress() string {
	return "sb://./failingCompensation"
}

func (f *failingActivityHost) ActivityType() ActivityType {
	return newFailingActivityHost
}

func TestActivityHost_HandleWorkItemMessage(t *testing.T) {
	callCount := 0
	compensateCount := 0
	activityType := newActivity1(&callCount, &compensateCount)

	send := func(ctx context.Context, uri string, routingSlip *RoutingSlip) error {
		return nil
	}

	host := NewActivityHost(send)
	host.Register(activityType)
	slip := NewRoutingSlip([]WorkItem{NewWorkItem(activityType, WorkItemArguments{})})
	ctx := context.Background()

	err := host.HandleMessage(ctx, "sb://./activity1", slip)
	if err != nil {
		t.Fatalf("HandleMessage returned error: %v", err)
	}

	if callCount != 1 {
		t.Errorf("Expected call count 1, got %d", callCount)
	}
}

func TestActivityHost_HandleCompensationMessage(t *testing.T) {
	callCount := 0
	compensateCount := 0
	activityType := newActivity1(&callCount, &compensateCount)

	send := func(ctx context.Context, uri string, routingSlip *RoutingSlip) error {
		return nil
	}

	host := NewActivityHost(send)
	host.Register(activityType)
	slip := NewRoutingSlip([]WorkItem{NewWorkItem(activityType, WorkItemArguments{})})
	ctx := context.Background()
	ProcessNextForTest(ctx, slip, nil)

	err := host.HandleMessage(ctx, "sb://./activity1Compensation", slip)
	if err != nil {
		t.Fatalf("HandleMessage returned error: %v", err)
	}

	if compensateCount != 1 {
		t.Errorf("Expected compensate count 1, got %d", compensateCount)
	}
}

func TestActivityHost_RejectUnknownMessage(t *testing.T) {
	callCount := 0
	compensateCount := 0
	activityType := newActivity1(&callCount, &compensateCount)

	send := func(ctx context.Context, uri string, routingSlip *RoutingSlip) error {
		return nil
	}

	host := NewActivityHost(send)
	host.Register(activityType)
	slip := NewRoutingSlip([]WorkItem{NewWorkItem(activityType, WorkItemArguments{})})
	ctx := context.Background()

	err := host.HandleMessage(ctx, "sb://./unknown", slip)
	if err == nil {
		t.Error("Expected error for unknown URI")
	}
}

func TestActivityHost_MultipleActivities(t *testing.T) {
	callCount1 := 0
	compensateCount1 := 0
	activityType1 := newActivity1(&callCount1, &compensateCount1)

	callCount2 := 0
	compensateCount2 := 0
	activityType2 := newActivity2(&callCount2, &compensateCount2)

	send := func(ctx context.Context, uri string, routingSlip *RoutingSlip) error {
		return nil
	}

	host := NewActivityHost(send)
	host.Register(activityType1)
	host.Register(activityType2)

	ctx := context.Background()

	// Handle activity1
	slip1 := NewRoutingSlip([]WorkItem{NewWorkItem(activityType1, WorkItemArguments{})})
	err := host.HandleMessage(ctx, "sb://./activity1", slip1)
	if err != nil {
		t.Fatalf("HandleMessage for activity1 returned error: %v", err)
	}

	// Handle activity2
	slip2 := NewRoutingSlip([]WorkItem{NewWorkItem(activityType2, WorkItemArguments{})})
	err = host.HandleMessage(ctx, "sb://./activity2", slip2)
	if err != nil {
		t.Fatalf("HandleMessage for activity2 returned error: %v", err)
	}

	if callCount1 != 1 {
		t.Errorf("Expected activity1 call count 1, got %d", callCount1)
	}
	if callCount2 != 1 {
		t.Errorf("Expected activity2 call count 1, got %d", callCount2)
	}
}

func TestActivityHost_ForwardSuccessContinuesForward(t *testing.T) {
	callCount1 := 0
	compensateCount1 := 0
	activityType1 := newActivity1(&callCount1, &compensateCount1)

	callCount2 := 0
	compensateCount2 := 0
	activityType2 := newActivity2(&callCount2, &compensateCount2)

	sentMessages := []struct {
		uri  string
		slip *RoutingSlip
	}{}

	send := func(ctx context.Context, uri string, routingSlip *RoutingSlip) error {
		sentMessages = append(sentMessages, struct {
			uri  string
			slip *RoutingSlip
		}{uri, routingSlip})
		return nil
	}

	host := NewActivityHost(send)
	host.Register(activityType1)
	slip := NewRoutingSlip([]WorkItem{
		NewWorkItem(activityType1, WorkItemArguments{}),
		NewWorkItem(activityType2, WorkItemArguments{}),
	})
	ctx := context.Background()

	host.HandleMessage(ctx, "sb://./activity1", slip)

	if len(sentMessages) != 1 {
		t.Fatalf("Expected 1 sent message, got %d", len(sentMessages))
	}
	if sentMessages[0].uri != "sb://./activity2" {
		t.Errorf("Expected message to activity2, got %s", sentMessages[0].uri)
	}
}

func TestActivityHost_ForwardFailureStartsCompensation(t *testing.T) {
	callCount1 := 0
	compensateCount1 := 0
	activityType1 := newActivity1(&callCount1, &compensateCount1)

	sentMessages := []struct {
		uri  string
		slip *RoutingSlip
	}{}

	send := func(ctx context.Context, uri string, routingSlip *RoutingSlip) error {
		sentMessages = append(sentMessages, struct {
			uri  string
			slip *RoutingSlip
		}{uri, routingSlip})
		return nil
	}

	host := NewActivityHost(send)
	host.Register(newFailingActivityHost)
	slip := NewRoutingSlip([]WorkItem{
		NewWorkItem(activityType1, WorkItemArguments{}),
		NewWorkItem(newFailingActivityHost, WorkItemArguments{}),
	})
	ctx := context.Background()
	ProcessNextForTest(ctx, slip, nil) // Complete Activity1

	host.HandleMessage(ctx, "sb://./failing", slip)

	if len(sentMessages) != 1 {
		t.Fatalf("Expected 1 sent message, got %d", len(sentMessages))
	}
	if sentMessages[0].uri != "sb://./activity1Compensation" {
		t.Errorf("Expected message to activity1Compensation, got %s", sentMessages[0].uri)
	}
}

func TestActivityHost_BackwardContinuesBackward(t *testing.T) {
	callCount1 := 0
	compensateCount1 := 0
	activityType1 := newActivity1(&callCount1, &compensateCount1)

	callCount2 := 0
	compensateCount2 := 0
	activityType2 := newActivity2(&callCount2, &compensateCount2)

	sentMessages := []struct {
		uri  string
		slip *RoutingSlip
	}{}

	send := func(ctx context.Context, uri string, routingSlip *RoutingSlip) error {
		sentMessages = append(sentMessages, struct {
			uri  string
			slip *RoutingSlip
		}{uri, routingSlip})
		return nil
	}

	host := NewActivityHost(send)
	host.Register(activityType2)
	slip := NewRoutingSlip([]WorkItem{
		NewWorkItem(activityType1, WorkItemArguments{}),
		NewWorkItem(activityType2, WorkItemArguments{}),
	})
	ctx := context.Background()
	ProcessNextForTest(ctx, slip, nil)
	ProcessNextForTest(ctx, slip, nil)

	host.HandleMessage(ctx, "sb://./activity2Compensation", slip)

	if len(sentMessages) != 1 {
		t.Fatalf("Expected 1 sent message, got %d", len(sentMessages))
	}
	if sentMessages[0].uri != "sb://./activity1Compensation" {
		t.Errorf("Expected message to activity1Compensation, got %s", sentMessages[0].uri)
	}
}

func TestActivityHost_Queues(t *testing.T) {
	callCount1 := 0
	compensateCount1 := 0
	activityType1 := newActivity1(&callCount1, &compensateCount1)

	callCount2 := 0
	compensateCount2 := 0
	activityType2 := newActivity2(&callCount2, &compensateCount2)

	host := NewActivityHost(nil)
	host.Register(activityType1)
	host.Register(activityType2)

	queues := host.Queues()
	if len(queues) != 4 {
		t.Errorf("Expected 4 queues, got %d", len(queues))
	}

	expected := map[string]bool{
		"sb://./activity1":             true,
		"sb://./activity1Compensation": true,
		"sb://./activity2":             true,
		"sb://./activity2Compensation": true,
	}
	for _, q := range queues {
		if !expected[q] {
			t.Errorf("Unexpected queue: %s", q)
		}
	}
}

func TestActivityHost_DistributedSagaSuccess(t *testing.T) {
	callCount1 := 0
	compensateCount1 := 0
	activityType1 := newActivity1(&callCount1, &compensateCount1)

	callCount2 := 0
	compensateCount2 := 0
	activityType2 := newActivity2(&callCount2, &compensateCount2)

	type message struct {
		uri  string
		slip *RoutingSlip
	}
	messages := []message{}

	send := func(ctx context.Context, uri string, routingSlip *RoutingSlip) error {
		messages = append(messages, message{uri, routingSlip})
		return nil
	}

	host := NewActivityHost(send)
	host.Register(activityType1)
	host.Register(activityType2)

	slip := NewRoutingSlip([]WorkItem{
		NewWorkItem(activityType1, WorkItemArguments{}),
		NewWorkItem(activityType2, WorkItemArguments{}),
	})
	ctx := context.Background()

	// Start saga
	send(ctx, slip.ProgressUri(), slip)

	// Process all messages
	for len(messages) > 0 {
		msg := messages[0]
		messages = messages[1:]
		host.HandleMessage(ctx, msg.uri, msg.slip)
	}

	if !slip.IsCompleted() {
		t.Error("Expected routing slip to be completed")
	}
	if callCount1 != 1 {
		t.Errorf("Expected activity1 call count 1, got %d", callCount1)
	}
	if callCount2 != 1 {
		t.Errorf("Expected activity2 call count 1, got %d", callCount2)
	}
}

func TestActivityHost_DistributedSagaWithCompensation(t *testing.T) {
	callCount1 := 0
	compensateCount1 := 0
	activityType1 := newActivity1(&callCount1, &compensateCount1)

	callCount2 := 0
	compensateCount2 := 0
	activityType2 := newActivity2(&callCount2, &compensateCount2)

	type message struct {
		uri  string
		slip *RoutingSlip
	}
	messages := []message{}

	send := func(ctx context.Context, uri string, routingSlip *RoutingSlip) error {
		messages = append(messages, message{uri, routingSlip})
		return nil
	}

	host := NewActivityHost(send)
	host.Register(activityType1)
	host.Register(activityType2)
	host.Register(newFailingActivityHost)

	slip := NewRoutingSlip([]WorkItem{
		NewWorkItem(activityType1, WorkItemArguments{}),
		NewWorkItem(activityType2, WorkItemArguments{}),
		NewWorkItem(newFailingActivityHost, WorkItemArguments{}),
	})
	ctx := context.Background()

	// Start saga
	send(ctx, slip.ProgressUri(), slip)

	// Process all messages
	for len(messages) > 0 {
		msg := messages[0]
		messages = messages[1:]
		host.HandleMessage(ctx, msg.uri, msg.slip)
	}

	if slip.IsInProgress() {
		t.Error("Expected routing slip to not be in progress after compensation")
	}
	if callCount1 != 1 {
		t.Errorf("Expected activity1 call count 1, got %d", callCount1)
	}
	if callCount2 != 1 {
		t.Errorf("Expected activity2 call count 1, got %d", callCount2)
	}
	if compensateCount1 != 1 {
		t.Errorf("Expected activity1 compensate count 1, got %d", compensateCount1)
	}
	if compensateCount2 != 1 {
		t.Errorf("Expected activity2 compensate count 1, got %d", compensateCount2)
	}
}

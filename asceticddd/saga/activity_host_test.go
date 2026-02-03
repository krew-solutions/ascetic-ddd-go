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

func TestActivityHost_AcceptWorkItemMessage(t *testing.T) {
	callCount := 0
	compensateCount := 0
	activityType := newActivity1(&callCount, &compensateCount)

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

	host := NewActivityHost(activityType, send)
	slip := NewRoutingSlip([]WorkItem{NewWorkItem(activityType, WorkItemArguments{})})
	ctx := context.Background()

	result, err := host.AcceptMessage(ctx, "sb://./activity1", slip)
	if err != nil {
		t.Fatalf("AcceptMessage returned error: %v", err)
	}

	if !result {
		t.Error("Expected AcceptMessage to return true")
	}
}

func TestActivityHost_AcceptCompensationMessage(t *testing.T) {
	callCount := 0
	compensateCount := 0
	activityType := newActivity1(&callCount, &compensateCount)

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

	host := NewActivityHost(activityType, send)
	slip := NewRoutingSlip([]WorkItem{NewWorkItem(activityType, WorkItemArguments{})})
	ctx := context.Background()
	slip.ProcessNext(ctx)

	result, err := host.AcceptMessage(ctx, "sb://./activity1Compensation", slip)
	if err != nil {
		t.Fatalf("AcceptMessage returned error: %v", err)
	}

	if !result {
		t.Error("Expected AcceptMessage to return true")
	}
}

func TestActivityHost_RejectUnknownMessage(t *testing.T) {
	callCount := 0
	compensateCount := 0
	activityType := newActivity1(&callCount, &compensateCount)

	send := func(ctx context.Context, uri string, routingSlip *RoutingSlip) error {
		return nil
	}

	host := NewActivityHost(activityType, send)
	slip := NewRoutingSlip([]WorkItem{NewWorkItem(activityType, WorkItemArguments{})})
	ctx := context.Background()

	result, err := host.AcceptMessage(ctx, "sb://./unknown", slip)
	if err != nil {
		t.Fatalf("AcceptMessage returned error: %v", err)
	}

	if result {
		t.Error("Expected AcceptMessage to return false for unknown URI")
	}
}

func TestActivityHost_RejectOtherActivityMessage(t *testing.T) {
	callCount1 := 0
	compensateCount1 := 0
	activityType1 := newActivity1(&callCount1, &compensateCount1)

	callCount2 := 0
	compensateCount2 := 0
	activityType2 := newActivity2(&callCount2, &compensateCount2)

	send := func(ctx context.Context, uri string, routingSlip *RoutingSlip) error {
		return nil
	}

	host := NewActivityHost(activityType1, send)
	slip := NewRoutingSlip([]WorkItem{NewWorkItem(activityType2, WorkItemArguments{})})
	ctx := context.Background()

	result, err := host.AcceptMessage(ctx, "sb://./activity2", slip)
	if err != nil {
		t.Fatalf("AcceptMessage returned error: %v", err)
	}

	if result {
		t.Error("Expected AcceptMessage to return false for other activity")
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

	host := NewActivityHost(activityType1, send)
	slip := NewRoutingSlip([]WorkItem{
		NewWorkItem(activityType1, WorkItemArguments{}),
		NewWorkItem(activityType2, WorkItemArguments{}),
	})
	ctx := context.Background()

	host.ProcessForwardMessage(ctx, slip)

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

	host := NewActivityHost(newFailingActivityHost, send)
	slip := NewRoutingSlip([]WorkItem{
		NewWorkItem(activityType1, WorkItemArguments{}),
		NewWorkItem(newFailingActivityHost, WorkItemArguments{}),
	})
	ctx := context.Background()
	slip.ProcessNext(ctx) // Complete Activity1

	host.ProcessForwardMessage(ctx, slip)

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

	host := NewActivityHost(activityType2, send)
	slip := NewRoutingSlip([]WorkItem{
		NewWorkItem(activityType1, WorkItemArguments{}),
		NewWorkItem(activityType2, WorkItemArguments{}),
	})
	ctx := context.Background()
	slip.ProcessNext(ctx)
	slip.ProcessNext(ctx)

	host.ProcessBackwardMessage(ctx, slip)

	if len(sentMessages) != 1 {
		t.Fatalf("Expected 1 sent message, got %d", len(sentMessages))
	}
	if sentMessages[0].uri != "sb://./activity1Compensation" {
		t.Errorf("Expected message to activity1Compensation, got %s", sentMessages[0].uri)
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

	host1 := NewActivityHost(activityType1, send)
	host2 := NewActivityHost(activityType2, send)
	hosts := []*ActivityHost{host1, host2}

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

		for _, host := range hosts {
			accepted, _ := host.AcceptMessage(ctx, msg.uri, msg.slip)
			if accepted {
				break
			}
		}
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

	host1 := NewActivityHost(activityType1, send)
	host2 := NewActivityHost(activityType2, send)
	hostFail := NewActivityHost(newFailingActivityHost, send)
	hosts := []*ActivityHost{host1, host2, hostFail}

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

		for _, host := range hosts {
			accepted, _ := host.AcceptMessage(ctx, msg.uri, msg.slip)
			if accepted {
				break
			}
		}
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

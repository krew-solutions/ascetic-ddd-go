package saga

import "testing"

func TestWorkLog_CreateWorkLog(t *testing.T) {
	activity := &stubActivity{}
	result := WorkResult{"reservationId": 12345}
	workLog := NewWorkLog(activity, result)

	if workLog.Result()["reservationId"] != 12345 {
		t.Errorf("Expected reservationId 12345, got %v", workLog.Result()["reservationId"])
	}
}

func TestWorkLog_ResultIsAccessible(t *testing.T) {
	activity := &stubActivity{}
	result := WorkResult{"key": "value", "count": 42}
	workLog := NewWorkLog(activity, result)

	if workLog.Result()["key"] != "value" {
		t.Errorf("Expected key 'value', got %v", workLog.Result()["key"])
	}
	if workLog.Result()["count"] != 42 {
		t.Errorf("Expected count 42, got %v", workLog.Result()["count"])
	}
}

func TestWorkLog_ActivityTypeIsFunction(t *testing.T) {
	activity1 := &stubActivity{}
	result := WorkResult{}

	workLog := NewWorkLog(activity1, result)

	// Create instance from stored type
	activity2 := workLog.ActivityType()()

	if _, ok := activity2.(*stubActivity); !ok {
		t.Error("Expected activity type to create stubActivity instance")
	}
}

package saga

import (
	"context"
	"testing"
)

// Test activity implementing NamedActivity
type testNamedActivity struct{}

func newTestNamedActivity() Activity {
	return &testNamedActivity{}
}

func (t *testNamedActivity) DoWork(ctx context.Context, workItem WorkItem) (*WorkLog, error) {
	workLog := NewWorkLog(t, WorkResult{})
	return &workLog, nil
}

func (t *testNamedActivity) Compensate(ctx context.Context, workLog WorkLog, routingSlip *RoutingSlip) (bool, error) {
	return true, nil
}

func (t *testNamedActivity) WorkItemQueueAddress() string {
	return "sb://./test"
}

func (t *testNamedActivity) CompensationQueueAddress() string {
	return "sb://./testCompensation"
}

func (t *testNamedActivity) ActivityType() ActivityType {
	return newTestNamedActivity
}

func (t *testNamedActivity) TypeName() string {
	return "TestNamedActivity"
}

// Another test activity
type anotherNamedActivity struct{}

func newAnotherNamedActivity() Activity {
	return &anotherNamedActivity{}
}

func (a *anotherNamedActivity) DoWork(ctx context.Context, workItem WorkItem) (*WorkLog, error) {
	workLog := NewWorkLog(a, WorkResult{})
	return &workLog, nil
}

func (a *anotherNamedActivity) Compensate(ctx context.Context, workLog WorkLog, routingSlip *RoutingSlip) (bool, error) {
	return true, nil
}

func (a *anotherNamedActivity) WorkItemQueueAddress() string {
	return "sb://./another"
}

func (a *anotherNamedActivity) CompensationQueueAddress() string {
	return "sb://./anotherCompensation"
}

func (a *anotherNamedActivity) ActivityType() ActivityType {
	return newAnotherNamedActivity
}

func (a *anotherNamedActivity) TypeName() string {
	return "AnotherNamedActivity"
}

func TestMapBasedResolver_RegisterAndResolve(t *testing.T) {
	resolver := NewMapBasedResolver()
	activityType := newTestNamedActivity

	resolver.Register("TestNamedActivity", activityType)

	resolved, err := resolver.Resolve("TestNamedActivity")
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}

	if resolved == nil {
		t.Fatal("Resolved activity type is nil")
	}

	// Create instance and verify it's the correct type
	activity := resolved()
	if _, ok := activity.(*testNamedActivity); !ok {
		t.Errorf("Expected *testNamedActivity, got %T", activity)
	}
}

func TestMapBasedResolver_ResolveUnregisteredType(t *testing.T) {
	resolver := NewMapBasedResolver()

	_, err := resolver.Resolve("UnregisteredActivity")
	if err == nil {
		t.Error("Expected error when resolving unregistered activity type")
	}
}

func TestMapBasedResolver_GetName(t *testing.T) {
	resolver := NewMapBasedResolver()
	activityType := newTestNamedActivity

	resolver.Register("TestNamedActivity", activityType)

	name, err := resolver.GetName(activityType)
	if err != nil {
		t.Fatalf("GetName failed: %v", err)
	}

	if name != "TestNamedActivity" {
		t.Errorf("Expected name 'TestNamedActivity', got '%s'", name)
	}
}

func TestMapBasedResolver_GetNameUnregisteredType(t *testing.T) {
	resolver := NewMapBasedResolver()
	activityType := newTestNamedActivity

	// Try to get name without registering
	name, err := resolver.GetName(activityType)

	// Should still work because of NamedActivity fallback
	if err != nil {
		t.Fatalf("GetName failed: %v", err)
	}

	if name != "TestNamedActivity" {
		t.Errorf("Expected name 'TestNamedActivity' from NamedActivity fallback, got '%s'", name)
	}
}

func TestMapBasedResolver_MultipleRegistrations(t *testing.T) {
	resolver := NewMapBasedResolver()

	resolver.Register("TestNamedActivity", newTestNamedActivity)
	resolver.Register("AnotherNamedActivity", newAnotherNamedActivity)

	// Resolve first
	resolved1, err := resolver.Resolve("TestNamedActivity")
	if err != nil {
		t.Fatalf("Resolve TestNamedActivity failed: %v", err)
	}

	activity1 := resolved1()
	if _, ok := activity1.(*testNamedActivity); !ok {
		t.Errorf("Expected *testNamedActivity, got %T", activity1)
	}

	// Resolve second
	resolved2, err := resolver.Resolve("AnotherNamedActivity")
	if err != nil {
		t.Fatalf("Resolve AnotherNamedActivity failed: %v", err)
	}

	activity2 := resolved2()
	if _, ok := activity2.(*anotherNamedActivity); !ok {
		t.Errorf("Expected *anotherNamedActivity, got %T", activity2)
	}
}

func TestMapBasedResolver_IsolatedInstances(t *testing.T) {
	// Create two separate resolvers to test isolation
	resolver1 := NewMapBasedResolver()
	resolver2 := NewMapBasedResolver()

	resolver1.Register("TestNamedActivity", newTestNamedActivity)

	// resolver1 should resolve successfully
	_, err := resolver1.Resolve("TestNamedActivity")
	if err != nil {
		t.Errorf("resolver1 failed to resolve: %v", err)
	}

	// resolver2 should NOT resolve (not registered)
	_, err = resolver2.Resolve("TestNamedActivity")
	if err == nil {
		t.Error("resolver2 should not be able to resolve TestNamedActivity")
	}
}

func TestMapBasedResolver_RegisterOverwrite(t *testing.T) {
	resolver := NewMapBasedResolver()

	resolver.Register("TestActivity", newTestNamedActivity)
	resolver.Register("TestActivity", newAnotherNamedActivity) // Overwrite

	resolved, err := resolver.Resolve("TestActivity")
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}

	// Should get the second registration
	activity := resolved()
	if _, ok := activity.(*anotherNamedActivity); !ok {
		t.Errorf("Expected *anotherNamedActivity after overwrite, got %T", activity)
	}
}

func TestNamedActivity_Interface(t *testing.T) {
	activity := newTestNamedActivity()

	// Verify it implements NamedActivity
	named, ok := activity.(NamedActivity)
	if !ok {
		t.Fatal("testNamedActivity does not implement NamedActivity interface")
	}

	if named.TypeName() != "TestNamedActivity" {
		t.Errorf("Expected TypeName 'TestNamedActivity', got '%s'", named.TypeName())
	}
}

package saga

import "testing"

func TestWorkItemArguments_CreateEmpty(t *testing.T) {
	args := WorkItemArguments{}
	if len(args) != 0 {
		t.Errorf("Expected empty WorkItemArguments, got length %d", len(args))
	}
}

func TestWorkItemArguments_CreateWithData(t *testing.T) {
	args := WorkItemArguments{"vehicleType": "Compact", "days": 5}
	if args["vehicleType"] != "Compact" {
		t.Errorf("Expected vehicleType 'Compact', got %v", args["vehicleType"])
	}
	if args["days"] != 5 {
		t.Errorf("Expected days 5, got %v", args["days"])
	}
}

func TestWorkItemArguments_SetAndGetItems(t *testing.T) {
	args := WorkItemArguments{}
	args["destination"] = "Paris"

	if args["destination"] != "Paris" {
		t.Errorf("Expected destination 'Paris', got %v", args["destination"])
	}

	if _, exists := args["destination"]; !exists {
		t.Error("Expected destination key to exist")
	}
}

func TestWorkItemArguments_MissingKey(t *testing.T) {
	args := WorkItemArguments{"a": 1}
	val, exists := args["missing"]

	if exists {
		t.Error("Expected missing key to not exist")
	}
	if val != nil {
		t.Errorf("Expected nil for missing key, got %v", val)
	}
}

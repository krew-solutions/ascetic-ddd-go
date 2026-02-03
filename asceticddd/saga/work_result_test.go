package saga

import "testing"

func TestWorkResult_CreateEmpty(t *testing.T) {
	result := WorkResult{}
	if len(result) != 0 {
		t.Errorf("Expected empty WorkResult, got length %d", len(result))
	}
}

func TestWorkResult_CreateWithData(t *testing.T) {
	result := WorkResult{"reservationId": 12345, "status": "confirmed"}
	if result["reservationId"] != 12345 {
		t.Errorf("Expected reservationId 12345, got %v", result["reservationId"])
	}
	if result["status"] != "confirmed" {
		t.Errorf("Expected status 'confirmed', got %v", result["status"])
	}
}

func TestWorkResult_SetAndGetItems(t *testing.T) {
	result := WorkResult{}
	result["key"] = "value"

	if result["key"] != "value" {
		t.Errorf("Expected key 'value', got %v", result["key"])
	}

	if _, exists := result["key"]; !exists {
		t.Error("Expected key to exist")
	}
}

func TestWorkResult_UpdateFromMap(t *testing.T) {
	result := WorkResult{"a": 1}

	// Merge another map
	updates := map[string]any{"b": 2, "c": 3}
	for k, v := range updates {
		result[k] = v
	}

	if result["a"] != 1 {
		t.Errorf("Expected a=1, got %v", result["a"])
	}
	if result["b"] != 2 {
		t.Errorf("Expected b=2, got %v", result["b"])
	}
	if result["c"] != 3 {
		t.Errorf("Expected c=3, got %v", result["c"])
	}
}

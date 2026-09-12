package inbox

import (
	"encoding/json"
	"testing"
)

// jsonPayload encodes a test payload as the bytes the inbox stores.
func jsonPayload(v map[string]any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}

// decodePayload reads a stored payload back as JSON.
func decodePayload(t *testing.T, payload []byte) map[string]any {
	t.Helper()
	var v map[string]any
	if err := json.Unmarshal(payload, &v); err != nil {
		t.Fatalf("payload is not JSON: %v", err)
	}
	return v
}

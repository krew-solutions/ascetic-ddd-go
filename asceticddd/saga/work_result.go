package saga

// WorkResult contains results from an activity's work execution.
// Stores key-value pairs representing the outcome of DoWork(),
// such as reservation IDs, confirmation numbers, etc.
type WorkResult map[string]any

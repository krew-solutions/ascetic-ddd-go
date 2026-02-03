# RoutingSlip Serialization Guide

This guide explains how to serialize and deserialize `RoutingSlip` instances for transmission over a message bus.

## Overview

The serialization mechanism allows `RoutingSlip` to be transmitted between services in a distributed SAGA implementation. Since `ActivityType` is a function pointer (not serializable), we use an **Interface-based approach** with dependency injection via `ActivityTypeResolver`.

## Key Components

### 1. ActivityTypeResolver

The `ActivityTypeResolver` interface provides bidirectional mapping between activity type names and `ActivityType` functions:

```go
type ActivityTypeResolver interface {
    Resolve(typeName string) (ActivityType, error)
    GetName(activityType ActivityType) (string, error)
}
```

### 2. MapBasedResolver

`MapBasedResolver` is the default implementation that uses in-memory maps:

```go
resolver := saga.NewMapBasedResolver()
resolver.Register("ReserveCarActivity", NewReserveCarActivity)
resolver.Register("ReserveHotelActivity", NewReserveHotelActivity)
```

### 3. NamedActivity Interface (Optional)

Activities can optionally implement `NamedActivity` to provide their type name:

```go
type NamedActivity interface {
    Activity
    TypeName() string
}

func (a *ReserveCarActivity) TypeName() string {
    return "ReserveCarActivity"
}
```

**Benefits:**
- Enables fallback serialization even without explicit registration
- Deserialization still requires registration for security
- Makes activity types self-documenting

## Basic Usage

### Step 1: Create a Resolver and Register Activities

```go
// Create resolver
resolver := saga.NewMapBasedResolver()

// Register all activity types
resolver.Register("ReserveCarActivity", NewReserveCarActivity)
resolver.Register("ReserveHotelActivity", NewReserveHotelActivity)
resolver.Register("ReserveFlightActivity", NewReserveFlightActivity)
```

### Step 2: Serialize RoutingSlip

```go
// Create routing slip
routingSlip := saga.NewRoutingSlip([]saga.WorkItem{
    saga.NewWorkItem(NewReserveCarActivity, saga.WorkItemArguments{
        "vehicleType": "SUV",
    }),
    saga.NewWorkItem(NewReserveHotelActivity, saga.WorkItemArguments{
        "roomType": "Suite",
    }),
})

// Process some work
ctx := context.Background()
routingSlip.ProcessNext(ctx)

// Serialize to intermediate form
serializable, err := routingSlip.ToSerializable(resolver)
if err != nil {
    return err
}

// Marshal to JSON for transmission
jsonData, err := json.Marshal(serializable)
if err != nil {
    return err
}

// Send over message bus
bus.Publish("saga/routing-slip", jsonData)
```

### Step 3: Deserialize RoutingSlip

```go
// Receive from message bus
jsonData := bus.Receive("saga/routing-slip")

// Unmarshal from JSON
var serializable saga.SerializableRoutingSlip
err := json.Unmarshal(jsonData, &serializable)
if err != nil {
    return err
}

// Convert back to RoutingSlip (requires resolver)
routingSlip, err := saga.FromSerializable(&serializable, resolver)
if err != nil {
    return err
}

// Continue processing
routingSlip.ProcessNext(ctx)
```

## Serialized Format

The JSON format looks like this:

```json
{
  "completedWorkLogs": [
    {
      "activityTypeName": "ReserveCarActivity",
      "result": {
        "reservationId": 12345
      }
    }
  ],
  "nextWorkItems": [
    {
      "activityTypeName": "ReserveHotelActivity",
      "arguments": {
        "roomType": "Suite",
        "checkInDate": "2024-01-15"
      }
    },
    {
      "activityTypeName": "ReserveFlightActivity",
      "arguments": {
        "destination": "LAX",
        "flightDate": "2024-01-15"
      }
    }
  ]
}
```

## Advanced Patterns

### Multiple Resolvers for Different Services

Create service-specific resolvers to limit what activities each service knows about:

```go
// Orchestrator knows all activities
resolverOrchestrator := saga.NewMapBasedResolver()
resolverOrchestrator.Register("ReserveCarActivity", NewReserveCarActivity)
resolverOrchestrator.Register("ReserveHotelActivity", NewReserveHotelActivity)
resolverOrchestrator.Register("ReserveFlightActivity", NewReserveFlightActivity)

// Car service only knows car activities
resolverCarService := saga.NewMapBasedResolver()
resolverCarService.Register("ReserveCarActivity", NewReserveCarActivity)

// Hotel service only knows hotel activities
resolverHotelService := saga.NewMapBasedResolver()
resolverHotelService.Register("ReserveHotelActivity", NewReserveHotelActivity)
```

### Compensation Serialization

The same serialization works for compensation flows:

```go
// After failure, serialize for compensation
serializable, err := routingSlip.ToSerializable(resolver)
if err != nil {
    return err
}

jsonData, _ := json.Marshal(serializable)

// Route to compensation queue
compensationQueue := routingSlip.CompensationUri()
bus.Publish(compensationQueue, jsonData)

// On compensation service
var serializable saga.SerializableRoutingSlip
json.Unmarshal(jsonData, &serializable)

routingSlip, _ := saga.FromSerializable(&serializable, resolver)

// Perform compensation
for routingSlip.IsInProgress() {
    routingSlip.UndoLast(ctx)
}
```

### Testing with Isolated Resolvers

Use separate resolvers per test for complete isolation:

```go
func TestMyScenario(t *testing.T) {
    // Each test gets its own resolver
    resolver := saga.NewMapBasedResolver()
    resolver.Register("TestActivity", NewTestActivity)

    // No interference from other tests
    // No global state pollution
}
```

## Design Rationale

### Why Not Global Registry?

The interface-based approach (Variant 2) was chosen over a global registry (Variant 1) because:

1. **No Global State**: Each resolver is independent, preventing state pollution between tests and services
2. **Better Testability**: Tests can create isolated resolvers without cleanup
3. **Explicit Dependencies**: Resolvers are passed explicitly, making dependencies clear
4. **Service Isolation**: Different services can have different resolver configurations
5. **Thread Safety**: No need for mutex locks on global state

### Trade-offs

**Pros:**
- Clean dependency injection
- Excellent testability
- Service isolation
- No global state

**Cons:**
- Slightly more verbose (must pass resolver)
- Requires resolver on both serialization and deserialization sides
- Activities must implement `TypeName()` for fallback

## Error Handling

### Common Errors

1. **Unregistered Activity Type (Deserialization)**
```go
_, err := saga.FromSerializable(&serializable, resolver)
// Error: "activity type not registered: UnknownActivity"
```

**Solution:** Ensure all activities are registered before deserialization.

2. **Unregistered Activity Type (Serialization without NamedActivity)**
```go
_, err := routingSlip.ToSerializable(resolver)
// Error: "cannot serialize work item 0: activity type not registered"
```

**Solution:** Either register the activity or implement `NamedActivity.TypeName()`.

## Best Practices

1. **Register Activities at Startup**
   ```go
   func init() {
       resolver = saga.NewMapBasedResolver()
       resolver.Register("Activity1", NewActivity1)
       resolver.Register("Activity2", NewActivity2)
   }
   ```

2. **Implement NamedActivity**
   ```go
   func (a *MyActivity) TypeName() string {
       return "MyActivity"
   }
   ```

3. **Use Descriptive Names**
   ```go
   resolver.Register("ReserveCarActivity", NewReserveCarActivity)
   // Not: resolver.Register("car", NewReserveCarActivity)
   ```

4. **Share Resolver Configuration**
   ```go
   // Create a package-level function to configure resolver
   func NewConfiguredResolver() *saga.MapBasedResolver {
       resolver := saga.NewMapBasedResolver()
       // Register all activities
       return resolver
   }
   ```

5. **Test Round-Trip Serialization**
   ```go
   func TestRoundTrip(t *testing.T) {
       original := createRoutingSlip()
       serializable, _ := original.ToSerializable(resolver)
       jsonData, _ := json.Marshal(serializable)

       var restored saga.SerializableRoutingSlip
       json.Unmarshal(jsonData, &restored)
       routingSlip, _ := saga.FromSerializable(&restored, resolver)

       // Verify state preserved
       assert.Equal(t, len(original.PendingWorkItems()),
                    len(routingSlip.PendingWorkItems()))
   }
   ```

## See Also

- [activity_resolver_test.go](activity_resolver_test.go) - Resolver tests
- [routing_slip_serialization_test.go](routing_slip_serialization_test.go) - Serialization tests
- [examples/serialization_example_test.go](examples/serialization_example_test.go) - Complete examples

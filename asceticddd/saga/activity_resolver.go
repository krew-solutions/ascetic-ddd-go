package saga

import "fmt"

// ActivityTypeResolver is an interface for resolving activity types by name.
// This allows for dependency injection and better testability compared to global registries.
type ActivityTypeResolver interface {
	// Resolve returns the ActivityType for the given type name.
	Resolve(typeName string) (ActivityType, error)

	// GetName returns the type name for the given ActivityType.
	GetName(activityType ActivityType) (string, error)
}

// MapBasedResolver is a simple map-based implementation of ActivityTypeResolver.
type MapBasedResolver struct {
	nameToType map[string]ActivityType
	typeToName map[uintptr]string
}

// NewMapBasedResolver creates a new MapBasedResolver.
func NewMapBasedResolver() *MapBasedResolver {
	return &MapBasedResolver{
		nameToType: make(map[string]ActivityType),
		typeToName: make(map[uintptr]string),
	}
}

// Register registers an activity type with the given name.
func (r *MapBasedResolver) Register(name string, activityType ActivityType) {
	r.nameToType[name] = activityType

	// Store pointer for reverse lookup
	activity := activityType()
	if named, ok := activity.(NamedActivity); ok {
		r.typeToName[getActivityTypePointer(activityType)] = named.TypeName()
	} else {
		r.typeToName[getActivityTypePointer(activityType)] = name
	}
}

// Resolve returns the ActivityType for the given type name.
func (r *MapBasedResolver) Resolve(typeName string) (ActivityType, error) {
	activityType, ok := r.nameToType[typeName]
	if !ok {
		return nil, fmt.Errorf("activity type not registered: %s", typeName)
	}
	return activityType, nil
}

// GetName returns the type name for the given ActivityType.
func (r *MapBasedResolver) GetName(activityType ActivityType) (string, error) {
	ptr := getActivityTypePointer(activityType)
	name, ok := r.typeToName[ptr]
	if !ok {
		// Fallback: try to use NamedActivity interface
		activity := activityType()
		if named, ok := activity.(NamedActivity); ok {
			return named.TypeName(), nil
		}
		return "", fmt.Errorf("activity type not registered")
	}
	return name, nil
}

// NamedActivity is an optional interface that activities can implement
// to provide their type name explicitly.
type NamedActivity interface {
	Activity
	TypeName() string
}

// getActivityTypePointer extracts a pointer value from ActivityType for comparison.
func getActivityTypePointer(activityType ActivityType) uintptr {
	// Create an instance and get its type name as a simple hash alternative
	activity := activityType()
	if named, ok := activity.(NamedActivity); ok {
		// Use type name as a stable identifier
		name := named.TypeName()
		var hash uintptr
		for i := 0; i < len(name); i++ {
			hash = hash*31 + uintptr(name[i])
		}
		return hash
	}
	// Fallback to a simple counter-based approach
	return 0
}

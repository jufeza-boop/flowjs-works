package activities

import "flowjs-works/engine/internal/models"

// Activity defines the interface that all activity nodes must implement
type Activity interface {
	// Execute runs the activity with the given input and context
	Execute(input map[string]interface{}, config map[string]interface{}, ctx *models.ExecutionContext) (map[string]interface{}, error)
	
	// Name returns the name/type of the activity
	Name() string
}

// ActivityRegistry manages the available activities
type ActivityRegistry struct {
	activities map[string]Activity
}

// NewActivityRegistry creates a new activity registry
func NewActivityRegistry() *ActivityRegistry {
	registry := &ActivityRegistry{
		activities: make(map[string]Activity),
	}
	
	// Register built-in activities
	registry.Register(&LoggerActivity{})
	registry.Register(&HTTPActivity{})
	
	return registry
}

// Register adds an activity to the registry
func (r *ActivityRegistry) Register(activity Activity) {
	r.activities[activity.Name()] = activity
}

// Get retrieves an activity by name
func (r *ActivityRegistry) Get(name string) (Activity, bool) {
	activity, ok := r.activities[name]
	return activity, ok
}

// List returns all registered activity names
func (r *ActivityRegistry) List() []string {
	names := make([]string, 0, len(r.activities))
	for name := range r.activities {
		names = append(names, name)
	}
	return names
}

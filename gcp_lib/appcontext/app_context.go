package appcontext

import (
	"context"
	config "gcp_lib/config"
)

// Define the central Application Context
type AppContext struct {
	Config *config.Config // Holds the loaded configuration

	Ctx context.Context // The standard Go context for cancellation, deadlines, and tracing

	// Other global resources can go here, like DB connections:
	// DB *sql.DB
}

func InitializeApplication(cfg *config.Config) *AppContext {
	// This is the root context—it is never canceled, times out, or carries values.
	rootCtx := context.Background()

	// Create the base AppContext used by all subsequent requests/tasks
	Ctx := &AppContext{
		Config: cfg,
		Ctx:    rootCtx, // Storing the root context
	}
	return Ctx
}

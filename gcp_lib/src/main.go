package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	appcontext "gcp_lib/appcontext"
	common "gcp_lib/common"
	confstore "gcp_lib/config"
	gcppubsub "gcp_lib/lib_pubsub"
	"log/slog"
)

const (
	defaultConfigPath = "" // Use default config path
	topicName         = "orders"
	subscriptionName  = "orders-worker-1"
)

// processMessage handles incoming Pub/Sub messages
func processMessage(ctx context.Context, msg *gcppubsub.Message) error {
	slog.Info("Received message",
		"id", msg.ID,
		"publish_time", msg.PublishTime,
		"attempts", msg.DeliveryAttempt,
		"data", string(msg.Data),
	)
	return nil // Return error to nack the message
}

// testPubSubFunctions demonstrates Pub/Sub functionality
func testPubSubFunctions(ctx context.Context, projectID string) error {
	// Create Pub/Sub client
	client, err := gcppubsub.NewPubSubClient(ctx, projectID)
	if err != nil {
		return fmt.Errorf("failed to create Pub/Sub client: %w", err)
	}
	defer client.Close()

	// Get or create topic
	topic := client.Topic(topicName)

	// Check if topic exists
	exists, err := topic.Exists(ctx)
	if err != nil {
		return fmt.Errorf("failed to check topic existence: %w", err)
	}
	if !exists {
		return fmt.Errorf("topic %q does not exist", topicName)
	}

	// Create subscription with retry policy
	sub, err := client.CreateSubscription(ctx, subscriptionName, topic, &gcppubsub.SubscriptionConfig{
		AckDeadline: 30 * time.Second,
		MinBackoff:  10 * time.Second,
		MaxBackoff:  10 * time.Minute,
	})
	if err != nil {
		return fmt.Errorf("failed to create subscription: %w", err)
	}

	// Publish a test message
	msgID, err := topic.Publish(ctx, []byte("hello"), map[string]string{"key": "value"})
	if err != nil {
		return fmt.Errorf("failed to publish message: %w", err)
	}
	slog.Info("Published message", "message_id", msgID)

	// Start receiving messages with concurrency control
	receiveCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	err = sub.Receive(receiveCtx, processMessage, &gcppubsub.ReceiveConfig{
		Concurrency:            5, // Process up to 5 messages concurrently
		MaxOutstandingMessages: 100,
		EnableMessageOrdering:  false,
	})
	if err != nil && !errors.Is(err, context.Canceled) {
		return fmt.Errorf("error receiving messages: %w", err)
	}

	return nil
}

func main() {
	// Set up signal handling for graceful shutdown
	ctx, cancel := signal.NotifyContext(context.Background(),
		syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// Configure structured logging
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	// Load configuration
	cfg, status := confstore.LoadConfig(defaultConfigPath)
	if status != common.Success {
		slog.Error("Failed to load configuration", "status", status.String())
		os.Exit(1)
	}

	// Initialize application context
	appCtx := appcontext.InitializeApplication(cfg)
	if appCtx == nil {
		slog.Error("Failed to initialize application context")
		os.Exit(1)
	}

	// Get project ID from service account
	if appCtx.Config.SA.ProjectID == "" {
		slog.Error("Project ID not found in service account")
		os.Exit(1)
	}

	// Run Pub/Sub example
	if err := testPubSubFunctions(ctx, appCtx.Config.SA.ProjectID); err != nil {
		slog.Error("Pub/Sub test failed", "error", err)
		os.Exit(1)
	}

	slog.Info("Application shutdown complete")
}

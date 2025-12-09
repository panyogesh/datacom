// Package gcppubsub provides a high-level wrapper around the Google Cloud Pub/Sub client
// with improved error handling, resource management, and concurrency control.
package gcppubsub

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"
	"time"

	"cloud.google.com/go/pubsub"
	"google.golang.org/api/iterator"
)

var (
	// ErrSubscriptionExists is returned when attempting to create a subscription that already exists
	ErrSubscriptionExists = errors.New("subscription already exists")
	// ErrTopicNotFound is returned when a specified topic does not exist
	ErrTopicNotFound = errors.New("topic not found")
)

// Message represents a single data unit transmitted via Pub/Sub
type Message struct {
	ID              string            `json:"id"`               // Unique identifier for the message
	Data            []byte            `json:"data"`             // The actual payload of the message
	Attributes      map[string]string `json:"attributes"`       // Key-Value metadata
	PublishTime     time.Time         `json:"publish_time"`     // Time at which message was published
	DeliveryAttempt int               `json:"delivery_attempt"` // Number of delivery attempts
}

// Handler defines the function signature for processing received messages.
// If the handler returns an error, the message will be nacked and retried.
type Handler func(ctx context.Context, msg *Message) error

// PubSubClient is the main client for interacting with Google Cloud Pub/Sub.
// It provides a simplified interface for common operations.
type PubSubClient struct {
	client *pubsub.Client
}

// NewPubSubClient creates a new Pub/Sub client for the specified project.
// The client should be closed after use using the Close() method.
func NewPubSubClient(ctx context.Context, projectID string) (*PubSubClient, error) {
	if projectID == "" {
		return nil, errors.New("project ID cannot be empty")
	}

	client, err := pubsub.NewClient(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to create pubsub client: %w", err)
	}

	return &PubSubClient{client: client}, nil
}

// Close releases any resources held by the PubSubClient.
// It should be called when the client is no longer needed.
func (c *PubSubClient) Close() error {
	if c.client == nil {
		return nil
	}
	return c.client.Close()
}

// Topic represents a Pub/Sub topic and provides methods for publishing messages.
type Topic struct {
	t *pubsub.Topic
}

// Topic returns a reference to a topic with the given ID.
// The topic is created if it doesn't exist when the first message is published.
func (c *PubSubClient) Topic(topicID string) *Topic {
	if topicID == "" {
		return &Topic{t: nil}
	}
	return &Topic{t: c.client.Topic(topicID)}
}

// ID returns the ID of the topic.
func (t *Topic) ID() string {
	if t.t == nil {
		return ""
	}
	return t.t.ID()
}

// Exists checks if the topic exists in the project.
func (t *Topic) Exists(ctx context.Context) (bool, error) {
	if t.t == nil {
		return false, nil
	}

	topic := t.t
	_, err := topic.Config(ctx)

	return err == nil, err
}

// Publish publishes a message to the topic with the given data and attributes.
// It returns the published message ID or an error if the operation fails.
func (t *Topic) Publish(ctx context.Context, data []byte, attrs map[string]string) (string, error) {
	if t.t == nil {
		return "", errors.New("invalid topic")
	}

	msg := &pubsub.Message{
		Data:        data,
		Attributes:  attrs,
		PublishTime: time.Now(),
	}

	result := t.t.Publish(ctx, msg)
	msgID, err := result.Get(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to publish message: %w", err)
	}

	return msgID, nil
}

// Stop stops the topic's publishing goroutines and releases resources.
// It should be called when the topic is no longer needed.
func (t *Topic) Stop() {
	if t.t != nil {
		t.t.Stop()
	}
}

// Subscription represents a Pub/Sub subscription and provides methods for receiving messages.
type Subscription struct {
	s *pubsub.Subscription
}

// Subscription returns a reference to a subscription with the given ID.
func (c *PubSubClient) Subscription(subID string) *Subscription {
	if subID == "" {
		return &Subscription{s: nil}
	}
	return &Subscription{s: c.client.Subscription(subID)}
}

// ID returns the ID of the subscription.
func (s *Subscription) ID() string {
	if s.s == nil {
		return ""
	}
	return s.s.ID()
}

// Exists checks if the subscription exists in the project.
func (s *Subscription) Exists(ctx context.Context) (bool, error) {
	if s.s == nil {
		return false, nil
	}

	sub := s.s
	_, err := sub.Config(ctx)

	return err == nil, err
}

// ReceiveConfig holds configuration options for message receiving.
type ReceiveConfig struct {
	// Number of messages to be processed concurrently.
	// If not set, defaults to 1.
	Concurrency int

	// If true, enables message ordering (requires topic to be configured for ordering).
	EnableMessageOrdering bool

	// MaxOutstandingMessages is the maximum number of unprocessed messages.
	// If not set, defaults to 1000.
	MaxOutstandingMessages int

	// MaxOutstandingBytes is the maximum size of unprocessed messages.
	// If not set, defaults to 1e9 (1GB).
	MaxOutstandingBytes int
}

// Receive starts receiving messages and calls the handler for each message.
// It blocks until the context is canceled or an error occurs.
// The handler is called concurrently for multiple messages based on the provided config.
func (s *Subscription) Receive(ctx context.Context, handler Handler, cfg *ReceiveConfig) error {
	if s.s == nil {
		return errors.New("invalid subscription")
	}

	if cfg == nil {
		cfg = &ReceiveConfig{}
	}

	sub := s.s

	// Apply configuration
	sub.ReceiveSettings.Synchronous = true

	if cfg.Concurrency > 0 {
		sub.ReceiveSettings.NumGoroutines = cfg.Concurrency
	}

	if cfg.MaxOutstandingMessages > 0 {
		sub.ReceiveSettings.MaxOutstandingMessages = cfg.MaxOutstandingMessages
	}

	if cfg.MaxOutstandingBytes > 0 {
		sub.ReceiveSettings.MaxOutstandingBytes = cfg.MaxOutstandingBytes
	}

	// Start receiving messages
	return sub.Receive(ctx, func(ctx context.Context, m *pubsub.Message) {
		// Recover from panics in the message handler
		defer func() {
			if r := recover(); r != nil {
				// Log the panic and stack trace
				fmt.Printf("panic in message handler: %v\n%s\n", r, string(debug.Stack()))
				m.Nack() // Nack the message to be retried
			}
		}()

		msg := &Message{
			ID:              m.ID,
			Data:            m.Data,
			Attributes:      m.Attributes,
			PublishTime:     m.PublishTime,
			DeliveryAttempt: *m.DeliveryAttempt,
		}

		// Call the handler and handle the result
		if err := handler(ctx, msg); err != nil {
			m.Nack() // Negative acknowledgment - message will be retried
			return
		}

		m.Ack() // Acknowledge successful processing
	})
}

// Delete removes the subscription from the project.
func (s *Subscription) Delete(ctx context.Context) error {
	if s.s == nil {
		return errors.New("invalid subscription")
	}
	return s.s.Delete(ctx)
}

// SubscriptionConfig holds configuration options for creating a subscription.
type SubscriptionConfig struct {
	// The topic to subscribe to.
	Topic *Topic

	// How long to wait for a message to be acknowledged before redelivering it.
	// If not set, defaults to 10 seconds.
	AckDeadline time.Duration

	// The minimum time between consecutive message delivery attempts.
	// If not set, defaults to 10 seconds.
	MinBackoff time.Duration

	// The maximum time between consecutive message delivery attempts.
	// If not set, defaults to 600 seconds.
	MaxBackoff time.Duration

	// If true, the subscription will only receive messages published after the
	// subscription was created. If false, it will receive all messages retained
	// in the topic's backlog.
	StartAtTime time.Time
}

// CreateSubscription creates a new subscription to the specified topic.
// Returns ErrSubscriptionExists if the subscription already exists.
func (c *PubSubClient) CreateSubscription(
	ctx context.Context,
	subID string,
	topic *Topic,
	cfg *SubscriptionConfig,
) (*Subscription, error) {
	if subID == "" {
		return nil, errors.New("subscription ID cannot be empty")
	}

	if topic == nil || topic.t == nil {
		return nil, errors.New("topic cannot be nil")
	}

	// Check if subscription already exists
	sub := c.client.Subscription(subID)
	exists, err := sub.Exists(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to check subscription existence: %w", err)
	}
	if exists {
		return nil, ErrSubscriptionExists
	}

	// Configure the subscription
	subConfig := pubsub.SubscriptionConfig{
		Topic: topic.t,
	}

	if cfg != nil {
		if cfg.AckDeadline > 0 {
			subConfig.AckDeadline = cfg.AckDeadline
		}

		if !cfg.StartAtTime.IsZero() {
			subConfig.RetentionDuration = time.Since(cfg.StartAtTime)
		}

		// Configure retry policy if backoff values are provided
		if cfg.MinBackoff > 0 || cfg.MaxBackoff > 0 {
			minBackoff := 10 * time.Second
			if cfg.MinBackoff > 0 {
				minBackoff = cfg.MinBackoff
			}

			maxBackoff := 10 * time.Minute
			if cfg.MaxBackoff > 0 {
				maxBackoff = cfg.MaxBackoff
			}

			subConfig.RetryPolicy = &pubsub.RetryPolicy{
				MinimumBackoff: minBackoff,
				MaximumBackoff: maxBackoff,
			}
		}
	}

	// Create the subscription
	s, err := c.client.CreateSubscription(ctx, subID, subConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create subscription: %w", err)
	}

	return &Subscription{s: s}, nil
}

// ListSubscriptions returns a list of all subscriptions in the project.
func (c *PubSubClient) ListSubscriptions(ctx context.Context) ([]string, error) {
	var subs []string

	it := c.client.Subscriptions(ctx)
	for {
		sub, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to list subscriptions: %w", err)
		}
		subs = append(subs, sub.ID())
	}

	return subs, nil
}

// ListTopics returns a list of all topics in the project.
func (c *PubSubClient) ListTopics(ctx context.Context) ([]string, error) {
	var topics []string

	it := c.client.Topics(ctx)
	for {
		topic, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to list topics: %w", err)
		}
		topics = append(topics, topic.ID())
	}

	return topics, nil
}

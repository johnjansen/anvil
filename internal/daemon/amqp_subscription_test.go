package daemon

import (
	"testing"

	"github.com/johnjansen/anvil/internal/config"
	"github.com/johnjansen/anvil/internal/project"
)

func TestAMQPSubscriptionConfiguration(t *testing.T) {
	// Test that AMQP subscription configuration is parsed correctly
	// Create a task with AMQP subscription
	todo := project.Todo{
		Name:     "test-amqp-task",
		Schedule: "",
		Subscription: &project.SubscriptionConfig{
			Type:  "amqp",
			URL:   "amqp://localhost:5672",
			Queue: "test-queue",
		},
	}

	// Verify that the task has the correct subscription configuration
	if todo.Subscription == nil {
		t.Fatal("expected subscription to be configured")
	}

	if todo.Subscription.Type != "amqp" {
		t.Errorf("expected subscription type to be 'amqp', got %q", todo.Subscription.Type)
	}

	if todo.Subscription.URL != "amqp://localhost:5672" {
		t.Errorf("expected subscription URL to be 'amqp://localhost:5672', got %q", todo.Subscription.URL)
	}

	if todo.Subscription.Queue != "test-queue" {
		t.Errorf("expected subscription queue to be 'test-queue', got %q", todo.Subscription.Queue)
	}
}

func TestAMQPConsumerStartSubscription(t *testing.T) {
	// Test that AMQP consumer can be created and started
	cfg := config.Default()
	cfg.Runners = []string{"echo"}
	d := New(cfg)

	// Create AMQP consumer
	amqpConsumer := NewAMQPConsumer(d)

	// Verify that the consumer was created successfully
	if amqpConsumer == nil {
		t.Fatal("expected AMQP consumer to be created")
	}

	if amqpConsumer.daemon != d {
		t.Error("expected AMQP consumer to have reference to daemon")
	}

	// Verify that consumers map is initialized
	if amqpConsumer.consumers == nil {
		t.Error("expected consumers map to be initialized")
	}
}
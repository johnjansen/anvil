package daemon

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/johnjansen/anvil/internal/project"
	"github.com/streadway/amqp"
)

// AMQPConsumer handles consuming messages from AMQP queues and triggering tasks
type AMQPConsumer struct {
	daemon      *Daemon
	consumers   map[string]*consumer // taskID -> consumer
	consumersMu sync.RWMutex
}

// consumer represents a single AMQP consumer for a task
type consumer struct {
	conn    *amqp.Connection
	ch      *amqp.Channel
	queue   string
	task    project.Todo
	project *project.Project
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

// NewAMQPConsumer creates a new AMQP consumer manager
func NewAMQPConsumer(daemon *Daemon) *AMQPConsumer {
	return &AMQPConsumer{
		daemon:    daemon,
		consumers: make(map[string]*consumer),
	}
}

// StartSubscription starts consuming messages for a task with AMQP subscription
func (ac *AMQPConsumer) StartSubscription(proj *project.Project, task project.Todo) error {
	if task.Subscription == nil || task.Subscription.Type != "amqp" {
		return nil // Not an AMQP subscription
	}

	// Validate required fields
	if task.Subscription.URL == "" {
		return fmt.Errorf("AMQP subscription requires URL")
	}
	if task.Subscription.Queue == "" {
		return fmt.Errorf("AMQP subscription requires queue name")
	}

	ac.consumersMu.Lock()
	defer ac.consumersMu.Unlock()

	// Check if already consuming
	if _, exists := ac.consumers[task.ID]; exists {
		return nil // Already consuming
	}

	// Create new consumer
	ctx, cancel := context.WithCancel(context.Background())
	c := &consumer{
		queue:   task.Subscription.Queue,
		task:    task,
		project: proj,
		cancel:  cancel,
	}

	// Connect to AMQP
	conn, err := amqp.Dial(task.Subscription.URL)
	if err != nil {
		cancel()
		return fmt.Errorf("failed to connect to AMQP: %w", err)
	}
	c.conn = conn

	// Create channel
	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		cancel()
		return fmt.Errorf("failed to create AMQP channel: %w", err)
	}
	c.ch = ch

	// Set prefetch count (default to 1)
	prefetch := 1
	if task.Subscription.Prefetch > 0 {
		prefetch = task.Subscription.Prefetch
	}
	if err := ch.Qos(prefetch, 0, false); err != nil {
		ch.Close()
		conn.Close()
		cancel()
		return fmt.Errorf("failed to set QoS: %w", err)
	}

	// Declare queue
	q, err := ch.QueueDeclare(
		task.Subscription.Queue, // name
		true,                    // durable
		false,                   // delete when unused
		false,                   // exclusive
		false,                   // no-wait
		nil,                     // arguments
	)
	if err != nil {
		ch.Close()
		conn.Close()
		cancel()
		return fmt.Errorf("failed to declare queue: %w", err)
	}

	// Start consuming messages
	msgs, err := ch.Consume(
		q.Name, // queue
		"",     // consumer
		false,  // auto-ack
		false,  // exclusive
		false,  // no-local
		false,  // no-wait
		nil,    // args
	)
	if err != nil {
		ch.Close()
		conn.Close()
		cancel()
		return fmt.Errorf("failed to start consuming: %w", err)
	}

	// Store consumer
	ac.consumers[task.ID] = c

	// Start message processing goroutine
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		ac.processMessages(ctx, c, msgs)
	}()

	log.Printf("Started AMQP consumer for task %s on queue %s", task.Name, task.Subscription.Queue)
	return nil
}

// StopSubscription stops consuming messages for a task
func (ac *AMQPConsumer) StopSubscription(taskID string) error {
	ac.consumersMu.Lock()
	defer ac.consumersMu.Unlock()

	c, exists := ac.consumers[taskID]
	if !exists {
		return nil // Not consuming
	}

	// Cancel context to stop message processing
	c.cancel()

	// Close channel and connection
	if c.ch != nil {
		c.ch.Close()
	}
	if c.conn != nil {
		c.conn.Close()
	}

	// Wait for processing to finish
	c.wg.Wait()

	// Remove from consumers map
	delete(ac.consumers, taskID)

	return nil
}

// StopAll stops all active consumers
func (ac *AMQPConsumer) StopAll() {
	ac.consumersMu.Lock()
	defer ac.consumersMu.Unlock()

	for taskID, c := range ac.consumers {
		// Cancel context to stop message processing
		c.cancel()

		// Close channel and connection
		if c.ch != nil {
			c.ch.Close()
		}
		if c.conn != nil {
			c.conn.Close()
		}

		// Wait for processing to finish
		c.wg.Wait()

		// Remove from consumers map
		delete(ac.consumers, taskID)
	}
}

// processMessages processes incoming AMQP messages
func (ac *AMQPConsumer) processMessages(ctx context.Context, c *consumer, msgs <-chan amqp.Delivery) {
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-msgs:
			if !ok {
				// Channel closed
				return
			}

			// Process the message by triggering the task
			ac.handleMessage(c, msg)
		}
	}
}

// handleMessage handles a single AMQP message by triggering the associated task
func (ac *AMQPConsumer) handleMessage(c *consumer, msg amqp.Delivery) {
	// Create environment variables with message payload
	env := make(map[string]string)
	for k, v := range c.task.Env {
		env[k] = v
	}

	// Add message payload as environment variable
	if len(msg.Body) > 0 {
		env["ANVIL_AMQP_PAYLOAD"] = string(msg.Body)
	}

	// Create a copy of the task with updated environment
	taskWithEnv := c.task
	taskWithEnv.Env = env

	// Acknowledge the message immediately since we're triggering the task
	// The task execution will determine success/failure
	if err := msg.Ack(false); err != nil {
		log.Printf("Failed to acknowledge AMQP message: %v", err)
	}

	// Check if daemon is shutting down
	select {
	case <-ac.daemon.stop:
		return
	default:
	}

	// Add to work queue
	select {
	case ac.daemon.workQueue <- workItem{project: c.project, todo: taskWithEnv}:
		log.Printf("Enqueued task %s from AMQP subscription", taskWithEnv.Name)
	default:
		log.Printf("Work queue full, dropping task %s from AMQP subscription", taskWithEnv.Name)
	}
}
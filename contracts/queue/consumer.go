package queue

// Consumer describes a configured queue consumer.
type Consumer interface {
	// Key returns the queue.queues configuration key.
	Key() string
	// Prepare initializes dependencies before consuming messages.
	Prepare() error
	// Handle processes one queue message.
	Handle(data []byte) (response any, err error)
}

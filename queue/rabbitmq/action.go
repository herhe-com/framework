package rabbitmq

// Action controls how a delivery is acknowledged after the handler returns.
//
// The numeric values are unchanged from the previous client library so stored
// or logged values keep their meaning.
type Action int

const (
	// Ack acknowledges a successfully processed message.
	Ack Action = iota
	// NackDiscard rejects the message without requeueing it.
	NackDiscard
	// NackRequeue rejects the message and asks RabbitMQ to redeliver it.
	NackRequeue
	// Manual leaves acknowledgement to the handler. It is not supported by the
	// driver and is rejected like any unknown action.
	Manual
)

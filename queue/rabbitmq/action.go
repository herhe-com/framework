package rabbitmq

import rabbitmqclient "github.com/wagslane/go-rabbitmq"

// Action controls how a RabbitMQ delivery is acknowledged.
type Action = rabbitmqclient.Action

const (
	// Ack acknowledges a successfully processed message.
	Ack = rabbitmqclient.Ack
	// NackDiscard rejects the message without requeueing it.
	NackDiscard = rabbitmqclient.NackDiscard
	// NackRequeue rejects the message and asks RabbitMQ to redeliver it.
	NackRequeue = rabbitmqclient.NackRequeue
)

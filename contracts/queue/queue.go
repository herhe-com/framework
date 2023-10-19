package queue

type Queue interface {
	Driver
	Channel(channel string) (Driver, error)
}

type Driver interface {
	Producer(body []byte, exchange string, routes []string) error
	Consumer(handler func(data []byte), queue string) error
	Close() error
}

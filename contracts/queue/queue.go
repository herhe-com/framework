package queue

type Queue interface {
	Driver
	Channel(channel string) (Driver, error)
}

type Driver interface {
	Producer(body []byte, exchange string, routes []string, delays ...int64) error
	Consumer(handler func(data []byte), queue string, delays ...bool) error
	Close() error
}

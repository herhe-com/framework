package crontab

type Crontab interface {
	Key() string
	Name() string
	Func()
}

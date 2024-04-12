package crontab

type Crontab interface {
	Name() string
	Rule() string
	Func()
}

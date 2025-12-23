package redis

import (
	"context"
	"errors"
	"net"

	"github.com/gookit/color"
	"github.com/herhe-com/framework/facades"
	"github.com/redis/go-redis/v9"
	"github.com/redis/go-redis/v9/maintnotifications"
)

type Redis struct {
	channel  *redis.Client
	channels map[string]*redis.Client
}

func NewApplication() (*Redis, error) {

	channel, name, err := newRedisClient("default")

	if err != nil {
		color.Errorf("[redis] %s", err)
		return nil, err
	}

	channels := make(map[string]*redis.Client)
	channels[name] = channel

	return &Redis{
		channels: channels,
		channel:  channel,
	}, nil
}

func (r *Redis) Channel(name string) (*redis.Client, error) {

	if dri, exist := r.channels[name]; exist {
		return dri, nil
	}

	dri, _, err := newRedisClient(name)

	if err != nil {
		return nil, err
	}

	r.channels[name] = dri

	return dri, nil
}

func (r *Redis) Default() *redis.Client {
	return r.channel
}

func newRedisClient(name string) (*redis.Client, string, error) {

	var db int
	var username, password, host, port string

	username = facades.Cfg.GetString("database.redis." + name + ".username")
	password = facades.Cfg.GetString("database.redis." + name + ".password")
	host = facades.Cfg.GetString("database.redis." + name + ".host")
	port = facades.Cfg.GetString("database.redis."+name+".port", "6379")
	db = facades.Cfg.GetInt("database.redis."+name+".db", 1)

	if host == "" {
		return nil, "", errors.New("invalid database config: mysql")
	}

	addr := net.JoinHostPort(host, port)

	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Username: username,
		Password: password,
		DB:       db,
		MaintNotificationsConfig: &maintnotifications.Config{
			Mode: maintnotifications.ModeDisabled,
		},
	})

	ctx := context.Background()

	if _, err := client.Ping(ctx).Result(); err != nil {
		return nil, "", err
	}

	return client, name, nil
}

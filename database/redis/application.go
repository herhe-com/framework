package redis

import (
	"context"
	"errors"
	"net"

	"github.com/gookit/color"
	redisconfig "github.com/herhe-com/framework/database/redis/config"
	"github.com/redis/go-redis/v9"
	"github.com/redis/go-redis/v9/maintnotifications"
)

type Redis struct {
	channel  *redis.Client
	channels map[string]*redis.Client
}

const DriverRedis string = "redis"

func NewApplication() (*Redis, error) {

	defaultName := DefaultName()
	channel, name, err := newRedisClient(defaultName)

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

// DefaultName returns the configured default redis connection name.
func DefaultName() string {
	return redisconfig.DefaultName()
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

	if configDriver := redisconfig.Driver(name, DriverRedis); configDriver != DriverRedis {
		return nil, "", errors.New("invalid database config: redis driver")
	}

	username = redisconfig.ConnectionString(name, "username", "")
	password = redisconfig.ConnectionString(name, "password", "")
	host = redisconfig.ConnectionString(name, "host", "")
	port = redisconfig.ConnectionString(name, "port", "6379")
	db = redisconfig.ConnectionInt(name, "db", 1)

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

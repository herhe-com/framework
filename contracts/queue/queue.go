package queue

import "github.com/wagslane/go-rabbitmq"

type Queue interface {
	Driver
	Channel(channel string, name string) (Driver, error)
}

type Driver interface {
	// Producer 生产者
	/*
	 * @Description: 队列生产者
	 * @Param: body 消息体
	 * @Param: exchange 交换机名称
	 * @Param: queue 队列名称
	 * @Param: routes 路由名称
	 * @Param: delay 延迟队列时间（秒）
	 * @Param: ttl 死信队列时间（秒）
	 * @Param: headers 消息头
	 */
	Producer(body []byte, exchange, queue string, routes []string, delay, ttl int64, headers ...rabbitmq.Table) error
	// Consumer 消费者
	/*
	 * @Param: handler 消费者回调函数
	 * @Param: exchange 交换机名称
	 * @Param: queue 队列名称
	 * @Param: route 路由名称
	 * @Param: delay 延迟队列时间（秒）
	 * @Param: ttl 死信队列时间（秒）
	 * @Param: retry 重试次数
	 */
	Consumer(handler func(data []byte) error, exchange, queue, route string, delay bool, ttl int64, retry int) error
	/*
	 * @Description: 关闭队列
	 */
	Close() error
}

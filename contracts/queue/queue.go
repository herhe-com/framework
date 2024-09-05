package queue

type Queue interface {
	Driver
	Channel(channel string) (Driver, error)
}

type Driver interface {
	/*
	 * @Description: 队列生产者
	 * @Param: body 消息体
	 * @Param: exchange 交换机名称
	 * @Param: routes 路由名称
	 * @Param: delays 延迟队列时间（秒）
	 */
	Producer(body []byte, exchange string, routes []string, delays ...int64) error
	/*
	 * @Description: 队列消费者
	 * @Param: handler 消费者处理函数
	 * @Param: queue 队列名称
	 * @Param: delays 是否延迟队列
	 */
	Consumer(handler func(data []byte), queue string, delays ...bool) error
	/*
	 * @Description: 关闭队列
	 */
	Close() error
}

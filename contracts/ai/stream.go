package ai

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// StreamLimit 是一条流式请求的时间口径。
//
// HTTP 客户端的 timeout 只约束到响应头:拿到 SSE 连接后读取正文不再受它约束,
// 所以「多久没有新输出」与「整次最多跑多久」必须由读取侧自己看门。两个维度缺一不可:
// 推理模型的思考过程本身可能很长,按总时长一刀切会误伤正常输出;只盯空闲又兜不住「一直慢速输出」。
//
// 字段为 0 或负数时表示该维度不限制。
type StreamLimit struct {
	Idle     time.Duration // 空闲超时:连续这么久没有任何分片即判定卡死,连接建立阶段同样计入
	Deadline time.Duration // 总时长上限:整次调用最多持续这么久
}

// StreamLimitError 表示流式调用被 StreamLimit 中断。
//
// Idle 为 true 是空闲超时(limit 内没有任何分片),为 false 是总时长超限;
// Limit 是触发中断的那条口径,便于调用方写进面向用户的提示。
type StreamLimitError struct {
	Idle  bool
	Limit time.Duration
}

func (e *StreamLimitError) Error() string {

	if e.Idle {
		return fmt.Sprintf("stream idle timeout: no chunk within %s", e.Limit)
	}

	return fmt.Sprintf("stream deadline exceeded: limit %s", e.Limit)
}

// StreamOf 发起流式请求并读完整个流,把每个分片交给 consume。
//
// 超时(空闲看门狗 + 总时长上限)、取消与流的释放都在这里完成:调用方不必自己起守护 goroutine,
// 也不必从中断原因里反推是空闲超时还是总时长超限。
//
// consume 返回错误时立即中断流并原样返回该错误,用于调用方按自身规则提前收手
// (例如识别到模型在循环输出)。consume 为 nil 时只读不处理。
//
// 返回的错误按以下优先级取:StreamLimit 触发的中断、consume 的错误、流内错误、驱动调用错误。
func StreamOf(c context.Context, driver Driver, request *ChatRequest, limit StreamLimit, consume func(*StreamResponse) error) error {

	// 用 cancel cause 承载中断原因:生产者被取消时只会报一个 context canceled,
	// 真正的原因要从这里取。
	ctx, cancel := context.WithCancelCause(c)
	defer cancel(nil)

	// 空闲看门狗:每收到一个分片就唤醒它重置计时,真的没有新输出才中断。
	reset := make(chan struct{}, 1)
	if limit.Idle > 0 {
		go idleOfStream(ctx, cancel, limit.Idle, reset)
	}

	// 总时长上限。
	if limit.Deadline > 0 {
		expire := time.AfterFunc(limit.Deadline, func() {
			cancel(&StreamLimitError{Idle: false, Limit: limit.Deadline})
		})
		defer expire.Stop()
	}

	stream, err := driver.Stream(ctx, request)

	if err == nil {
		// 必须读完通道:生产者在发送被阻塞时,只有 ctx 取消才能退出。
		for item := range stream {
			if item.Error != nil {
				err = item.Error
				break
			}

			// 收到分片(包括只有思考过程的分片)就重置空闲计时。
			if limit.Idle > 0 {
				select {
				case reset <- struct{}{}:
				default:
				}
			}

			if consume != nil {
				if itemErr := consume(item); itemErr != nil {
					err = itemErr
					break
				}
			}
		}
	}

	// 优先用自己的中断原因:驱动或生产者抛出的 context canceled 是结果,不是原因。
	if cause := context.Cause(ctx); cause != nil {
		var expired *StreamLimitError
		if errors.As(cause, &expired) {
			err = expired
		} else if err == nil {
			err = cause
		}
	}

	// 中断后立即释放流:生产者还在发送时会因 ctx 取消而退出,HTTP 响应体也随之关闭。
	cancel(nil)

	return err
}

// idleOfStream 看住空闲超时:每收到一个分片重置计时,limit 内没有任何分片就以
// StreamLimitError 取消 ctx。
func idleOfStream(ctx context.Context, cancel context.CancelCauseFunc, limit time.Duration, reset <-chan struct{}) {

	timer := time.NewTimer(limit)
	defer timer.Stop()

	for {
		select {
		case <-reset:
			// 计时器可能已经触发,Reset 前先按标准姿势把已到期的值排空。
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			timer.Reset(limit)
		case <-timer.C:
			cancel(&StreamLimitError{Idle: true, Limit: limit})
			return
		case <-ctx.Done():
			return
		}
	}
}

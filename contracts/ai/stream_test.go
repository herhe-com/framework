package ai

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

// streamDriver 是只实现流式的假驱动:produce 里自行决定怎么发分片与何时停。
type streamDriver struct {
	produce func(c context.Context, ch chan *StreamResponse)
	err     error
}

func (d *streamDriver) Chat(context.Context, *ChatRequest) (*ChatResponse, error) {
	return nil, errors.New("chat not implemented")
}

func (d *streamDriver) Stream(c context.Context, _ *ChatRequest) (chan *StreamResponse, error) {

	if d.err != nil {
		return nil, d.err
	}

	ch := make(chan *StreamResponse)

	go func() {
		defer close(ch)
		d.produce(c, ch)
	}()

	return ch, nil
}

func (d *streamDriver) Dri() string {
	return "stream"
}

func (d *streamDriver) Model() string {
	return "fake"
}

// chunkOf 构造一个只带正文的分片。
func chunkOf(content string) *StreamResponse {
	return &StreamResponse{Model: "fake", Choices: []StreamChoice{{Delta: MessageDelta{Content: content}}}}
}

func TestStreamOfConsume(t *testing.T) {

	driver := &streamDriver{produce: func(c context.Context, ch chan *StreamResponse) {
		Send(c, ch, chunkOf("你"))
		Send(c, ch, chunkOf("好"))
		Send(c, ch, &StreamResponse{Done: true})
	}}

	total := strings.Builder{}
	err := StreamOf(context.Background(), driver, &ChatRequest{}, StreamLimit{}, func(item *StreamResponse) error {
		for _, choice := range item.Choices {
			total.WriteString(choice.Delta.Content)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("stream: %v", err)
	}

	if total.String() != "你好" {
		t.Fatalf("分片内容: 期望 你好, 实际 %q", total.String())
	}
}

func TestStreamOfIdleTimeout(t *testing.T) {

	chunks := 0
	driver := &streamDriver{produce: func(c context.Context, ch chan *StreamResponse) {
		Send(c, ch, chunkOf("开头"))
		// 之后一直不再输出:只能由空闲看门狗中断。
		<-c.Done()
	}}

	err := StreamOf(context.Background(), driver, &ChatRequest{}, StreamLimit{Idle: 30 * time.Millisecond}, func(*StreamResponse) error {
		chunks++
		return nil
	})

	var expired *StreamLimitError
	if !errors.As(err, &expired) {
		t.Fatalf("期望 StreamLimitError, 实际 %v", err)
	}
	if !expired.Idle || expired.Limit != 30*time.Millisecond {
		t.Fatalf("中断口径: 期望空闲 30ms, 实际 %+v", expired)
	}
	if chunks != 1 {
		t.Fatalf("已收分片数: 期望 1, 实际 %d", chunks)
	}
}

func TestStreamOfDeadline(t *testing.T) {

	driver := &streamDriver{produce: func(c context.Context, ch chan *StreamResponse) {
		// 分片一直到达,空闲超时不会触发,只有总时长上限能收住。
		for {
			select {
			case <-time.After(10 * time.Millisecond):
			case <-c.Done():
				return
			}
			if !Send(c, ch, chunkOf("x")) {
				return
			}
		}
	}}

	started := time.Now()
	err := StreamOf(context.Background(), driver, &ChatRequest{}, StreamLimit{
		Idle:     500 * time.Millisecond,
		Deadline: 60 * time.Millisecond,
	}, nil)

	var expired *StreamLimitError
	if !errors.As(err, &expired) {
		t.Fatalf("期望 StreamLimitError, 实际 %v", err)
	}
	if expired.Idle || expired.Limit != 60*time.Millisecond {
		t.Fatalf("中断口径: 期望总时长 60ms, 实际 %+v", expired)
	}
	if elapsed := time.Since(started); elapsed > 400*time.Millisecond {
		t.Fatalf("应按总时长上限及时中断, 实际耗时 %v", elapsed)
	}
}

func TestStreamOfConsumeStop(t *testing.T) {

	stop := errors.New("识别到循环输出")
	driver := &streamDriver{produce: func(c context.Context, ch chan *StreamResponse) {
		for {
			if !Send(c, ch, chunkOf("重复")) {
				return
			}
		}
	}}

	calls := 0
	err := StreamOf(context.Background(), driver, &ChatRequest{}, StreamLimit{}, func(*StreamResponse) error {
		calls++
		if calls == 2 {
			return stop
		}
		return nil
	})

	if !errors.Is(err, stop) {
		t.Fatalf("期望 consume 的错误被原样返回, 实际 %v", err)
	}
	if calls != 2 {
		t.Fatalf("consume 调用次数: 期望 2, 实际 %d", calls)
	}
}

func TestStreamOfDriverError(t *testing.T) {

	expect := errors.New("网关拒绝连接")
	driver := &streamDriver{err: expect}

	err := StreamOf(context.Background(), driver, &ChatRequest{}, StreamLimit{Idle: time.Second}, nil)
	if !errors.Is(err, expect) {
		t.Fatalf("期望驱动错误被透出, 实际 %v", err)
	}
}

func TestStreamOfUnlimited(t *testing.T) {

	driver := &streamDriver{produce: func(c context.Context, ch chan *StreamResponse) {
		// 两个维度都不限制时,慢输出不应被中断。
		select {
		case <-time.After(30 * time.Millisecond):
		case <-c.Done():
			return
		}
		Send(c, ch, chunkOf("慢但正常"))
	}}

	err := StreamOf(context.Background(), driver, &ChatRequest{}, StreamLimit{}, nil)
	if err != nil {
		t.Fatalf("不限制时不应中断: %v", err)
	}
}

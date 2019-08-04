package gconcurrent

import (
	"context"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestWorkerParam(t *testing.T) {
	pf := func(recovered interface{}, funcName string) {

	}
	w := NewWorkerPool(WpOption{InitWorkerNum: 1, MaxWorkerNum: 5, QueueSize: 10, PanicFunc: pf})
	wp := w.(*workerPool)
	assert.Equal(t, 0, w.GetActiveNum())
	assert.Equal(t, 1, w.GetWorkerNum())
	assert.Equal(t, 10, w.GetQueueSize())
	assert.Equal(t, 5, wp.maxWorkerNum)
	assert.NotNil(t, wp.handlePanic)
	w.Shutdown(context.Background())
}

func TestWorkerExecute(t *testing.T) {
	w := NewWorkerPool()
	evt := make(chan struct{})

	w.Execute(func(ctx context.Context) {
		evt <- struct{}{}
		evt <- struct{}{}
	})

	<-evt
	assert.Equal(t, 1, w.GetActiveNum())

	<-evt
	time.Sleep(20 * time.Millisecond)
	assert.Equal(t, 0, w.GetActiveNum())

	close(evt)
	w.Shutdown(context.Background())
}

func TestWorkerExecuteIncWorker(t *testing.T) {
	w := NewWorkerPool(WpOption{InitWorkerNum: 1, MaxWorkerNum: 5, QueueSize: 100})
	evt := make(chan struct{})

	w.Execute(func(ctx context.Context) {
		evt <- struct{}{}
		println("TestWorkerExecuteIncWorker_1")
	})

	time.Sleep(50 * time.Millisecond)
	w.Execute(func(ctx context.Context) {
		evt <- struct{}{}
		println("TestWorkerExecuteIncWorker_2")
	})

	<-evt
	<-evt
	assert.Equal(t, 2, w.GetWorkerNum())

	close(evt)
	w.Shutdown(context.Background())
}

func TestWorkerSubmitTimeout(t *testing.T) {
	w := NewWorkerPool(WpOption{InitWorkerNum: 1, MaxWorkerNum: 1, QueueSize: 1})
	evt := make(chan struct{})

	err := w.Submit(func(ctx context.Context) {
		evt <- struct{}{}
		println("TestWorkerSubmitTimeout_1")
	}, time.Second)
	assert.Nil(t, err)

	err = w.Submit(func(ctx context.Context) {
		evt <- struct{}{}
		println("TestWorkerSubmitTimeout_2")
	}, time.Second)
	assert.Nil(t, err)

	err = w.Submit(func(ctx context.Context) {
		evt <- struct{}{}
		println("TestWorkerSubmitTimeout_3")
	}, 5*time.Millisecond)
	assert.Equal(t, ErrTimeout, err)

	close(evt)
	w.Shutdown(context.Background())

}

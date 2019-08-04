package gconcurrent

import (
	"context"
	"errors"
	"reflect"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

var (
	ErrTimeout = errors.New("add to job queue timeout")
)

// JobFunc is a job function will execute by worker goroutine
type JobFunc func(context.Context)

// PanicFunc is a function will process panic which throw in worker function
type PanicFunc func(recovered interface{}, funcName string)

type queueItem struct {
	jobFunc  JobFunc
	funcName string
}

type WorkerPool interface {
	// Execute add worker function to queue to wait for executing by worker goroutine
	Execute(f JobFunc)

	// Execute summit worker function to queue  with a timeout timer,
	// if add queue success it will wait for executing by worker goroutine
	Submit(f JobFunc, timeout time.Duration) error

	// Stop cancel all goroutines started by this pool and wait
	Shutdown(ctx context.Context)

	// GetActiveNum return the current number of active goroutine which is processing a job
	GetActiveNum() int

	// GetWorkerNum return the number of started worker goroutine
	GetWorkerNum() int

	// GetQueueSize return the number of queue size
	GetQueueSize() int
}

type workerPool struct {
	queue         chan *queueItem
	queueSize     int
	workerNum     int32
	initWorkerNum int
	maxWorkerNum  int
	activeNum     int32

	mux    sync.Mutex
	ctx    context.Context
	cancel context.CancelFunc

	handlePanic PanicFunc
}

// WpOption is the worker pool parameter
type WpOption struct {
	InitWorkerNum int       `json:"init_worker_num" yaml:"init_worker_num"`
	MaxWorkerNum  int       `json:"max_worker_num" yaml:"max_worker_num"`
	QueueSize     int       `json:"queue_size" yaml:"queue_size"`
	PanicFunc     PanicFunc `json:"-"`
}

// NewWorkerPool creates a instance of WorkerPool with given option
// default parameters:
//   queueSize: 100
//   initWorkerNum: 2
//   maxWorkerNum: 50
func NewWorkerPool(opt ...WpOption) WorkerPool {
	ctx, cancel := context.WithCancel(context.TODO())
	wp := &workerPool{
		queueSize:     100,
		initWorkerNum: 2,
		maxWorkerNum:  50,
		ctx:           ctx,
		cancel:        cancel,
	}

	if len(opt) >= 1 {
		cfg := opt[0]
		wp.initWorkerNum = cfg.InitWorkerNum
		wp.maxWorkerNum = cfg.MaxWorkerNum
		wp.queueSize = cfg.QueueSize
		wp.handlePanic = cfg.PanicFunc
	}

	wp.queue = make(chan *queueItem, wp.queueSize)
	wp.run(wp.initWorkerNum)

	return wp
}

func (w *workerPool) GetActiveNum() int {
	return int(atomic.LoadInt32(&w.activeNum))
}

func (w *workerPool) GetWorkerNum() int {
	return int(atomic.LoadInt32(&w.workerNum))
}

func (w *workerPool) GetQueueSize() int {
	return w.queueSize
}

func (w *workerPool) toItem(jf JobFunc) *queueItem {
	pc := reflect.ValueOf(jf).Pointer()
	f := runtime.FuncForPC(pc)
	funcName := f.Name()

	return &queueItem{
		jobFunc:  jf,
		funcName: funcName,
	}
}

func (w *workerPool) run(incNum int) {
	for idx := 0; idx < incNum; idx++ {
		atomic.AddInt32(&w.workerNum, 1)
		go func() {
			for it := range w.queue {
				w.executeOne(it)
			}
		}()
	}
}

func (w *workerPool) executeOne(it *queueItem) {
	atomic.AddInt32(&w.activeNum, 1)
	defer func() {
		recovered := recover()
		if recovered != nil && w.handlePanic != nil {
			w.handlePanic(recovered, it.funcName)
		}
		atomic.AddInt32(&w.activeNum, -1)
	}()
	it.jobFunc(w.ctx)
}

func (w *workerPool) incWorker() {
	activeNum := w.GetActiveNum()
	workerNum := w.GetWorkerNum()
	if activeNum == workerNum && workerNum < w.maxWorkerNum {
		w.mux.Lock()
		workerNum = w.GetWorkerNum()
		incNum := workerNum / 2
		if incNum < 1 {
			incNum = 1
		}
		if workerNum+incNum > w.maxWorkerNum {
			incNum = w.maxWorkerNum - workerNum
		}
		w.run(incNum)
		w.mux.Unlock()
	}

}

func (w *workerPool) Execute(jf JobFunc) {
	w.incWorker()
	w.queue <- w.toItem(jf)
}

func (w *workerPool) Submit(jf JobFunc, timeout time.Duration) error {
	w.incWorker()
	select {
	case <-time.NewTimer(timeout).C:
		return ErrTimeout
	case w.queue <- w.toItem(jf):
		return nil
	}
}

func (w *workerPool) Shutdown(ctx context.Context) {
	close(w.queue)
	w.cancel()

	for {
		timer := time.NewTimer(time.Millisecond * 100)
		select {
		case <-timer.C:
			if w.checkNoActive() {
				return
			}
		case <-ctx.Done():
			return
		}
	}
}

func (w *workerPool) checkNoActive() bool {
	return atomic.LoadInt32(&w.activeNum) == 0
}

package gconcurrent

import (
	"context"
	"fmt"
	"runtime"
	"sync"
)

// TaskGroup 包为一组子任务的 goroutine 提供了 goroutine 同步,错误取消功能.
//
//TaskGroup 包含三种常用方式
//
//1、直接使用 此时不会因为一个任务失败导致所有任务被 cancel:
//		g := &gconcurrent.TaskGroup{}
//		g.Go(func(ctx context.Context) {
//			// NOTE: 此时 ctx 为 context.Background()
//			// do something
//		})
//
//2、WithContext 使用 NewTaskGroupWithContext 时不会因为一个任务失败导致所有任务被 cancel:
//		g := gconcurrent.NewTaskGroupWithContext(ctx)
//		g.Go(func(ctx context.Context) {
//			// NOTE: 此时 ctx 为 gconcurrent.NewTaskGroupWithContext 传递的 ctx
//			// do something
//		})
//
//3、WithCancel 使用 NewTaskGroupWithCancel 时如果有一个人任务失败会导致所有*未进行或进行中*的任务被 cancel:
//		g := gconcurrent.NewTaskGroupWithCancel(ctx)
//		g.Go(func(ctx context.Context) {
//			// NOTE: 此时 ctx 是从 gconcurrent.NewTaskGroupWithCancel 传递的 ctx 派生出的 ctx
//			// do something
//		})
//
//设置最大并行数 GOMAXPROCS 对以上三种使用方式均起效
//NOTE: 由于 taskgroup 实现问题,设定 GOMAXPROCS 的 taskgroup 需要立即调用 Wait() 例如:
//
//		g := gconcurrent.WithCancel(ctx)
//		g.GOMAXPROCS(2)
//		// task1
//		g.Go(func(ctx context.Context) {
//			fmt.Println("task1")
//		})
//		// task2
//		g.Go(func(ctx context.Context) {
//			fmt.Println("task2")
//		})
//		// task3
//		g.Go(func(ctx context.Context) {
//			fmt.Println("task3")
//		})
//		// NOTE: 此时设置的 GOMAXPROCS 为2, 添加了三个任务 task1, task2, task3 此时 task3 是不会运行的!
//		// 只有调用了 Wait task3 才有运行的机会
//		g.Wait() // task3 运行

// A TaskGroup is a collection of goroutines working on subtasks that are part of
// the same overall task.
//
// A zero Group is valid and does not cancel on error.
type TaskGroup struct {
	err     error
	wg      sync.WaitGroup
	errOnce sync.Once

	workerOnce sync.Once
	ch         chan func(ctx context.Context) error
	chs        []func(ctx context.Context) error

	ctx    context.Context
	cancel func()
}

// NewTaskGroupWithContext create a Group.
// given function from Go will receive this context,
func NewTaskGroupWithContext(ctx context.Context) *TaskGroup {
	return &TaskGroup{ctx: ctx}
}

// NewTaskGroupWithCancel create a new Group and an associated Context derived from ctx.
//
// given function from Go will receive context derived from this ctx,
// The derived Context is canceled the first time a function passed to Go
// returns a non-nil error or the first time Wait returns, whichever occurs
// first.
func NewTaskGroupWithCancel(ctx context.Context) *TaskGroup {
	ctx, cancel := context.WithCancel(ctx)
	return &TaskGroup{ctx: ctx, cancel: cancel}
}

func (g *TaskGroup) do(f func(ctx context.Context) error) {
	ctx := g.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	var err error
	defer func() {
		if r := recover(); r != nil {
			buf := make([]byte, 64<<10)
			buf = buf[:runtime.Stack(buf, false)]
			err = fmt.Errorf("errgroup: panic recovered: %s\n%s", r, buf)
		}
		if err != nil {
			g.errOnce.Do(func() {
				g.err = err
				if g.cancel != nil {
					g.cancel()
				}
			})
		}
		g.wg.Done()
	}()
	err = f(ctx)
}

// GOMAXPROCS set max goroutine to work.
func (g *TaskGroup) GOMAXPROCS(n int) {
	if n <= 0 {
		panic("task group: GOMAXPROCS must great than 0")
	}
	g.workerOnce.Do(func() {
		g.ch = make(chan func(context.Context) error, n)
		for i := 0; i < n; i++ {
			go func() {
				for f := range g.ch {
					g.do(f)
				}
			}()
		}
	})
}

// Go calls the given function in a new goroutine.
//
// The first call to return a non-nil error cancels the group; its error will be
// returned by Wait.
func (g *TaskGroup) Go(f func(ctx context.Context) error) {
	g.wg.Add(1)
	if g.ch != nil {
		select {
		case g.ch <- f:
		default:
			g.chs = append(g.chs, f)
		}
		return
	}
	go g.do(f)
}

// Wait blocks until all function calls from the Go method have returned, then
// returns the first non-nil error (if any) from them.
func (g *TaskGroup) Wait() error {
	if g.ch != nil {
		for _, f := range g.chs {
			g.ch <- f
		}
	}
	g.wg.Wait()
	if g.ch != nil {
		close(g.ch) // let all receiver exit
	}
	if g.cancel != nil {
		g.cancel()
	}
	return g.err
}

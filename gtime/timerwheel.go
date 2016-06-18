package gtime

import (
	"crypto/md5"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"io"
	"sync"
	"time"
)

const (
	defaultTickDuration = 100 * time.Millisecond //默认的时间轮间隔时间
	defaultWheelCount   = 512                    //默认的卡槽数512个
)

// TimerWheel 时钟轮
type TimerWheel struct {
	lock          sync.Mutex    // 锁
	tickDuration  time.Duration // 卡槽每次跳动的时间间隔
	roundDuration time.Duration // 一轮耗时
	wheelCount    int           // 卡槽数
	wheel         []*iterator   // 卡槽
	quit          chan struct{} // 退出
	wheelCursor   int           // 当前卡槽位置
}

// iterator 时间轮卡槽迭代器
type iterator struct {
	items map[string]*wheelTimeOut
}

// wheelTimeOut 超时处理对象
type wheelTimeOut struct {
	id       string        // 定时器标识
	delay    time.Duration // 延迟时间
	index    int           // 卡槽索引位置
	rounds   int           // 需要转动的周期数
	task     func()        // 到期执行的任务
	outCh    chan struct{} // 到期事件队列
	times    int           // 超时设定的次数
	exptimes int           // 已超时的次数
}

// NewTimerWheel 初始化时间轮对象
func NewTimerWheel() *TimerWheel {
	return &TimerWheel{
		tickDuration:  defaultTickDuration,
		wheelCount:    defaultWheelCount,
		wheel:         createWheel(defaultWheelCount),
		quit:          make(chan struct{}),
		wheelCursor:   0,
		roundDuration: defaultTickDuration * defaultWheelCount,
	}
}

// Start 启动时间轮
func (t *TimerWheel) Start() {
	tick := time.NewTicker(t.tickDuration)
	go func() {
		for {
			select {
			case <-tick.C:
				t.wheelCursor++
				if t.wheelCursor == t.wheelCount {
					t.wheelCursor = 0
				}

				iterator := t.wheel[t.wheelCursor]
				tasks := t.fetchExpiredTimeouts(iterator)
				t.notifyExpiredTimeOut(tasks)
			case <-t.quit:
				tick.Stop()
				return
			}
		}
	}()
}

// Stop 停止时间轮
func (t *TimerWheel) Stop() {
	t.quit <- struct{}{}
}

func createWheel(wheelCount int) []*iterator {
	arr := make([]*iterator, wheelCount)
	for v := 0; v < wheelCount; v++ {
		arr[v] = &iterator{items: make(map[string]*wheelTimeOut)}
	}
	return arr
}

// AfterFunc 添加一个超时任务
func (t *TimerWheel) AfterFunc(delay time.Duration, times int, f func()) (string, error) {
	if f == nil {
		return "", errors.New("task is empty")
	}

	if delay <= 0 {
		return "", errors.New("delay Must be greater than zero")
	}

	if times <= 0 {
		times = 0x0FFFFFFF
	}

	timeOut := &wheelTimeOut{
		delay: delay,
		task:  f,
		times: times,
	}

	tid, err := t.scheduleTimeOut(timeOut)
	return tid, err
}

// After 添加一个超时任务
func (t *TimerWheel) After(delay time.Duration, times int) (chan struct{}, string, error) {
	if delay <= 0 {
		return nil, "", errors.New("delay Must be greater than zero")
	}

	if times <= 0 {
		times = 0x0FFFFFFF
	}

	timeOut := &wheelTimeOut{
		delay: delay,
		outCh: make(chan struct{}),
		times: times,
	}

	tid, err := t.scheduleTimeOut(timeOut)
	return timeOut.outCh, tid, err
}

// Cancel 取消定时器
func (t *TimerWheel) Cancel(timerID string) error {
	t.lock.Lock()
	defer t.lock.Unlock()
	for _, it := range t.wheel {
		delete(it.items, timerID)
	}
	return nil
}

func (t *TimerWheel) scheduleTimeOut(timeOut *wheelTimeOut) (string, error) {
	if timeOut.delay < t.tickDuration {
		timeOut.delay = t.tickDuration
	}
	lastRoundDelay := timeOut.delay % t.roundDuration
	lastTickDelay := timeOut.delay % t.tickDuration

	// 计算卡槽位置
	relativeIndex := lastRoundDelay / t.tickDuration
	if lastTickDelay != 0 {
		relativeIndex = relativeIndex + 1
	}
	// 计算时间轮圈数
	remainingRounds := timeOut.delay / t.roundDuration
	if lastRoundDelay == 0 {
		remainingRounds = remainingRounds - 1
	}

	t.lock.Lock()
	defer t.lock.Unlock()

	stopIndex := t.wheelCursor + int(relativeIndex)
	if stopIndex >= t.wheelCount {
		stopIndex = stopIndex - t.wheelCount
		timeOut.rounds = int(remainingRounds) + 1
	} else {
		timeOut.rounds = int(remainingRounds)
	}
	timeOut.index = stopIndex
	item := t.wheel[stopIndex]
	if item == nil {
		item = &iterator{
			items: make(map[string]*wheelTimeOut),
		}
	}

	if timeOut.id == "" {
		key, err := guid()
		if err != nil {
			return "", err
		}
		timeOut.id = key
	}

	item.items[timeOut.id] = timeOut
	t.wheel[stopIndex] = item

	return timeOut.id, nil
}

// 判断当前卡槽中是否有超时任务,将超时task加入切片中
func (t *TimerWheel) fetchExpiredTimeouts(iter *iterator) []*wheelTimeOut {
	t.lock.Lock()
	defer t.lock.Unlock()

	task := []*wheelTimeOut{}

	for k, v := range iter.items {
		if v.rounds <= 0 { //已经超时了
			task = append(task, v)
			delete(iter.items, k)
		} else {
			v.rounds--
		}
	}

	return task
}

// 执行超时任务
func (t *TimerWheel) notifyExpiredTimeOut(tasks []*wheelTimeOut) {
	for _, task := range tasks {
		task.exptimes++
		if task.exptimes < task.times { // 如果执行的次数小于设置的次数，则再次调度
			t.scheduleTimeOut(task)
		}

		if task.task != nil {
			go task.task()
		} else {
			go func(times int) {
				task.outCh <- struct{}{}
				if times == task.times {
					close(task.outCh)
				}
			}(task.exptimes)
		}
	}
}

func md5str(s string) string {
	h := md5.New()
	h.Write([]byte(s))
	return hex.EncodeToString(h.Sum(nil))

}

func guid() (string, error) {
	b := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return "", err
	}

	return md5str(base64.URLEncoding.EncodeToString(b)), nil
}

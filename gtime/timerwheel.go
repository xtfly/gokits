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
	defaultTickDuration = 100 * time.Millisecond //默认的时间轮 间隔时间 1秒
	defaultWheelCount   = 512                    //默认的卡槽数512个
)

// TimerWheel 时钟轮
type TimerWheel struct {
	state         int           //启动(1)or 停止(-1) 状态
	tickDuration  time.Duration //卡槽每次跳动的时间间隔
	roundDuration time.Duration //一轮耗时
	wheelCount    int           //卡槽数
	wheel         []*Iterator   //卡槽
	tick          *time.Ticker  //时钟
	lock          sync.Mutex    //锁
	wheelCursor   int           //当前卡槽位置
	mask          int           //卡槽最大索引数
}

// Iterator 时间轮卡槽迭代器
type Iterator struct {
	items map[string]*WheelTimeOut
}

// WheelTimeOut 超时处理对象
type WheelTimeOut struct {
	id       string        // 定时器标识
	delay    time.Duration // 延迟时间
	index    int           // 卡槽索引位置
	rounds   int           // 需要转动的周期数
	task     func()        // 到期执行的任务
	times    int           // 超时设定的次数
	exptimes int           // 已超时的次数
}

// NewTimerWheel 初始化时间轮对象
func NewTimerWheel() *TimerWheel {
	return &TimerWheel{
		tickDuration:  defaultTickDuration,
		wheelCount:    defaultWheelCount,
		wheel:         createWheel(),
		wheelCursor:   0,
		mask:          defaultWheelCount - 1,
		roundDuration: defaultTickDuration * defaultWheelCount,
	}
}

// Start 启动时间轮
func (t *TimerWheel) Start() {
	t.lock.Lock()

	t.tick = time.NewTicker(defaultTickDuration)
	defer t.lock.Unlock()

	go func() {
		for range t.tick.C {
			t.wheelCursor++
			if t.wheelCursor == defaultWheelCount {
				t.wheelCursor = 0
			}

			iterator := t.wheel[t.wheelCursor]
			tasks := t.fetchExpiredTimeouts(iterator)
			t.notifyExpiredTimeOut(tasks)
		}
	}()
}

// Stop 停止时间轮
func (t *TimerWheel) Stop() {
	t.tick.Stop()
}

func createWheel() []*Iterator {
	arr := make([]*Iterator, defaultWheelCount)
	for v := 0; v < defaultWheelCount; v++ {
		arr[v] = &Iterator{items: make(map[string]*WheelTimeOut)}
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

	timeOut := &WheelTimeOut{
		delay: delay,
		task:  f,
		times: times,
	}

	tid, err := t.scheduleTimeOut(timeOut)
	return tid, err
}

// Cancel ..
func (t *TimerWheel) Cancel(timerID string) error {
	for _, it := range t.wheel {
		for k := range it.items {
			if timerID == k {
				delete(it.items, k)
			}
		}
	}
	return nil
}

func (t *TimerWheel) scheduleTimeOut(timeOut *WheelTimeOut) (string, error) {
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
	if stopIndex >= defaultWheelCount {
		stopIndex = stopIndex - defaultWheelCount
		timeOut.rounds = int(remainingRounds) + 1
	} else {
		timeOut.rounds = int(remainingRounds)
	}
	timeOut.index = stopIndex
	item := t.wheel[stopIndex]
	if item == nil {
		item = &Iterator{
			items: make(map[string]*WheelTimeOut),
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
func (t *TimerWheel) fetchExpiredTimeouts(iterator *Iterator) []*WheelTimeOut {
	t.lock.Lock()
	defer t.lock.Unlock()

	task := []*WheelTimeOut{}

	for k, v := range iterator.items {
		if v.rounds <= 0 { //已经超时了
			task = append(task, v)
			delete(iterator.items, k)
		} else {
			v.rounds--
		}
	}

	return task
}

// 执行超时任务
func (t *TimerWheel) notifyExpiredTimeOut(tasks []*WheelTimeOut) {
	for _, task := range tasks {
		task.exptimes++
		if task.exptimes < task.times { // 如果执行的次数小于设置的次数，则再次调度
			t.scheduleTimeOut(task)
		}
		go task.task()
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

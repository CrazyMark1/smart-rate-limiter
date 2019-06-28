package limit

import (
	"fmt"
	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/mem"
	"sync/atomic"
	"time"
)

type TokenBucket struct {
	Capacity          int64        //令牌桶容量  可以缓存部分令牌
	Ch                chan bool    //令牌桶
	CountPerSecond    int64        //每秒生成令牌数
	AddTimer          *time.Ticker //增加令牌的定时器
	RealPerSecond     int64        //实际上每秒的请求量
	Counter           int64        //计数器
	RealPerSenconds   *Queue       //缓存每秒请求量的队列
	ResetCounterTimer *time.Ticker //重置计数器的定时器
	UpdateTimer       *time.Ticker //更新生成令牌速率的定时器
	CpuPercent        float64      //cpu使用率的预警值
	MemPercent        float64      //内存使用率的预警值
}

/**
创建一个令牌桶
*/
func NewTokenBucket(capacity int64, countPerSecond int64, cpuPercent float64, memPercent float64) *TokenBucket {
	tokenBucket := &TokenBucket{
		Capacity:          capacity,
		Ch:                make(chan bool, capacity),
		CountPerSecond:    countPerSecond,
		AddTimer:          time.NewTicker(time.Duration(int64(time.Second) / countPerSecond)),
		RealPerSenconds:   &Queue{},
		ResetCounterTimer: time.NewTicker(1 * time.Second),
		UpdateTimer:       time.NewTicker(10 * time.Second),
		CpuPercent:        cpuPercent,
		MemPercent:        memPercent,
	}
	//启动增加令牌的协程
	go tokenBucket.add()
	//启动更新生成令牌速率的协程
	go tokenBucket.updateRate()
	//启动记录请求速率协程
	go tokenBucket.resetAndSaveCounter()
	return tokenBucket
}

func (tokenBucket *TokenBucket) add() {
	for {
		select {
		case <-tokenBucket.AddTimer.C:
			tokenBucket.doAdd()
		}
	}
}

/**
获取令牌，没有获取到直接返回
*/
func (tokenBucket *TokenBucket) GetToken() bool {
	atomic.AddInt64(&tokenBucket.Counter, 1)
	select {
	case <-tokenBucket.Ch:
		return true
	default:
		return false
	}
}

/**
获取令牌，没有获取到则等待，直到超时
*/
func (tokenBucket *TokenBucket) TryGetToken(timeout time.Duration) bool {
	atomic.AddInt64(&tokenBucket.Counter, 1)
	t := time.NewTimer(timeout)
	defer t.Stop()
	select {
	case <-tokenBucket.Ch:
		return true
	case <-t.C:
		return false
	}
}

func (tokenBucket *TokenBucket) doAdd() {
	if int64(len(tokenBucket.Ch)) < tokenBucket.Capacity {
		tokenBucket.Ch <- true
	}
}

/**
如果cpu使用率或者内存使用率大于预警值，则令牌生成速率下降10%
如果cpu使用率与内存使用率低于预警值的80%，并且真实请求令牌速率大于令牌生成速率，则令牌生成速率提升10%
*/
func (tokenBucket *TokenBucket) updateRate() {
	for {
		select {
		case <-tokenBucket.UpdateTimer.C:
			v, _ := mem.VirtualMemory()
			//内存使用
			mem := v.UsedPercent
			//cpu使用
			cc, _ := cpu.Percent(10*time.Second, false)
			if cc[0] > tokenBucket.MemPercent || mem > tokenBucket.MemPercent {
				tokenBucket.CountPerSecond = tokenBucket.CountPerSecond - tokenBucket.CountPerSecond/10
				tokenBucket.doUpdateRate()
			}

			if cc[0] < tokenBucket.MemPercent-tokenBucket.MemPercent/5 &&
				mem < tokenBucket.MemPercent-tokenBucket.MemPercent/5 &&
				tokenBucket.RealPerSecond > tokenBucket.CountPerSecond {
				tokenBucket.CountPerSecond = tokenBucket.CountPerSecond + tokenBucket.CountPerSecond/10
				tokenBucket.doUpdateRate()
			}
			fmt.Printf("内存使用率：%f,cpu使用率：%f,当前令牌桶限速:%d,真实速度：%d\n",
				mem, cc[0], tokenBucket.CountPerSecond, tokenBucket.RealPerSecond)
		}
	}

}

func (tokenBucket *TokenBucket) doUpdateRate() {
	t := tokenBucket.AddTimer
	defer t.Stop()
	tokenBucket.AddTimer = time.NewTicker(tokenBucket.getDuration())
}

func (tokenBucket *TokenBucket) getDuration() time.Duration {
	return time.Duration(int64(time.Second) / tokenBucket.CountPerSecond)
}

/**
真实令牌请求速率取最近10s内，每秒请求的最大值
*/
func (tokenBucket *TokenBucket) resetAndSaveCounter() {
	for {
		select {
		case <-tokenBucket.ResetCounterTimer.C:
			//如果队列大于10 则丢弃先塞入的值 直到size=10
			for i := 0; i < len(*tokenBucket.RealPerSenconds)-10; i++ {
				tokenBucket.RealPerSenconds.Pop()
			}

			tokenBucket.RealPerSenconds.Push(tokenBucket.Counter)
			//计数器清零
			for {
				if atomic.CompareAndSwapInt64(&tokenBucket.Counter, tokenBucket.Counter, 0) {
					break
				}
			}
			var temp int64
			for _, value := range *tokenBucket.RealPerSenconds {
				if temp < value {
					temp = value
				}
			}
			tokenBucket.RealPerSecond = temp
		}
	}

}

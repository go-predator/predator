/*
 * @Author: thepoy
 * @Email: thepoy@163.com
 * @File Name: pool.go
 * @Created: 2021-07-29 22:30:37
 * @Modified:  2021-11-24 20:50:22
 */

package predator

import (
	"errors"
	"fmt"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog"
)

// errors
var (
	// return if pool size <= 0
	ErrInvalidPoolCap = errors.New("invalid pool cap")
	// put task but pool already closed
	ErrPoolAlreadyClosed = errors.New("pool already closed")
	// only the error type can be captured and processed
	ErrUnkownType = errors.New("recover only allows error type, but an unknown type is received")
)

// running status
const (
	RUNNING = 1
	STOPED  = 0
)

// Task task to-do
type Task struct {
	crawler   *Crawler
	req       *Request
	isChained bool
}

// Pool task pool
type Pool struct {
	capacity       uint64
	runningWorkers uint64
	status         int64
	chTask         chan *Task
	log            *zerolog.Logger
	blockPanic     bool
	sync.Mutex
}

// NewPool init pool
func NewPool(capacity uint64) (*Pool, error) {
	if capacity <= 0 {
		return nil, ErrInvalidPoolCap
	}
	p := &Pool{
		capacity: capacity,
		status:   RUNNING,
		chTask:   make(chan *Task, capacity),
	}

	return p, nil
}

func (p *Pool) checkWorker() {
	p.Lock()
	defer p.Unlock()

	if p.runningWorkers == 0 && len(p.chTask) > 0 {
		p.run()
	}
}

// GetCap get capacity
func (p *Pool) GetCap() uint64 {
	return p.capacity
}

// GetRunningWorkers get running workers
func (p *Pool) GetRunningWorkers() uint64 {
	return atomic.LoadUint64(&p.runningWorkers)
}

func (p *Pool) incRunning() {
	atomic.AddUint64(&p.runningWorkers, 1)
}

func (p *Pool) decRunning() {
	atomic.AddUint64(&p.runningWorkers, ^uint64(0))
}

// Put put a task to pool
func (p *Pool) Put(task *Task) error {
	p.Lock()
	defer p.Unlock()

	if p.status == STOPED {
		return ErrPoolAlreadyClosed
	}

	// run worker
	if p.GetRunningWorkers() < p.GetCap() {
		p.run()
	}

	// send task
	if p.status == RUNNING {
		p.chTask <- task
	}

	return nil
}

func (p *Pool) run() {
	p.incRunning()

	go func() {
		defer func() {
			p.decRunning()

			if r := recover(); r != nil {
				if p.blockPanic {
					// 打印panic的堆栈信息
					debug.PrintStack()

					p.log.Error().Err(fmt.Errorf("worker panic: %s", r))
				} else {
					// panic 只允许 error 类型
					if e, ok := r.(error); ok {
						panic(e)
					} else {
						panic(fmt.Sprintf("%s: %v", ErrUnkownType, r))
					}
				}
			}

			p.checkWorker() // check worker avoid no worker running
		}()

		for task := range p.chTask {
			task.crawler.prepare(task.req, task.isChained)
		}
	}()

}

func (p *Pool) setStatus(status int64) bool {
	p.Lock()
	defer p.Unlock()

	if p.status == status {
		return false
	}

	p.status = status

	return true
}

// Close close pool graceful
func (p *Pool) Close() {

	if !p.setStatus(STOPED) { // stop put task
		return
	}

	for len(p.chTask) > 0 { // wait all task be consumed
		time.Sleep(1e6) // reduce CPU load
	}

	close(p.chTask)
}

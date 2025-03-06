package parallel

import "sync"

type Pool interface {
	Run(runner func())
	Wait()
}

type SequentialPool struct{}

func NewSequentialPool() *SequentialPool {
	return &SequentialPool{}
}

func (p *SequentialPool) Run(runner func()) {
	runner()
}

func (p *SequentialPool) Wait() {}

type ConcurrentPool struct {
	wg   sync.WaitGroup
	pool chan struct{}
}

func NewConcurrentPool(size int) *ConcurrentPool {
	return &ConcurrentPool{
		pool: make(chan struct{}, size),
	}
}

func (p *ConcurrentPool) Run(runner func()) {
	p.wg.Add(1)
	go func() {
		p.pool <- struct{}{}
		defer func() {
			<-p.pool
			p.wg.Done()
		}()
		runner()
	}()
}

func (p *ConcurrentPool) Wait() {
	p.wg.Wait()
}

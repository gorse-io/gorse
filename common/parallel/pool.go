package parallel

import "sync"

type Pool interface {
	Run(runner func())
	Wait()
}

type SequentialPool struct{}

func (p *SequentialPool) Run(runner func()) {
	runner()
}

func (p *SequentialPool) Wait() {}

type ConcurrentPool struct {
	wg sync.WaitGroup
}

func (p *ConcurrentPool) Run(runner func()) {
	p.wg.Add(1)
	go func() {
		runner()
		p.wg.Done()
	}()
}

func (p *ConcurrentPool) Wait() {
	p.wg.Wait()
}

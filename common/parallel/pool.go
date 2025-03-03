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

type InfinitePool struct {
	wg sync.WaitGroup
}

func (p *InfinitePool) Run(runner func()) {
	p.wg.Add(1)
	go func() {
		runner()
		p.wg.Done()
	}()
}

func (p *InfinitePool) Wait() {
	p.wg.Wait()
}

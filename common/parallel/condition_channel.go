// Copyright 2022 gorse Project Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package parallel

type ConditionChannel struct {
	in chan struct{} // input channel
	C  chan struct{} // output channel
}

func NewConditionChannel() *ConditionChannel {
	in := make(chan struct{})
	out := make(chan struct{})
	go func() {
		count := 0
		for {
			if count == 0 {
				<-in
				count++
			} else {
				select {
				case <-in:
					count++
				case out <- struct{}{}:
					count = 0
				}
			}
		}
	}()
	return &ConditionChannel{in: in, C: out}
}

func (c *ConditionChannel) Signal() {
	c.in <- struct{}{}
}

func (c *ConditionChannel) Close() {
	close(c.in)
	close(c.C)
}

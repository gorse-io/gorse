// Copyright 2020 gorse Project Authors
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
package worker

import (
	"fmt"
	"github.com/hashicorp/memberlist"
	"time"
)

type Worker struct {
}

func (w *Worker) Serve() {
	config := memberlist.DefaultLocalConfig()
	config.Name = "Worker"
	list, err := memberlist.Create(config)
	if err != nil {
		panic("Failed to create memberlist: " + err.Error())
	}

	_, err = list.Join([]string{"192.168.1.162:8001"})
	if err != nil {
		panic("Failed to join cluster: " + err.Error())
	}

	for {
		for _, member := range list.Members() {
			fmt.Printf("Member: %s %s %v\n", member.Name, member.Addr, member.Port)
		}
		time.Sleep(time.Second * 10)
	}
}

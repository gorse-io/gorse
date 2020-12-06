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
package protocol

import (
	"encoding/json"
	"log"
)

const WorkerNodePrefix = "worker"
const LeaderNodePrefix = "leader"

type Meta struct {
	Role string
	Port int
	Name string
}

func NewName(role string, port int, hostname string) string {
	data, err := json.Marshal(Meta{
		Role: role,
		Port: port,
		Name: hostname,
	})
	if err != nil {
		log.Fatal(err)
	}
	return string(data)
}

func ParseName(s string) (string, int, string, error) {
	var meta Meta
	err := json.Unmarshal([]byte(s), &meta)
	return meta.Role, meta.Port, meta.Name, err
}

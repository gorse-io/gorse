// Copyright 2021 gorse Project Authors
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

package master

import (
	"context"
	"encoding/json"
	"time"

	"github.com/gorse-io/gorse/protocol"
	"github.com/gorse-io/gorse/storage/meta"
	"github.com/juju/errors"
)

// GetMeta returns latest configuration.
func (m *Master) GetMeta(ctx context.Context, nodeInfo *protocol.NodeInfo) (*protocol.Meta, error) {
	// register node
	node := &meta.Node{
		UUID:       nodeInfo.Uuid,
		Hostname:   nodeInfo.Hostname,
		Type:       nodeInfo.NodeType.String(),
		Version:    nodeInfo.BinaryVersion,
		UpdateTime: time.Now().UTC(),
	}
	if err := m.metaStore.UpdateNode(node); err != nil {
		return nil, err
	}
	// marshall config
	s, err := json.Marshal(m.Config)
	if err != nil {
		return nil, err
	}
	// save ranking model version
	m.collaborativeFilteringModelMutex.RLock()
	var collaborativeFilteringModelId int64
	if m.CollaborativeFilteringModel != nil && !m.CollaborativeFilteringModel.Invalid() {
		collaborativeFilteringModelId = m.CollaborativeFilteringModelId
	}
	m.collaborativeFilteringModelMutex.RUnlock()
	// save click model version
	m.clickModelMutex.RLock()
	var clickThroughRateModelId int64
	if m.ClickModel != nil && !m.ClickModel.Invalid() {
		clickThroughRateModelId = m.ClickThroughRateModelId
	}
	m.clickModelMutex.RUnlock()
	// collect nodes
	workers := make([]string, 0)
	servers := make([]string, 0)
	nodes, err := m.metaStore.ListNodes()
	if err != nil {
		return nil, err
	}
	for _, n := range nodes {
		switch n.Type {
		case protocol.NodeType_Worker.String():
			workers = append(workers, n.UUID)
		case protocol.NodeType_Server.String():
			servers = append(servers, n.UUID)
		}
	}
	return &protocol.Meta{
		Config:                        string(s),
		CollaborativeFilteringModelId: collaborativeFilteringModelId,
		ClickThroughRateModelId:       clickThroughRateModelId,
		Me:                            nodeInfo.Uuid,
		Workers:                       workers,
		Servers:                       servers,
	}, nil
}

func (m *Master) PushProgress(
	_ context.Context,
	in *protocol.PushProgressRequest) (*protocol.PushProgressResponse, error) {
	// check empty progress
	if len(in.Progress) == 0 {
		return &protocol.PushProgressResponse{}, nil
	}
	// check tracers
	tracer := in.Progress[0].Tracer
	for _, p := range in.Progress {
		if p.Tracer != tracer {
			return nil, errors.Errorf("tracers must be the same, expect %v, got %v", tracer, p.Tracer)
		}
	}
	// store progress
	m.remoteProgress.Store(tracer, protocol.DecodeProgress(in))
	return &protocol.PushProgressResponse{}, nil
}

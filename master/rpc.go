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
	"github.com/juju/errors"
	"github.com/zhenghaoz/gorse/base/log"
	"github.com/zhenghaoz/gorse/model/cf"
	"github.com/zhenghaoz/gorse/model/ctr"
	"github.com/zhenghaoz/gorse/protocol"
	"github.com/zhenghaoz/gorse/storage/meta"
	"go.uber.org/zap"
	"io"
	"time"
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
	var rankingModelVersion int64
	if m.CollaborativeFilteringModel != nil && !m.CollaborativeFilteringModel.Invalid() {
		rankingModelVersion = m.CollaborativeFilteringModelVersion
	}
	m.collaborativeFilteringModelMutex.RUnlock()
	// save click model version
	m.clickModelMutex.RLock()
	var clickModelVersion int64
	if m.ClickModel != nil && !m.ClickModel.Invalid() {
		clickModelVersion = m.ClickModelVersion
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
		Config:              string(s),
		RankingModelVersion: rankingModelVersion,
		ClickModelVersion:   clickModelVersion,
		Me:                  nodeInfo.Uuid,
		Workers:             workers,
		Servers:             servers,
	}, nil
}

// GetRankingModel returns latest ranking model.
func (m *Master) GetRankingModel(version *protocol.VersionInfo, sender protocol.Master_GetRankingModelServer) error {
	m.collaborativeFilteringModelMutex.RLock()
	defer m.collaborativeFilteringModelMutex.RUnlock()
	// skip empty model
	if m.CollaborativeFilteringModel == nil || m.CollaborativeFilteringModel.Invalid() {
		return errors.New("no valid model found")
	}
	// check model version
	if m.CollaborativeFilteringModelVersion != version.Version {
		return errors.New("model version mismatch")
	}
	// encode model
	reader, writer := io.Pipe()
	var encoderError error
	go func() {
		defer func(writer *io.PipeWriter) {
			err := writer.Close()
			if err != nil {
				log.Logger().Error("fail to close pipe", zap.Error(err))
			}
		}(writer)
		err := cf.MarshalModel(writer, m.CollaborativeFilteringModel)
		if err != nil {
			log.Logger().Error("fail to marshal collaborative filtering model", zap.Error(err))
			encoderError = err
			return
		}
	}()
	// send model
	for {
		buf := make([]byte, batchSize)
		n, err := reader.Read(buf)
		if err == io.EOF {
			log.Logger().Debug("complete sending ranking model")
			break
		} else if err != nil {
			return err
		}
		err = sender.Send(&protocol.Fragment{Data: buf[:n]})
		if err != nil {
			return err
		}
	}
	return encoderError
}

// GetClickModel returns latest click model.
func (m *Master) GetClickModel(version *protocol.VersionInfo, sender protocol.Master_GetClickModelServer) error {
	m.clickModelMutex.RLock()
	defer m.clickModelMutex.RUnlock()
	// skip empty model
	if m.ClickModel == nil || m.ClickModel.Invalid() {
		return errors.New("no valid model found")
	}
	// check empty model
	if m.ClickModelVersion != version.Version {
		return errors.New("model version mismatch")
	}
	// encode model
	reader, writer := io.Pipe()
	var encoderError error
	go func() {
		defer func(writer *io.PipeWriter) {
			err := writer.Close()
			if err != nil {
				log.Logger().Error("fail to close pipe", zap.Error(err))
			}
		}(writer)
		err := ctr.MarshalModel(writer, m.ClickModel)
		if err != nil {
			log.Logger().Error("fail to marshal click model", zap.Error(err))
			encoderError = err
			return
		}
	}()
	// send model
	for {
		buf := make([]byte, batchSize)
		n, err := reader.Read(buf)
		if err == io.EOF {
			log.Logger().Debug("complete sending click model")
			break
		} else if err != nil {
			return err
		}
		err = sender.Send(&protocol.Fragment{Data: buf[:n]})
		if err != nil {
			return err
		}
	}
	return encoderError
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

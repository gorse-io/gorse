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
	"github.com/zhenghaoz/gorse/base"
	"github.com/zhenghaoz/gorse/model/click"
	"github.com/zhenghaoz/gorse/model/ranking"
	"github.com/zhenghaoz/gorse/protocol"
	"go.uber.org/zap"
	"google.golang.org/grpc/peer"
	"io"
	"strings"
)

// Node could be worker node for server node.
type Node struct {
	Name     string
	Type     string
	IP       string
	HttpPort int64
}

const (
	ServerNode = "Server"
	WorkerNode = "Worker"
)

// NewNode creates a node from Context and NodeInfo.
func NewNode(ctx context.Context, nodeInfo *protocol.NodeInfo) *Node {
	node := new(Node)
	node.Name = nodeInfo.NodeName
	node.HttpPort = nodeInfo.HttpPort
	// read address
	p, _ := peer.FromContext(ctx)
	hostAndPort := p.Addr.String()
	node.IP = strings.Split(hostAndPort, ":")[0]
	// read type
	switch nodeInfo.NodeType {
	case protocol.NodeType_ServerNode:
		node.Type = ServerNode
	case protocol.NodeType_WorkerNode:
		node.Type = WorkerNode
	}
	return node
}

// GetMeta returns latest configuration.
func (m *Master) GetMeta(ctx context.Context, nodeInfo *protocol.NodeInfo) (*protocol.Meta, error) {
	// register node
	node := NewNode(ctx, nodeInfo)
	if node.Type != "" {
		if err := m.ttlCache.Set(nodeInfo.NodeName, node); err != nil {
			base.Logger().Error("failed to set ttl cache", zap.Error(err))
			return nil, err
		}
	}
	// marshall config
	s, err := json.Marshal(m.GorseConfig)
	if err != nil {
		return nil, err
	}
	// collect user index version
	m.userIndexMutex.RLock()
	var userIndexVersion int64
	if m.userIndex != nil {
		userIndexVersion = m.userIndexVersion
	}
	m.userIndexMutex.RUnlock()
	// save ranking model version
	m.rankingModelMutex.RLock()
	var rankingModelVersion int64
	if m.rankingModel != nil && !m.rankingModel.Invalid() {
		rankingModelVersion = m.rankingModelVersion
	}
	m.rankingModelMutex.RUnlock()
	// save click model version
	m.clickModelMutex.RLock()
	var clickModelVersion int64
	if m.clickModel != nil && !m.clickModel.Invalid() {
		clickModelVersion = m.clickModelVersion
	}
	m.clickModelMutex.RUnlock()
	// collect nodes
	workers := make([]string, 0)
	servers := make([]string, 0)
	m.nodesInfoMutex.RLock()
	for name, info := range m.nodesInfo {
		switch info.Type {
		case WorkerNode:
			workers = append(workers, name)
		case ServerNode:
			servers = append(servers, name)
		}
	}
	m.nodesInfoMutex.RUnlock()
	return &protocol.Meta{
		Config:              string(s),
		UserIndexVersion:    userIndexVersion,
		RankingModelVersion: rankingModelVersion,
		ClickModelVersion:   clickModelVersion,
		Me:                  nodeInfo.NodeName,
		Workers:             workers,
		Servers:             servers,
	}, nil
}

// GetRankingModel returns latest ranking model.
func (m *Master) GetRankingModel(version *protocol.VersionInfo, sender protocol.Master_GetRankingModelServer) error {
	m.rankingModelMutex.RLock()
	defer m.rankingModelMutex.RUnlock()
	// skip empty model
	if m.rankingModel == nil || m.rankingModel.Invalid() {
		return errors.New("no valid model found")
	}
	// check model version
	if m.rankingModelVersion != version.Version {
		return errors.New("model version mismatch")
	}
	// encode model
	reader, writer := io.Pipe()
	var encoderError error
	go func() {
		defer func(writer *io.PipeWriter) {
			err := writer.Close()
			if err != nil {
				base.Logger().Error("fail to close pipe", zap.Error(err))
			}
		}(writer)
		err := ranking.MarshalModel(writer, m.rankingModel)
		if err != nil {
			base.Logger().Error("fail to marshal ranking model", zap.Error(err))
			encoderError = err
			return
		}
	}()
	// send model
	for {
		buf := make([]byte, batchSize)
		n, err := reader.Read(buf)
		if err == io.EOF {
			base.Logger().Debug("complete sending ranking model")
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
	if m.clickModel == nil || m.clickModel.Invalid() {
		return errors.New("no valid model found")
	}
	// check empty model
	if m.clickModelVersion != version.Version {
		return errors.New("model version mismatch")
	}
	// encode model
	reader, writer := io.Pipe()
	var encoderError error
	go func() {
		defer func(writer *io.PipeWriter) {
			err := writer.Close()
			if err != nil {
				base.Logger().Error("fail to close pipe", zap.Error(err))
			}
		}(writer)
		err := click.MarshalModel(writer, m.clickModel)
		if err != nil {
			base.Logger().Error("fail to marshal click model", zap.Error(err))
			encoderError = err
			return
		}
	}()
	// send model
	for {
		buf := make([]byte, batchSize)
		n, err := reader.Read(buf)
		if err == io.EOF {
			base.Logger().Debug("complete sending click model")
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

// GetUserIndex returns latest user index.
func (m *Master) GetUserIndex(version *protocol.VersionInfo, sender protocol.Master_GetUserIndexServer) error {
	m.userIndexMutex.RLock()
	defer m.userIndexMutex.RUnlock()
	// skip empty model
	if m.userIndex == nil {
		return errors.New("no valid index found")
	}
	// check empty model
	if m.userIndexVersion != version.Version {
		return errors.New("index version mismatch")
	}
	// encode model
	reader, writer := io.Pipe()
	var encoderError error
	go func() {
		defer func(writer *io.PipeWriter) {
			err := writer.Close()
			if err != nil {
				base.Logger().Error("fail to close pipe", zap.Error(err))
			}
		}(writer)
		err := base.MarshalIndex(writer, m.userIndex)
		if err != nil {
			base.Logger().Error("fail to marshal index", zap.Error(err))
			encoderError = err
			return
		}
	}()
	// send model
	for {
		buf := make([]byte, batchSize)
		n, err := reader.Read(buf)
		if err == io.EOF {
			base.Logger().Debug("complete sending user index")
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

// nodeUp handles node information inserted events.
func (m *Master) nodeUp(key string, value interface{}) {
	node := value.(*Node)
	base.Logger().Info("node up",
		zap.String("node_name", key),
		zap.String("node_ip", node.IP),
		zap.String("node_type", node.Type))
	m.nodesInfoMutex.Lock()
	defer m.nodesInfoMutex.Unlock()
	m.nodesInfo[key] = node
}

// nodeDown handles node information timout events.
func (m *Master) nodeDown(key string, value interface{}) {
	node := value.(*Node)
	base.Logger().Info("node down",
		zap.String("node_name", key),
		zap.String("node_ip", node.IP),
		zap.String("node_type", node.Type))
	m.nodesInfoMutex.Lock()
	defer m.nodesInfoMutex.Unlock()
	delete(m.nodesInfo, key)
}

func (m *Master) StartTask(
	_ context.Context,
	in *protocol.StartTaskRequest) (*protocol.StartTaskResponse, error) {
	m.taskMonitor.Start(in.Name, int(in.Total))
	return &protocol.StartTaskResponse{}, nil
}

func (m *Master) UpdateTask(
	_ context.Context,
	in *protocol.UpdateTaskRequest) (*protocol.UpdateTaskResponse, error) {
	m.taskMonitor.Update(in.Name, int(in.Done))
	return &protocol.UpdateTaskResponse{}, nil
}

func (m *Master) FinishTask(
	_ context.Context,
	in *protocol.FinishTaskRequest) (*protocol.FinishTaskResponse, error) {
	m.taskMonitor.Finish(in.Name)
	return &protocol.FinishTaskResponse{}, nil
}

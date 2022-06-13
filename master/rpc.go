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
	"github.com/zhenghaoz/gorse/base/task"
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
	Name          string
	Type          string
	IP            string
	HttpPort      int64
	BinaryVersion string
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
	node.BinaryVersion = nodeInfo.BinaryVersion
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
			log.Logger().Error("failed to set ttl cache", zap.Error(err))
			return nil, err
		}
	}
	// marshall config
	s, err := json.Marshal(m.Config)
	if err != nil {
		return nil, err
	}
	// save ranking model version
	m.rankingModelMutex.RLock()
	var rankingModelVersion int64
	if m.RankingModel != nil && !m.RankingModel.Invalid() {
		rankingModelVersion = m.RankingModelVersion
	}
	m.rankingModelMutex.RUnlock()
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
	if m.RankingModel == nil || m.RankingModel.Invalid() {
		return errors.New("no valid model found")
	}
	// check model version
	if m.RankingModelVersion != version.Version {
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
		err := ranking.MarshalModel(writer, m.RankingModel)
		if err != nil {
			log.Logger().Error("fail to marshal ranking model", zap.Error(err))
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
		err := click.MarshalModel(writer, m.ClickModel)
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

// nodeUp handles node information inserted events.
func (m *Master) nodeUp(key string, value interface{}) {
	node := value.(*Node)
	log.Logger().Info("node up",
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
	log.Logger().Info("node down",
		zap.String("node_name", key),
		zap.String("node_ip", node.IP),
		zap.String("node_type", node.Type))
	m.nodesInfoMutex.Lock()
	defer m.nodesInfoMutex.Unlock()
	delete(m.nodesInfo, key)
}

func (m *Master) PushTaskInfo(
	_ context.Context,
	in *protocol.PushTaskInfoRequest) (*protocol.PushTaskInfoResponse, error) {
	m.taskMonitor.TaskLock.Lock()
	defer m.taskMonitor.TaskLock.Unlock()
	m.taskMonitor.Tasks[in.GetName()] = task.NewTaskFromPB(in)
	return &protocol.PushTaskInfoResponse{}, nil
}

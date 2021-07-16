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
	"bufio"
	"bytes"
	"context"
	"encoding/gob"
	"encoding/json"
	"github.com/zhenghaoz/gorse/base"
	"github.com/zhenghaoz/gorse/model/click"
	"github.com/zhenghaoz/gorse/model/ranking"
	"github.com/zhenghaoz/gorse/protocol"
	"go.uber.org/zap"
	"google.golang.org/grpc/peer"
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
	if m.rankingModel != nil {
		rankingModelVersion = m.rankingModelVersion
	}
	m.rankingModelMutex.RUnlock()
	// save click model version
	m.clickModelMutex.RLock()
	var clickModelVersion int64
	if m.clickModel != nil {
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
func (m *Master) GetRankingModel(context.Context, *protocol.NodeInfo) (*protocol.Model, error) {
	m.rankingModelMutex.RLock()
	defer m.rankingModelMutex.RUnlock()
	// skip empty model
	if m.rankingModel.Invalid() {
		return &protocol.Model{Version: 0}, nil
	}
	// encode model
	modelData, err := ranking.EncodeModel(m.rankingModel)
	if err != nil {
		return nil, err
	}
	return &protocol.Model{
		Name:    m.rankingModelName,
		Version: m.rankingModelVersion,
		Model:   modelData,
	}, nil
}

// GetClickModel returns latest click model.
func (m *Master) GetClickModel(context.Context, *protocol.NodeInfo) (*protocol.Model, error) {
	m.clickModelMutex.RLock()
	defer m.clickModelMutex.RUnlock()
	// skip empty model
	if m.clickModel.Invalid() {
		return &protocol.Model{Version: 0}, nil
	}
	// encode model
	modelData, err := click.EncodeModel(m.clickModel)
	if err != nil {
		return nil, err
	}
	return &protocol.Model{
		Version: m.clickModelVersion,
		Model:   modelData,
	}, nil
}

// GetUserIndex returns latest user index.
func (m *Master) GetUserIndex(context.Context, *protocol.NodeInfo) (*protocol.UserIndex, error) {
	m.userIndexMutex.RLock()
	defer m.userIndexMutex.RUnlock()
	// skip empty model
	if m.userIndex == nil {
		return &protocol.UserIndex{Version: 0}, nil
	}
	// encode index
	buf := bytes.NewBuffer(nil)
	writer := bufio.NewWriter(buf)
	encoder := gob.NewEncoder(writer)
	if err := encoder.Encode(m.userIndex); err != nil {
		return nil, err
	}
	if err := writer.Flush(); err != nil {
		return nil, err
	}
	return &protocol.UserIndex{
		Version:   m.userIndexVersion,
		UserIndex: buf.Bytes(),
	}, nil
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

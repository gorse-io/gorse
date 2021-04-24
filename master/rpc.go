package master

import (
	"bufio"
	"bytes"
	"context"
	"encoding/gob"
	"encoding/json"
	"github.com/zhenghaoz/gorse/base"
	"github.com/zhenghaoz/gorse/model/pr"
	"github.com/zhenghaoz/gorse/protocol"
	"go.uber.org/zap"
	"google.golang.org/grpc/peer"
	"strings"
)

type Node struct {
	Name string
	Type string
	IP   string
	Http string
}

func NewNode(ctx context.Context, nodeInfo *protocol.NodeInfo) *Node {
	node := new(Node)
	node.Name = nodeInfo.NodeName
	node.Http = nodeInfo.HttpAddress
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

func (m *Master) GetMeta(ctx context.Context, nodeInfo *protocol.NodeInfo) (*protocol.Meta, error) {
	// save node
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
	// save user index version
	m.userIndexMutex.Lock()
	var userIndexVersion int64
	if m.userIndex != nil {
		userIndexVersion = m.userIndexVersion
	}
	m.userIndexMutex.Unlock()
	// save pr version
	m.prMutex.Lock()
	var prVersion int64
	if m.prModel != nil {
		prVersion = m.prVersion
	}
	m.prMutex.Unlock()
	// save fm version
	//m.fmMutex.Lock()
	//var fmVersion int64
	//if m.fmModel != nil {
	//	fmVersion = m.ctrVersion
	//}
	//m.fmMutex.Unlock()
	// collect nodes
	workers := make([]string, 0)
	servers := make([]string, 0)
	m.nodesInfoMutex.Lock()
	for name, info := range m.nodesInfo {
		switch info.Type {
		case WorkerNode:
			workers = append(workers, name)
		case ServerNode:
			servers = append(servers, name)
		}
	}
	m.nodesInfoMutex.Unlock()
	return &protocol.Meta{
		Config:           string(s),
		UserIndexVersion: userIndexVersion,
		//FmVersion:        fmVersion,
		PrVersion: prVersion,
		Me:        nodeInfo.NodeName,
		Workers:   workers,
		Servers:   servers,
	}, nil
}

func (m *Master) GetPRModel(context.Context, *protocol.NodeInfo) (*protocol.Model, error) {
	m.prMutex.Lock()
	defer m.prMutex.Unlock()
	// skip empty model
	if m.prModel == nil {
		return &protocol.Model{Version: 0}, nil
	}
	// encode model
	modelData, err := pr.EncodeModel(m.prModel)
	if err != nil {
		return nil, err
	}
	return &protocol.Model{
		Name:    m.prModelName,
		Version: m.prVersion,
		Model:   modelData,
	}, nil
}

//func (m *Master) GetFactorizationMachine(context.Context, *protocol.NodeInfo) (*protocol.Model, error) {
//	m.fmMutex.Lock()
//	defer m.fmMutex.Unlock()
//	// skip empty model
//	if m.fmModel == nil {
//		return &protocol.Model{Version: 0}, nil
//	}
//	// encode model
//	modelData, err := ctr.EncodeModel(m.fmModel)
//	if err != nil {
//		return nil, err
//	}
//	return &protocol.Model{
//		Version: m.ctrVersion,
//		Model:   modelData,
//	}, nil
//}

func (m *Master) GetUserIndex(context.Context, *protocol.NodeInfo) (*protocol.UserIndex, error) {
	m.userIndexMutex.Lock()
	defer m.userIndexMutex.Unlock()
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

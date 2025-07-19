// Copyright 2024 gorse Project Authors
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

package blob

import (
	"context"
	"github.com/juju/errors"
	"github.com/zhenghaoz/gorse/base/log"
	"github.com/zhenghaoz/gorse/protocol"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/timestamppb"
	"io"
	"os"
	"path"
	"time"
)

type Store interface {
	Open(name string) (io.ReadCloser, error)
	Create(name string) (io.WriteCloser, chan struct{}, error)
}

type MasterStoreServer struct {
	protocol.UnimplementedBlobStoreServer
	dir string
}

func NewMasterStoreServer(dir string) *MasterStoreServer {
	// Create directory if not exists
	err := os.MkdirAll(dir, os.ModePerm)
	if err != nil {
		log.Logger().Fatal("failed to create directory", zap.Error(err))
	}
	return &MasterStoreServer{dir: dir}
}

func (s *MasterStoreServer) UploadBlob(stream grpc.ClientStreamingServer[protocol.UploadBlobRequest, protocol.UploadBlobResponse]) error {
	// Create temp file
	file, err := os.CreateTemp(s.dir, "upload-*")
	if err != nil {
		return err
	}
	defer func(file *os.File) {
		_ = file.Close()
	}(file)
	// Write data
	var (
		name      string
		timestamp time.Time
	)
	for {
		req, err := stream.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return err
		}
		// Assign name
		if name == "" {
			name = req.Name
		} else if name != req.Name {
			return errors.New("inconsistent name")
		}
		// Assign timestamp
		if timestamp.IsZero() {
			timestamp = req.Timestamp.AsTime()
		} else if !timestamp.Equal(req.Timestamp.AsTime()) {
			return errors.New("inconsistent timestamp")
		}
		// Write data
		_, err = file.Write(req.Data)
		if err != nil {
			return err
		}
	}
	// Close file
	err = file.Close()
	if err != nil {
		return err
	}
	// Rename file
	err = os.Rename(file.Name(), path.Join(s.dir, name))
	if err != nil {
		return err
	}
	return stream.SendAndClose(&protocol.UploadBlobResponse{})
}

func (s *MasterStoreServer) FetchBlob(ctx context.Context, request *protocol.FetchBlobRequest) (*protocol.FetchBlobResponse, error) {
	fileInfo, err := os.Stat(path.Join(s.dir, request.Name))
	if err != nil {
		return nil, err
	}
	return &protocol.FetchBlobResponse{Timestamp: timestamppb.New(fileInfo.ModTime())}, nil
}

func (s *MasterStoreServer) DownloadBlob(request *protocol.DownloadBlobRequest, stream grpc.ServerStreamingServer[protocol.DownloadBlobResponse]) error {
	// Open file
	file, err := os.Open(path.Join(s.dir, request.Name))
	if err != nil {
		return err
	}
	defer func(file *os.File) {
		err = file.Close()
		if err != nil {
			log.Logger().Error("failed to close file", zap.Error(err))
		}
	}(file)
	// Send data
	for {
		data := make([]byte, 1024*1024*4)
		n, err := file.Read(data)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return err
		}
		err = stream.Send(&protocol.DownloadBlobResponse{Data: data[:n]})
		if err != nil {
			return err
		}
	}
	return nil
}

type MasterStoreClient struct {
	client protocol.BlobStoreClient
}

func NewMasterStoreClient(clientConn *grpc.ClientConn) *MasterStoreClient {
	return &MasterStoreClient{client: protocol.NewBlobStoreClient(clientConn)}
}

func (c *MasterStoreClient) Open(name string) (io.ReadCloser, error) {
	stream, err := c.client.DownloadBlob(context.Background(), &protocol.DownloadBlobRequest{Name: name})
	if err != nil {
		return nil, err
	}
	pr, pw := io.Pipe()
	go func() {
		for {
			resp, err := stream.Recv()
			if err != nil {
				_ = pw.CloseWithError(err)
				return
			}
			_, err = pw.Write(resp.Data)
			if err != nil {
				_ = pw.CloseWithError(err)
				return
			}
		}
	}()
	return pr, nil
}

func (c *MasterStoreClient) Create(name string) (io.WriteCloser, chan struct{}, error) {
	stream, err := c.client.UploadBlob(context.Background())
	if err != nil {
		return nil, nil, err
	}
	done := make(chan struct{})
	pr, pw := io.Pipe()
	go func() {
		defer close(done)
		for {
			data := make([]byte, 1024*1024*4)
			n, err := pr.Read(data)
			if err != nil {
				if errors.Is(err, io.EOF) {
					break
				}
				log.Logger().Error("failed to read data", zap.Error(err))
				return
			}
			err = stream.Send(&protocol.UploadBlobRequest{
				Name:      name,
				Timestamp: timestamppb.Now(),
				Data:      data[:n],
			})
			if err != nil {
				log.Logger().Error("failed to send data", zap.Error(err))
				return
			}
		}
		_, err = stream.CloseAndRecv()
		if err != nil {
			log.Logger().Error("failed to close stream", zap.Error(err))
		}
	}()
	return pw, done, nil
}

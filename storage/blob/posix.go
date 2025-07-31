// Copyright 2025 gorse Project Authors
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
	"io"
	"os"
	"path"

	"github.com/gorse-io/gorse/base/log"
	"go.uber.org/zap"
)

type POSIX struct {
	dir string
}

func NewPOSIX(dir string) *POSIX {
	return &POSIX{dir: dir}
}

// Open a file for reading. It returns an io.Reader that can be used to read the file's content.
func (p *POSIX) Open(name string) (io.ReadCloser, error) {
	fullPath := path.Join(p.dir, name)
	return os.Open(fullPath)
}

// Create a new file for writing. It returns an io.WriteCloser that can be used to write data to the file. It also
// returns a done channel that is closed when the writing is complete.
func (p *POSIX) Create(name string) (io.WriteCloser, chan struct{}, error) {
	fullPath := path.Join(p.dir, name)
	if err := os.MkdirAll(path.Dir(fullPath), os.ModePerm); err != nil {
		return nil, nil, err
	}
	file, err := os.Create(fullPath)
	if err != nil {
		return nil, nil, err
	}
	done := make(chan struct{})
	pr, pw := io.Pipe()
	go func() {
		defer func() {
			_ = file.Close()
			close(done)
		}()
		_, err := io.Copy(file, pr)
		if err != nil {
			log.Logger().Error("failed to write to file", zap.String("file", fullPath), zap.Error(err))
		}
	}()
	return pw, done, err
}

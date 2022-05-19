// Copyright 2022 gorse Project Authors
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

package log

import (
	"github.com/emicklei/go-restful/v3"
	"github.com/go-sql-driver/mysql"
	"go.uber.org/zap"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

var logger *zap.Logger

func init() {
	SetProductionLogger()
}

// Logger get current logger
func Logger() *zap.Logger {
	return logger
}

func ResponseLogger(resp *restful.Response) *zap.Logger {
	return logger.With(zap.String("request_id", resp.Header().Get("X-Request-ID")))
}

// SetProductionLogger set current logger in production mode.
func SetProductionLogger(outputPaths ...string) {
	for _, outputPath := range outputPaths {
		err := os.MkdirAll(filepath.Dir(outputPath), os.ModePerm)
		if err != nil {
			panic(err)
		}
	}
	cfg := zap.NewProductionConfig()
	cfg.OutputPaths = append(cfg.OutputPaths, outputPaths...)
	var err error
	logger, err = cfg.Build()
	if err != nil {
		panic(err)
	}
}

// SetDevelopmentLogger set current logger in development mode.
func SetDevelopmentLogger(outputPaths ...string) {
	for _, outputPath := range outputPaths {
		err := os.MkdirAll(filepath.Dir(outputPath), os.ModePerm)
		if err != nil {
			panic(err)
		}
	}
	cfg := zap.NewDevelopmentConfig()
	cfg.OutputPaths = append(cfg.OutputPaths, outputPaths...)
	var err error
	logger, err = cfg.Build()
	if err != nil {
		panic(err)
	}
}

func CloseLogger() {
	cfg := zap.NewProductionConfig()
	cfg.Level = zap.NewAtomicLevelAt(zap.FatalLevel)
	var err error
	logger, err = cfg.Build()
	if err != nil {
		panic(err)
	}
}

const mysqlPrefix = "mysql://"

func RedactDBURL(rawURL string) string {
	if strings.HasPrefix(rawURL, mysqlPrefix) {
		parsed, err := mysql.ParseDSN(rawURL[len(mysqlPrefix):])
		if err != nil {
			return rawURL
		}
		parsed.User = strings.Repeat("x", len(parsed.User))
		parsed.Passwd = strings.Repeat("x", len(parsed.Passwd))
		return mysqlPrefix + parsed.FormatDSN()
	} else {
		parsed, err := url.Parse(rawURL)
		if err != nil {
			return rawURL
		}
		username := parsed.User.Username()
		password, _ := parsed.User.Password()
		parsed.User = url.UserPassword(strings.Repeat("x", len(username)), strings.Repeat("x", len(password)))
		return parsed.String()
	}
}

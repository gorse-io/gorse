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
	"net/url"
	"os"
	"runtime"
	"strings"
	"testing"

	"github.com/emicklei/go-restful/v3"
	"github.com/go-sql-driver/mysql"
	"github.com/spf13/pflag"
	"go.opentelemetry.io/otel"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

var (
	logger       *zap.Logger
	openaiLogger *zap.Logger
)

func init() {
	// setup default logger
	var err error
	logger, err = zap.NewProduction()
	if err != nil {
		panic(err)
	}
	// setup OpenAI logger
	if testing.Testing() {
		openaiLogger, err = zap.NewDevelopment()
		if err != nil {
			panic(err)
		}
	} else {
		openaiLogger = zap.NewNop()
	}
	// Windows file sink support: https://github.com/uber-go/zap/issues/621
	if runtime.GOOS == "windows" {
		if err := zap.RegisterSink("windows", func(u *url.URL) (zap.Sink, error) {
			// Remove leading slash left by url.Parse()
			return os.OpenFile(u.Path[1:], os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)
		}); err != nil {
			logger.Fatal("failed to register Windows file sink", zap.Error(err))
		}
	}
}

// Logger get current logger
func Logger() *zap.Logger {
	return logger
}

func ResponseLogger(resp *restful.Response) *zap.Logger {
	return logger.With(zap.String("request_id", resp.Header().Get("X-Request-ID")))
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

func AddFlags(flagSet *pflag.FlagSet) {
	flagSet.String("log-path", "", "path of log file")
	flagSet.Int("log-max-size", 100, "maximum size in megabytes of the log file")
	flagSet.Int("log-max-age", 0, "maximum number of days to retain old log files")
	flagSet.Int("log-max-backups", 0, "maximum number of old log files to retain")
}

func SetLogger(flagSet *pflag.FlagSet, debug bool) {
	// enable or disable debug mode
	var (
		encoder zapcore.Encoder
		level   zapcore.LevelEnabler
	)
	if debug {
		encoder = zapcore.NewConsoleEncoder(zap.NewDevelopmentEncoderConfig())
		level = zap.DebugLevel
	} else {
		encoder = zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig())
		level = zap.InfoLevel
	}
	// create lumberjack logger
	writers := []zapcore.WriteSyncer{zapcore.AddSync(os.Stdout)}
	if flagSet.Changed("log-path") {
		path, _ := flagSet.GetString("log-path")
		maxSize, _ := flagSet.GetInt("log-max-size")
		maxAge, _ := flagSet.GetInt("log-max-age")
		maxBackups, _ := flagSet.GetInt("log-max-backups")
		writers = append(writers, zapcore.AddSync(&lumberjack.Logger{
			Filename:   path,
			MaxSize:    maxSize,
			MaxBackups: maxBackups,
			MaxAge:     maxAge,
			Compress:   false,
		}))
	}
	// create zap logger
	core := zapcore.NewCore(encoder, zap.CombineWriteSyncers(writers...), level)
	logger = zap.New(core)
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

func GetErrorHandler() otel.ErrorHandler {
	return &errorHandler{}
}

type errorHandler struct{}

func (h *errorHandler) Handle(err error) {
	Logger().Error("opentelemetry failure", zap.Error(err))
}

func OpenAILogger() *zap.Logger {
	return openaiLogger
}

func InitOpenAILogger(filename string) {
	if filename != "" {
		openaiLogger = zap.New(
			zapcore.NewCore(
				zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig()),
				zapcore.AddSync(&lumberjack.Logger{
					Filename: filename,
				}),
				zap.InfoLevel))
	}
}

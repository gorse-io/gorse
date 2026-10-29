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
	"sync"
	"testing"
	"time"

	"github.com/c-bata/goptuna"
	"github.com/emicklei/go-restful/v3"
	"github.com/go-sql-driver/mysql"
	"github.com/spf13/pflag"
	"go.opentelemetry.io/otel"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest"
	"gopkg.in/natefinch/lumberjack.v2"
)

var (
	logger       *zap.Logger
	openaiLogger *zap.Logger
	accessLogger *zap.Logger
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

// EventHandler records named events with dynamic fields.
// Callbacks may run concurrently and must not modify field values.
type EventHandler func(name string, fields ...zap.Field)

var (
	eventHandlerMu sync.RWMutex
	eventHandler   EventHandler
)

// SetEventHandler replaces the event handler. A nil handler disables events.
// Events already submitted continue to use their original handler.
func SetEventHandler(handler EventHandler) {
	eventHandlerMu.Lock()
	defer eventHandlerMu.Unlock()
	eventHandler = handler
}

// Event asynchronously records an event without writing to the application log.
// A timestamp field is added automatically at submission time.
// The field slice is copied, but referenced objects must remain immutable until
// the handler finishes.
func Event(name string, fields ...zap.Field) {
	eventHandlerMu.RLock()
	handler := eventHandler
	eventHandlerMu.RUnlock()
	if handler != nil {
		eventFields := make([]zap.Field, len(fields)+1)
		copy(eventFields, fields)
		eventFields[len(fields)] = zap.Time("timestamp", time.Now())
		go handler(name, eventFields...)
	}
}

// Logger get current logger
func Logger() *zap.Logger {
	return logger
}

func ResponseLogger(resp *restful.Response) *zap.Logger {
	return logger.With(zap.String("request_id", resp.Header().Get("X-Request-ID")))
}

// AccessLogger returns the access logger for HTTP request logging
func AccessLogger() *zap.Logger {
	if accessLogger != nil {
		return accessLogger
	}
	return logger
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

// SetTestLogger redirects Gorse logs to t.Log until the test finishes.
func SetTestLogger(t testing.TB) {
	t.Helper()
	oldLogger := logger
	oldOpenAILogger := openaiLogger
	oldAccessLogger := accessLogger

	testLogger := zaptest.NewLogger(t, zaptest.Level(zap.DebugLevel))
	logger = testLogger
	openaiLogger = testLogger
	accessLogger = testLogger

	t.Cleanup(func() {
		logger = oldLogger
		openaiLogger = oldOpenAILogger
		accessLogger = oldAccessLogger
	})
}

func AddFlags(flagSet *pflag.FlagSet) {
	flagSet.String("log-path", "", "path of log file")
	flagSet.String("access-log-path", "", "path of access log file for RESTful API")
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

	// setup access logger if access-log-path is set
	if accessLogPath, _ := flagSet.GetString("access-log-path"); accessLogPath != "" {
		maxSize, _ := flagSet.GetInt("log-max-size")
		maxAge, _ := flagSet.GetInt("log-max-age")
		maxBackups, _ := flagSet.GetInt("log-max-backups")
		accessLogger = zap.New(
			zapcore.NewCore(
				zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig()),
				zapcore.AddSync(&lumberjack.Logger{
					Filename:   accessLogPath,
					MaxSize:    maxSize,
					MaxBackups: maxBackups,
					MaxAge:     maxAge,
					Compress:   false,
				}),
				zap.InfoLevel))
	}
}

const mysqlPrefix = "mysql://"

// RedactSecret preserves the first three and last four characters of long secrets.
// Secrets of seven characters or fewer are masked entirely; empty values stay empty.
func RedactSecret(value string) string {
	if value == "" {
		return ""
	}
	characters := []rune(value)
	if len(characters) <= 7 {
		return "******"
	}
	return string(characters[:3]) + "******" + string(characters[len(characters)-4:])
}

// RedactDBURL masks credentials while preserving database addresses and options.
// Invalid connection strings return an empty string to avoid leaking credentials.
func RedactDBURL(rawURL string) string {
	if rawURL == "" {
		return rawURL
	}
	if strings.HasPrefix(rawURL, mysqlPrefix) {
		parsed, err := mysql.ParseDSN(rawURL[len(mysqlPrefix):])
		if err != nil {
			return ""
		}
		parsed.User = RedactSecret(parsed.User)
		parsed.Passwd = RedactSecret(parsed.Passwd)
		for key, value := range parsed.Params {
			decodedKey, err := url.QueryUnescape(key)
			if err != nil {
				return ""
			}
			if isDBCredentialKey(decodedKey) {
				parsed.Params[key] = RedactSecret(value)
			}
		}
		return strings.ReplaceAll(mysqlPrefix+parsed.FormatDSN(), "%2A", "*")
	}
	// SQLite and Xvec use filesystem paths; preserve them verbatim.
	if strings.HasPrefix(rawURL, "sqlite://") || strings.HasPrefix(rawURL, "xvec://") {
		return rawURL
	}
	// net/url does not support MongoDB authorities containing multiple IPv6 hosts.
	// Parse each host separately, then restore the list after parsing the URL.
	var mongoHosts []string
	if strings.HasPrefix(rawURL, "mongodb://") || strings.HasPrefix(rawURL, "mongodb+srv://") {
		prefix, rest, _ := strings.Cut(rawURL, "://")
		end := strings.IndexAny(rest, "/?#")
		if end < 0 {
			end = len(rest)
		}
		authority := rest[:end]
		hostStart := strings.LastIndex(authority, "@") + 1
		if strings.Contains(authority[hostStart:], ",") {
			mongoHosts = strings.Split(authority[hostStart:], ",")
			for i, host := range mongoHosts {
				parsed, err := url.Parse(prefix + "://" + host)
				if err != nil || parsed.Host == "" || parsed.User != nil {
					return ""
				}
				mongoHosts[i] = parsed.Host
			}
			rawURL = prefix + "://" + authority[:hostStart] + "localhost" + rest[end:]
		}
	}
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.Opaque != "" {
		return ""
	}
	if len(mongoHosts) > 0 {
		parsed.Host = strings.Join(mongoHosts, ",")
	}
	if parsed.User != nil {
		username := parsed.User.Username()
		password, hasPassword := parsed.User.Password()
		if hasPassword {
			parsed.User = url.UserPassword(RedactSecret(username), RedactSecret(password))
		} else {
			parsed.User = url.User(RedactSecret(username))
		}
	}
	// Database connection strings do not use fragments for connection options.
	if parsed.Fragment != "" {
		parsed.Fragment = "********"
		parsed.RawFragment = ""
	}
	query, err := url.ParseQuery(parsed.RawQuery)
	if err != nil {
		return ""
	}
	changed := parsed.Fragment != ""
	for key, values := range query {
		if !isDBCredentialKey(key) {
			continue
		}
		for i, value := range values {
			if value != "" {
				values[i] = RedactSecret(value)
				changed = true
			}
		}
	}
	if changed {
		parsed.RawQuery = query.Encode()
	}
	return strings.ReplaceAll(parsed.String(), "%2A", "*")
}

// isDBCredentialKey matches documented connection credentials and SDK auth fields.
// The vector database SDK fields below are sensitive when present, although
// Gorse's current vector adapters do not read authentication from URL queries.
func isDBCredentialKey(key string) bool {
	key = strings.NewReplacer("-", "", "_", "", ".", "").Replace(strings.ToLower(key))
	switch key {
	case "user", // PostgreSQL: database username (lib/pq Config.User).
		"password",                        // PostgreSQL: database password (lib/pq Config.Password).
		"passfile",                        // PostgreSQL: password file (lib/pq Config.Passfile).
		"sslkey",                          // PostgreSQL: client private key path or inline PEM (lib/pq Config.SSLKey).
		"sslpassword",                     // PostgreSQL/libpq: client private key password.
		"tlscertificatekeyfile",           // MongoDB: client certificate/private key file.
		"sslclientcertificatekeyfile",     // MongoDB: alias for tlsCertificateKeyFile.
		"tlsprivatekeyfile",               // MongoDB: separate client private key file.
		"tlscertificatekeyfilepassword",   // MongoDB: client private key password.
		"sslclientcertificatekeypassword", // MongoDB: alias for tlsCertificateKeyFilePassword.
		"authmechanismproperties",         // MongoDB: authentication properties, including AWS_SESSION_TOKEN.
		"username",                        // Weaviate/Milvus SDKs: database username.
		"apikey",                          // Qdrant/Weaviate/Milvus SDKs: API key.
		"clientsecret",                    // Weaviate SDK: OAuth client secret.
		"accesstoken",                     // Weaviate SDK: OAuth access token.
		"refreshtoken":                    // Weaviate SDK: OAuth refresh token.
		return true
	}
	return false
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

type OptunaLogger struct {
	logger *zap.Logger
}

func NewOptunaLogger(logger *zap.Logger) goptuna.Logger {
	return &OptunaLogger{logger: logger}
}

func (o OptunaLogger) Debug(msg string, fields ...any) {
	o.logger.Debug(msg, zap.Any("fields", fields))
}

func (o OptunaLogger) Info(msg string, fields ...any) {
	o.logger.Info(msg, zap.Any("fields", fields))
}

func (o OptunaLogger) Warn(msg string, fields ...any) {
	o.logger.Warn(msg, zap.Any("fields", fields))
}

func (o OptunaLogger) Error(msg string, fields ...any) {
	o.logger.Error(msg, zap.Any("fields", fields))
}

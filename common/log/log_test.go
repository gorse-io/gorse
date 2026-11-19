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
	"os"
	"sync"
	"testing"
	"time"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestSetDevelopmentLogger(t *testing.T) {
	temp, err := os.MkdirTemp("", "gorse")
	assert.NoError(t, err)
	flagSet := pflag.NewFlagSet("test", pflag.ContinueOnError)
	AddFlags(flagSet)
	// set existed path
	err = flagSet.Set("log-path", temp+"/gorse.log")
	assert.NoError(t, err)
	SetLogger(flagSet, true)
	Logger().Debug("test")
	assert.FileExists(t, temp+"/gorse.log")
	// set non-existed path
	err = flagSet.Set("log-path", temp+"/gorse/gorse.log")
	assert.NoError(t, err)
	SetLogger(flagSet, true)
	Logger().Debug("test")
	assert.FileExists(t, temp+"/gorse/gorse.log")
}

func TestSetProductionLogger(t *testing.T) {
	temp, err := os.MkdirTemp("", "gorse")
	assert.NoError(t, err)
	flagSet := pflag.NewFlagSet("test", pflag.ContinueOnError)
	AddFlags(flagSet)
	// set existed path
	err = flagSet.Set("log-path", temp+"/gorse.log")
	assert.NoError(t, err)
	SetLogger(flagSet, false)
	Logger().Info("test")
	assert.FileExists(t, temp+"/gorse.log")
	// set non-existed path
	err = flagSet.Set("log-path", temp+"/gorse/gorse.log")
	assert.NoError(t, err)
	SetLogger(flagSet, false)
	Logger().Info("test")
	assert.FileExists(t, temp+"/gorse/gorse.log")
}

func TestSetTestLogger(t *testing.T) {
	oldLogger := Logger()
	oldOpenAILogger := OpenAILogger()
	oldAccessLogger := accessLogger

	t.Run("redirect", func(t *testing.T) {
		SetTestLogger(t)

		assert.NotSame(t, oldLogger, Logger())
		assert.NotSame(t, oldOpenAILogger, OpenAILogger())
		assert.NotSame(t, oldAccessLogger, accessLogger)
		Logger().Info("test")
		OpenAILogger().Info("test")
		AccessLogger().Info("test")
	})

	assert.Same(t, oldLogger, Logger())
	assert.Same(t, oldOpenAILogger, OpenAILogger())
	assert.Same(t, oldAccessLogger, accessLogger)
}

func TestRedactSecret(t *testing.T) {
	assert.Empty(t, RedactSecret(""))
	assert.Equal(t, "******", RedactSecret("a"))
	assert.Equal(t, "******", RedactSecret("abcdefg"))
	assert.Equal(t, "abc******efgh", RedactSecret("abcdefgh"))
	assert.Equal(t, "abc******ijkl", RedactSecret("abcdefghijkl"))
	assert.Equal(t, "******", RedactSecret("甲乙丙丁戊己庚"))
	assert.Equal(t, "甲乙丙******戊己庚辛", RedactSecret("甲乙丙丁戊己庚辛"))

	assert.Equal(t, "******", RedactSecret("******"))
	assert.Equal(t, "abc******efgh", RedactSecret("abc******efgh"))
	assert.Equal(t, "abc******ijkl", RedactSecret("abc******ijkl"))
	assert.Equal(t, "甲乙丙******戊己庚辛", RedactSecret("甲乙丙******戊己庚辛"))
}

func TestRedactDBURL(t *testing.T) {
	assert.Empty(t, RedactDBURL(""))
	assert.Equal(t, "sqlite://data/data.sqlite",
		RedactDBURL("sqlite://data/data.sqlite"))
	assert.Equal(t, "sqlite://:memory:",
		RedactDBURL("sqlite://:memory:"))
	assert.Equal(t, `sqlite://C:\data\gorse.db?mode=rwc`,
		RedactDBURL(`sqlite://C:\data\gorse.db?mode=rwc`))
	assert.Equal(t, "xvec://data/vectors",
		RedactDBURL("xvec://data/vectors"))
	assert.Equal(t, `xvec://C:\data\vectors`,
		RedactDBURL(`xvec://C:\data\vectors`))
	assert.Equal(t, "mysql://******:gor******pass@tcp(localhost:3306)/gorse?parseTime=true",
		RedactDBURL("mysql://gorse:gorse_pass@tcp(localhost:3306)/gorse?parseTime=true"))
	assert.Equal(t, "mysql://******:******@unix(/var/run/mysql.sock)/gorse?parseTime=true",
		RedactDBURL("mysql://bob:secret@unix(/var/run/mysql.sock)/gorse?parseTime=true"))
	assert.Equal(t, "mysql://******:p?@******#&ss@tcp(localhost:3306)/gorse",
		RedactDBURL("mysql://bob:p?@/#&ss@tcp(localhost:3306)/gorse"))
	assert.Equal(t, "mysql://******:******@tcp(localhost:3306)/gorse?parseTime=true&api_key=******&password=******",
		RedactDBURL("mysql://bob:secret@tcp(localhost:3306)/gorse?password=secret&api_key=secret&parseTime=true"))
	assert.Equal(t, "mysql://******:******@tcp(localhost:3306)/gorse?connectionAttributes=token%3Asecret&parseTime=true",
		RedactDBURL("mysql://bob:secret@tcp(localhost:3306)/gorse?connectionAttributes=token%3Asecret&parseTime=true"))
	assert.Equal(t, "mysql://******:******@tcp(localhost:3306)/gorse?allowCleartextPasswords=true&allowNativePasswords=false&allowOldPasswords=true",
		RedactDBURL("mysql://bob:secret@tcp(localhost:3306)/gorse?allowCleartextPasswords=true&allowNativePasswords=false&allowOldPasswords=true"))
	assert.Equal(t, "postgres://******:******@1.2.3.4:5432/mydb?sslmode=verify-full",
		RedactDBURL("postgres://bob:secret@1.2.3.4:5432/mydb?sslmode=verify-full"))
	assert.Equal(t, "postgresql://localhost/gorse?password=******&sslmode=require&user=******",
		RedactDBURL("postgresql://localhost/gorse?user=bob&password=secret&sslmode=require"))
	assert.Equal(t, "postgres://localhost/gorse?passfile=%2Fet******pass&sslkey=%2Fet******.key",
		RedactDBURL("postgres://localhost/gorse?passfile=/etc/pgpass&sslkey=/etc/client.key"))
	assert.Equal(t, "postgres://localhost/gorse?client_id=frontend&credential_provider=default&password_policy=strict&tokenizer=standard&uid=report&user_id=operator",
		RedactDBURL("postgres://localhost/gorse?client_id=frontend&credential_provider=default&password_policy=strict&tokenizer=standard&uid=report&user_id=operator"))
	assert.Equal(t, "mongodb://******:******@host1:27017,host2:27018/gorse?replicaSet=rs0&authSource=admin",
		RedactDBURL("mongodb://bob:secret@host1:27017,host2:27018/gorse?replicaSet=rs0&authSource=admin"))
	assert.Equal(t, "mongodb://******:******@[::1]:27017,[::2]:27018/gorse?replicaSet=rs0",
		RedactDBURL("mongodb://bob:secret@[::1]:27017,[::2]:27018/gorse?replicaSet=rs0"))
	assert.Equal(t, "mongodb+srv://******:******@cluster.example/gorse?authMechanism=MONGODB-AWS&authMechanismProperties=AWS******cret",
		RedactDBURL("mongodb+srv://bob:secret@cluster.example/gorse?authMechanism=MONGODB-AWS&authMechanismProperties=AWS_SESSION_TOKEN%3Asecret"))
	assert.Equal(t, "mongodb://localhost/gorse?tls=true&tlsCertificateKeyFilePassword=******",
		RedactDBURL("mongodb://localhost/gorse?tls=true&tlsCertificateKeyFilePassword=secret"))
	assert.Equal(t, "mongodb://localhost/gorse?tls=true&tlsPrivateKeyFile=%2Fet******.key",
		RedactDBURL("mongodb://localhost/gorse?tls=true&tlsPrivateKeyFile=/etc/client.key"))
	assert.Equal(t, "rediss://:******@localhost:6379/0?protocol=3",
		RedactDBURL("rediss://:secret@localhost:6379/0?protocol=3"))
	assert.Equal(t, "redis+cluster://******:******@host1:6379?addr=host2:6379&addr=host3:6379",
		RedactDBURL("redis+cluster://bob:secret@host1:6379?addr=host2:6379&addr=host3:6379"))
	assert.Equal(t, "redis://******@localhost:6379/0",
		RedactDBURL("redis://bob@localhost:6379/0"))
	assert.Equal(t, "postgres://******:******@[::1]:5432/gorse",
		RedactDBURL("postgres://bob:secret@[::1]:5432/gorse"))
	assert.Equal(t, "postgres://******:******@localhost/gorse",
		RedactDBURL("postgres://b%40b:s%3Acr%40t@localhost/gorse"))
	assert.Equal(t, "postgres://dat******user:dat******word@localhost/gorse",
		RedactDBURL("postgres://data-user:data-password@localhost/gorse"))
	assert.Equal(t, "postgres://a%40b******defg:x%3Ay******cdef@localhost/gorse",
		RedactDBURL("postgres://a%40bcdefg:x%3Ayabcdef@localhost/gorse"))
	assert.Equal(t, "qdrant://localhost:6334?api_key=******&api_key=******&timeout=5",
		RedactDBURL("qdrant://localhost:6334?api_key=first&api_key=second&timeout=5"))
	assert.Equal(t, "qdrant://localhost:6334?api_key=******&timeout=5",
		RedactDBURL("qdrant://localhost:6334?%61pi_key=s%26cr%3Dt&timeout=5"))
	assert.Equal(t, "qdrant://localhost:6334/db%2Fname?api_key=a%26b******efgh&note=a%26b%3Dc%2B",
		RedactDBURL("qdrant://localhost:6334/db%2Fname?api_key=a%26bcdefgh&note=a%26b%3Dc%2B"))
	assert.Equal(t, "weaviates://localhost?access_token=abc******hijk&client_secret=abc******hijk&refresh_token=abc******hijk",
		RedactDBURL("weaviates://localhost?access_token=abcdefghijk&client_secret=abcdefghijk&refresh_token=abcdefghijk"))
	assert.Equal(t, "postgres://localhost/gorse?user=&password=&sslmode=require",
		RedactDBURL("postgres://localhost/gorse?user=&password=&sslmode=require"))
	assert.Equal(t, "postgres://localhost/gorse?password=******#********",
		RedactDBURL("postgres://localhost/gorse?password=secret#token=secret"))
	assert.Equal(t, "postgres://localhost/gorse#********",
		RedactDBURL("postgres://localhost/gorse#token=secret"))
	assert.Empty(t, RedactDBURL("mysql://gorse:gorse_pass@tcp(localhost:3306) gorse?parseTime=true"))
	assert.Empty(t, RedactDBURL("postgres://bob:secret@1.2.3.4:5432 mydb?sslmode=verify-full"))
	assert.Empty(t, RedactDBURL("redis://bob:%zz@localhost:6379/0"))
	assert.Empty(t, RedactDBURL("postgres://bob:secret@localhost/%zz"))
	assert.Empty(t, RedactDBURL("postgres://localhost/gorse?password=%zz&user=bob"))
	assert.Empty(t, RedactDBURL("postgres://localhost/gorse?password=secret;user=bob"))
	assert.Empty(t, RedactDBURL("mysql://bob:secret@tcp(localhost:3306)/gorse?password=%zz"))
	assert.Empty(t, RedactDBURL("bob:secret@localhost/gorse"))
	assert.Empty(t, RedactDBURL("postgres:///gorse?password=secret"))
	assert.Empty(t, RedactDBURL("mysql://bob:secret@tcp(localhost:3306)/gorse?%zz=secret"))
	assert.Empty(t, RedactDBURL("mongodb://bob:secret@host1:27017,,host2:27018/gorse"))
	assert.Empty(t, RedactDBURL("********"))

	// Already-redacted URLs remain unchanged.
	assert.Equal(t, "mysql://******:gor******pass@tcp(localhost:3306)/gorse?parseTime=true",
		RedactDBURL("mysql://******:gor******pass@tcp(localhost:3306)/gorse?parseTime=true"))
	assert.Equal(t, "mysql://******:******@unix(/var/run/mysql.sock)/gorse?parseTime=true",
		RedactDBURL("mysql://******:******@unix(/var/run/mysql.sock)/gorse?parseTime=true"))
	assert.Equal(t, "mysql://******:p?@******#&ss@tcp(localhost:3306)/gorse",
		RedactDBURL("mysql://******:p?@******#&ss@tcp(localhost:3306)/gorse"))
	assert.Equal(t, "mysql://******:******@tcp(localhost:3306)/gorse?parseTime=true&api_key=******&password=******",
		RedactDBURL("mysql://******:******@tcp(localhost:3306)/gorse?parseTime=true&api_key=******&password=******"))
	assert.Equal(t, "mysql://******:******@tcp(localhost:3306)/gorse?connectionAttributes=token%3Asecret&parseTime=true",
		RedactDBURL("mysql://******:******@tcp(localhost:3306)/gorse?connectionAttributes=token%3Asecret&parseTime=true"))
	assert.Equal(t, "mysql://******:******@tcp(localhost:3306)/gorse?allowCleartextPasswords=true&allowNativePasswords=false&allowOldPasswords=true",
		RedactDBURL("mysql://******:******@tcp(localhost:3306)/gorse?allowCleartextPasswords=true&allowNativePasswords=false&allowOldPasswords=true"))
	assert.Equal(t, "postgres://******:******@1.2.3.4:5432/mydb?sslmode=verify-full",
		RedactDBURL("postgres://******:******@1.2.3.4:5432/mydb?sslmode=verify-full"))
	assert.Equal(t, "postgresql://localhost/gorse?password=******&sslmode=require&user=******",
		RedactDBURL("postgresql://localhost/gorse?password=******&sslmode=require&user=******"))
	assert.Equal(t, "mongodb://******:******@host1:27017,host2:27018/gorse?replicaSet=rs0&authSource=admin",
		RedactDBURL("mongodb://******:******@host1:27017,host2:27018/gorse?replicaSet=rs0&authSource=admin"))
	assert.Equal(t, "mongodb://******:******@[::1]:27017,[::2]:27018/gorse?replicaSet=rs0",
		RedactDBURL("mongodb://******:******@[::1]:27017,[::2]:27018/gorse?replicaSet=rs0"))
	assert.Equal(t, "mongodb+srv://******:******@cluster.example/gorse?authMechanism=MONGODB-AWS&authMechanismProperties=AWS******cret",
		RedactDBURL("mongodb+srv://******:******@cluster.example/gorse?authMechanism=MONGODB-AWS&authMechanismProperties=AWS******cret"))
	assert.Equal(t, "mongodb://localhost/gorse?tls=true&tlsCertificateKeyFilePassword=******",
		RedactDBURL("mongodb://localhost/gorse?tls=true&tlsCertificateKeyFilePassword=******"))
	assert.Equal(t, "rediss://:******@localhost:6379/0?protocol=3",
		RedactDBURL("rediss://:******@localhost:6379/0?protocol=3"))
	assert.Equal(t, "redis+cluster://******:******@host1:6379?addr=host2:6379&addr=host3:6379",
		RedactDBURL("redis+cluster://******:******@host1:6379?addr=host2:6379&addr=host3:6379"))
	assert.Equal(t, "redis://******@localhost:6379/0",
		RedactDBURL("redis://******@localhost:6379/0"))
	assert.Equal(t, "postgres://******:******@[::1]:5432/gorse",
		RedactDBURL("postgres://******:******@[::1]:5432/gorse"))
	assert.Equal(t, "postgres://******:******@localhost/gorse",
		RedactDBURL("postgres://******:******@localhost/gorse"))
	assert.Equal(t, "postgres://dat******user:dat******word@localhost/gorse",
		RedactDBURL("postgres://dat******user:dat******word@localhost/gorse"))
	assert.Equal(t, "postgres://a%40b******defg:x%3Ay******cdef@localhost/gorse",
		RedactDBURL("postgres://a%40b******defg:x%3Ay******cdef@localhost/gorse"))
	assert.Equal(t, "qdrant://localhost:6334?api_key=******&api_key=******&timeout=5",
		RedactDBURL("qdrant://localhost:6334?api_key=******&api_key=******&timeout=5"))
	assert.Equal(t, "qdrant://localhost:6334?api_key=******&timeout=5",
		RedactDBURL("qdrant://localhost:6334?api_key=******&timeout=5"))
	assert.Equal(t, "qdrant://localhost:6334/db%2Fname?api_key=a%26b******efgh&note=a%26b%3Dc%2B",
		RedactDBURL("qdrant://localhost:6334/db%2Fname?api_key=a%26b******efgh&note=a%26b%3Dc%2B"))
	assert.Equal(t, "postgres://localhost/gorse?password=******#********",
		RedactDBURL("postgres://localhost/gorse?password=******#********"))
	assert.Equal(t, "postgres://localhost/gorse#********",
		RedactDBURL("postgres://localhost/gorse#********"))
}

func TestEvent(t *testing.T) {
	t.Cleanup(func() { SetEventHandler(nil) })
	type event struct {
		name      string
		timestamp time.Time
		fields    map[string]any
	}
	events := make(chan event, 1)
	SetEventHandler(func(name string, timestamp time.Time, fields map[string]any) {
		events <- event{name: name, timestamp: timestamp, fields: fields}
	})
	now := time.Now()
	fields := []zap.Field{
		zap.String("custom", "value"),
		zap.Int64("bytes", 123),
		zap.Duration("duration", time.Millisecond),
		zap.Time("custom_time", now),
		zap.String("timestamp", "custom timestamp"),
	}
	Event("custom_event", fields...)
	after := time.Now()
	fields[0] = zap.String("custom", "changed")
	select {
	case got := <-events:
		require.Equal(t, "custom_event", got.name)
		require.Equal(t, map[string]any{
			"custom":      "value",
			"bytes":       int64(123),
			"duration":    time.Millisecond,
			"custom_time": now.Round(0),
			"timestamp":   "custom timestamp",
		}, got.fields)
		require.False(t, got.timestamp.Before(now))
		require.False(t, got.timestamp.After(after))
	case <-time.After(time.Second):
		t.Fatal("event was not emitted")
	}
	SetEventHandler(nil)
	require.NotPanics(t, func() { Event("ignored") })
}

func TestEventConcurrentDispatch(t *testing.T) {
	t.Cleanup(func() { SetEventHandler(nil) })
	const workers, count = 4, 100
	var delivered, producers sync.WaitGroup
	delivered.Add(workers * count)
	SetEventHandler(func(string, time.Time, map[string]any) { delivered.Done() })
	for range workers {
		producers.Go(func() {
			for range count {
				Event("concurrent")
			}
		})
	}
	producers.Wait()
	done := make(chan struct{})
	go func() {
		delivered.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("events were not emitted")
	}
}

func TestEventBindsHandlerBeforeDispatch(t *testing.T) {
	t.Cleanup(func() { SetEventHandler(nil) })
	const count = 100
	var delivered sync.WaitGroup
	delivered.Add(count)
	wrongHandler := make(chan struct{}, count)
	for range count {
		SetEventHandler(func(string, time.Time, map[string]any) { delivered.Done() })
		Event("queued")
		SetEventHandler(func(string, time.Time, map[string]any) {
			wrongHandler <- struct{}{}
			delivered.Done()
		})
	}
	done := make(chan struct{})
	go func() {
		delivered.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("events were not emitted")
	}
	require.Empty(t, wrongHandler)
}

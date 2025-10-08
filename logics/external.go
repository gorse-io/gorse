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

package logics

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/gorse-io/gorse/config"
	"github.com/pkg/errors"
	"github.com/samber/lo"
	"modernc.org/quickjs"
)

type External struct {
	vm     *quickjs.VM
	client *http.Client
	script string
	name   string
}

func NewExternal(cfg config.ExternalConfig) (*External, error) {
	vm, err := quickjs.NewVM()
	if err != nil {
		return nil, errors.WithStack(err)
	}

	// Add environment variables
	env, err := vm.NewObjectValue()
	if err != nil {
		return nil, errors.WithStack(err)
	}
	for _, e := range os.Environ() {
		parts := strings.SplitN(e, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key, err := vm.NewAtom(parts[0])
		if err != nil {
			return nil, errors.WithStack(err)
		}
		value := parts[1]
		if err := env.SetProperty(key, value); err != nil {
			return nil, errors.WithStack(err)
		}
	}
	envKey, err := vm.NewAtom("env")
	if err != nil {
		return nil, errors.WithStack(err)
	}
	if err := vm.GlobalObject().SetProperty(envKey, env); err != nil {
		return nil, errors.WithStack(err)
	}

	// Register fetch function
	external := &External{
		vm:     vm,
		client: &http.Client{},
		script: cfg.Script,
		name:   cfg.Name,
	}
	if err = vm.RegisterFunc("fetch", external.fetch, false); err != nil {
		return nil, errors.WithStack(err)
	}

	return external, nil
}

func (e *External) Close() error {
	return e.vm.Close()
}

func (e *External) Pull(userId string) (res []string, err error) {
	defer func() {
		if r := recover(); r != nil {
			var ok bool
			err, ok = r.(error)
			if !ok {
				err = errors.Errorf("%v", r)
			}
		}
	}()

	userIdKey, err := e.vm.NewAtom("user_id")
	if err != nil {
		return nil, errors.WithStack(err)
	}
	err = e.vm.GlobalObject().SetProperty(userIdKey, userId)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	result, err := e.vm.Eval(e.script, quickjs.EvalGlobal)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	switch v := result.(type) {
	case string:
		var items []string
		if err := json.Unmarshal([]byte(v), &items); err != nil {
			return nil, errors.WithStack(err)
		}
		return items, nil
	case *quickjs.Object:
		var items []string
		if err := json.Unmarshal([]byte(v.String()), &items); err != nil {
			return nil, errors.WithStack(err)
		}
		return items, nil
	default:
		return nil, errors.New("script must return string or object")
	}
}

func (e *External) fetch(args ...quickjs.Value) quickjs.Value {
	var (
		url string
		req = quickjs.UndefinedValue
	)
	if len(args) == 1 {
		switch v := lo.Must(args[0].Any()).(type) {
		case string:
			url = v
		case *quickjs.Object:
			req = args[0]
		default:
			panic("fetch requires first argument to be string or object")
		}
	} else if len(args) == 2 {
		if _, ok := lo.Must(args[0].Any()).(string); !ok {
			panic("fetch requires first argument to be string")
		}
		if _, ok := lo.Must(args[1].Any()).(*quickjs.Object); !ok {
			panic("fetch requires second argument to be object")
		}
		url = lo.Must(args[0].Any()).(string)
		req = args[1]
	} else {
		panic("fetch requires 1 or 2 arguments")
	}
	r := e.parseRequest(url, req)
	resp, err := e.client.Do(r)
	if err != nil {
		panic(err)
	}
	return e.newResponse(resp)
}

// parseRequest parse Fetch API Request.
func (e *External) parseRequest(url string, req quickjs.Value) *http.Request {
	method := "GET"
	headers := make(map[string]string)
	body := ""
	if !req.IsUndefined() {
		// Request.method
		methodKey := lo.Must(e.vm.NewAtom("method"))
		methodValue := lo.Must(req.GetPropertyValue(methodKey))
		if !methodValue.IsUndefined() {
			method = lo.Must(methodValue.Any()).(string)
		}
		// Request.headers
		headersKey := lo.Must(e.vm.NewAtom("headers"))
		headersValue := lo.Must(req.GetPropertyValue(headersKey))
		if !headersValue.IsUndefined() {
			if headersObj, ok := lo.Must(headersValue.Any()).(*quickjs.Object); ok {
				if err := json.Unmarshal([]byte(headersObj.String()), &headers); err != nil {
					panic(err)
				}
			}
		}
		// Request.url
		urlKey := lo.Must(e.vm.NewAtom("url"))
		urlValue := lo.Must(req.GetPropertyValue(urlKey))
		if !urlValue.IsUndefined() {
			url = lo.Must(urlValue.Any()).(string)
		}
		// Request.body
		bodyKey := lo.Must(e.vm.NewAtom("body"))
		bodyValue := lo.Must(req.GetPropertyValue(bodyKey))
		if !bodyValue.IsUndefined() {
			body = lo.Must(bodyValue.Any()).(string)
		}
	}

	r := lo.Must(http.NewRequest(method, url, strings.NewReader(body)))
	for key, value := range headers {
		r.Header.Add(key, value)
	}
	return r
}

// newResponse convert http.Response to Fetch API Response.
func (e *External) newResponse(resp *http.Response) quickjs.Value {
	if resp == nil {
		return quickjs.UndefinedValue
	}
	response := lo.Must(e.vm.NewObjectValue())
	// Response.ok
	okKey := lo.Must(e.vm.NewAtom("ok"))
	lo.Must0(response.SetProperty(okKey, resp.StatusCode >= 200 && resp.StatusCode < 300))
	// Response.status
	statusKey := lo.Must(e.vm.NewAtom("status"))
	lo.Must0(response.SetProperty(statusKey, resp.StatusCode))
	// Response.statusText
	statusTextKey := lo.Must(e.vm.NewAtom("statusText"))
	lo.Must0(response.SetProperty(statusTextKey, resp.Status))
	// Response.body
	bodyKey := lo.Must(e.vm.NewAtom("body"))
	body := lo.Must(io.ReadAll(resp.Body))
	lo.Must0(response.SetProperty(bodyKey, string(body)))
	// Response.headers
	headersKey := lo.Must(e.vm.NewAtom("headers"))
	headers := lo.Must(e.vm.NewObjectValue())
	for key, values := range resp.Header {
		headerKey := lo.Must(e.vm.NewAtom(key))
		lo.Must0(headers.SetProperty(headerKey, strings.Join(values, ", ")))
	}
	lo.Must0(response.SetProperty(headersKey, headers))
	return response
}

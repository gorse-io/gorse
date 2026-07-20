// Copyright 2026 gorse Project Authors
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

package server

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/emicklei/go-restful/v3"
	"github.com/gorse-io/gorse/common/event"
	"github.com/stretchr/testify/require"
)

type channelEventRecorder struct {
	apiEvents chan event.APIEvent
	ctxErrors chan error
}

func (r *channelEventRecorder) RecordAPI(ctx context.Context, e event.APIEvent) {
	r.apiEvents <- e
	r.ctxErrors <- ctx.Err()
}

func (r *channelEventRecorder) RecordStorage(context.Context, event.StorageEvent) {}

func TestLogFilterRecordsBillingDimensions(t *testing.T) {
	recorder := &channelEventRecorder{
		apiEvents: make(chan event.APIEvent, 1),
		ctxErrors: make(chan error, 1),
	}
	event.SetEventRecorder(recorder)
	t.Cleanup(func() { event.SetEventRecorder(&event.NopRecorder{}) })

	restServer := &RestServer{}
	service := new(restful.WebService)
	service.Path("/api").Filter(restServer.LogFilter)
	ctx, cancel := context.WithCancel(t.Context())
	service.Route(service.POST("/items/{item-id}").To(func(request *restful.Request, response *restful.Response) {
		_, err := io.ReadAll(request.Request.Body)
		require.NoError(t, err)
		_, err = response.Write([]byte("response"))
		require.NoError(t, err)
		cancel()
	}))
	container := restful.NewContainer()
	container.Add(service)

	request := httptest.NewRequest(http.MethodPost, "/api/items/item-1", strings.NewReader("request")).WithContext(ctx)
	request.ContentLength = 1 << 20
	response := httptest.NewRecorder()
	container.ServeHTTP(response, request)

	recorded := <-recorder.apiEvents
	require.Equal(t, http.MethodPost, recorded.Method)
	require.Equal(t, "/api/items/{item-id}", recorded.Route)
	require.EqualValues(t, len("request"), recorded.RequestBytes)
	require.EqualValues(t, len("response"), recorded.ResponseBytes)
	require.NoError(t, <-recorder.ctxErrors)
}

// Copyright 2020 gorse Project Authors
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
	"fmt"
	"github.com/haxii/go-swagger-ui/static"
	jsoniter "github.com/json-iterator/go"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	querySwaggerURLKey  string = "url"
	querySwaggerFileKey string = "file"
	querySwaggerHost    string = "host"
	localSwaggerDir     string = "/swagger"
	apiDocsPath         string = "/apidocs/"
)

var swaggerFile string

func serveLocalFile(localFilePath string, w http.ResponseWriter, r *http.Request) {
	newHost := r.URL.Query().Get("host")
	if len(newHost) == 0 {
		http.ServeFile(w, r, localFilePath)
		return
	}
	isJSON := false
	switch filepath.Ext(localFilePath) {
	case ".json":
		isJSON = true
	case ".yaml":
		fallthrough
	case ".yml":
		isJSON = false
	default:
		http.Error(w, "unknown swagger file: "+localFilePath, http.StatusBadRequest)
		return
	}

	// open file
	file, err := os.Open(localFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			http.Error(w, "file not exists", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer file.Close()
	swg := new(map[string]interface{})
	if isJSON {
		dec := jsoniter.NewDecoder(file)
		err = dec.Decode(swg)
	} else {
		dec := yaml.NewDecoder(file)
		err = dec.Decode(swg)
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	(*swg)["host"] = newHost
	var resp []byte
	if isJSON {
		resp, err = jsoniter.Marshal(swg)
	} else {
		resp, err = yaml.Marshal(swg)
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Cache-Control", "no-cache, max-age=0, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	if _, err = w.Write(resp); err != nil {
		log.Error("server:", err)
	}
}

func handler(w http.ResponseWriter, r *http.Request) {
	source := r.URL.Path[len(apiDocsPath):]
	if len(source) == 0 {
		source = "index.html"
	}

	// serve the local file
	localFile := ""
	if strings.HasPrefix(source, "swagger/") {
		// we treat path started with swagger as a direct request of a local swagger file
		localFile = filepath.Join(localSwaggerDir, source[len("swagger/"):])
	}
	if len(localFile) > 0 {
		serveLocalFile(localFile, w, r)
		return
	}

	// server the swagger UI
	//
	// find the in-memory static files
	staticFile, exists := static.Files[source]
	if !exists {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	// set up the content type
	switch filepath.Ext(source) {
	case ".html":
		w.Header().Set("Content-Type", "text/html")
	case ".js":
		w.Header().Set("Content-Type", "application/javascript")
	case ".css":
		w.Header().Set("Content-Type", "text/css")
	default:
		w.Header().Set("Content-Type", "application/octet-stream")
	}

	// return back the non-index files
	if source != "index.html" {
		w.Header().Set("Content-Length", strconv.Itoa(len(staticFile)))
		if _, err := w.Write(staticFile); err != nil {
			log.Error("server:", err)
		}
		return
	}

	// set up the index page
	targetSwagger := swaggerFile
	if f := r.URL.Query().Get(querySwaggerFileKey); len(f) > 0 {
		// requesting a local file, join it with a `swagger/` prefix
		base, err := url.Parse("swagger/")
		if err != nil {
			return
		}
		target, err := url.Parse(f)
		if err != nil {
			return
		}
		targetSwagger = base.ResolveReference(target).String()
		if h := r.URL.Query().Get(querySwaggerHost); len(h) > 0 {
			targetSwagger += "?host=" + h
		}
	} else if _url := r.URL.Query().Get(querySwaggerURLKey); len(_url) > 0 {
		// deal with the query swagger firstly
		targetSwagger = _url
	}
	// replace the target swagger file in index
	indexHTML := string(staticFile)
	indexHTML = strings.Replace(indexHTML,
		"https://petstore.swagger.io/v2/swagger.json",
		targetSwagger, -1)
	w.Header().Set("Content-Length", strconv.Itoa(len(indexHTML)))
	if _, err := fmt.Fprint(w, indexHTML); err != nil {
		log.Error("server:", err)
	}
}

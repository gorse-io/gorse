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

package main

import (
	"sort"
	"strings"
	"time"

	adminclient "github.com/gorse-io/gorse/client"
)

func sortFeedback(feedback []adminclient.Feedback) {
	sort.Slice(feedback, func(i, j int) bool {
		return feedback[i].Timestamp.After(feedback[j].Timestamp)
	})
}

func getConfigValue(configMap map[string]any, names ...string) (string, any, bool) {
	for _, name := range names {
		value, ok := configMap[name]
		if ok {
			return name, value, true
		}
	}
	return "", nil, false
}

func formatConfigMap(configMap map[string]any) map[string]any {
	formatted := make(map[string]any, len(configMap))
	for key, value := range configMap {
		formatted[key] = formatConfigValue(value)
	}
	return formatted
}

func formatConfigValue(value any) any {
	switch v := value.(type) {
	case time.Duration:
		s := v.String()
		if strings.HasSuffix(s, "m0s") {
			s = s[:len(s)-2]
		}
		if strings.HasSuffix(s, "h0m") {
			s = s[:len(s)-2]
		}
		return s
	case map[string]any:
		return formatConfigMap(v)
	case []any:
		formatted := make([]any, len(v))
		for i, item := range v {
			formatted[i] = formatConfigValue(item)
		}
		return formatted
	default:
		return v
	}
}

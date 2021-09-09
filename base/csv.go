// Copyright 2021 gorse Project Authors
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

package base

import (
	"bufio"
	"fmt"
	"strings"
)

// ValidateId validates user/item id. Id cannot be empty and contain [/,].
func ValidateId(text string) error {
	text = strings.TrimSpace(text)
	if text == "" {
		return fmt.Errorf("id cannot be empty")
	} else if strings.Contains(text, "/") {
		return fmt.Errorf("id cannot contain `/`")
	}
	return nil
}

// ValidateLabel validates label. Label cannot be empty and contain [/,|].
func ValidateLabel(text string) error {
	text = strings.TrimSpace(text)
	if text == "" {
		return fmt.Errorf("label cannot be empty")
	} else if strings.Contains(text, "/") {
		return fmt.Errorf("label cannot contain `/`")
	} else if strings.Contains(text, "|") {
		return fmt.Errorf("label cannot contain `|`")
	}
	return nil
}

// Escape text for csv.
func Escape(text string) string {
	// check if need escape
	if !strings.Contains(text, ",") &&
		!strings.Contains(text, "\"") &&
		!strings.Contains(text, "\n") &&
		!strings.Contains(text, "\r") {
		return text
	}
	// start to encode
	builder := strings.Builder{}
	builder.WriteRune('"')
	for _, c := range text {
		if c == '"' {
			builder.WriteString("\"\"")
		} else {
			builder.WriteRune(c)
		}
	}
	builder.WriteRune('"')
	return builder.String()
}

// ReadLines parse fields of each line for csv file.
func ReadLines(sc *bufio.Scanner, sep string, handler func(int, []string) bool) error {
	lineCount := 0               // line number of current position
	fields := make([]string, 0)  // fields for current line
	builder := strings.Builder{} // string builder for current field
	quoted := false              // whether current position in quote
	for sc.Scan() {
		// read line
		lineStr := sc.Text()
		line := []rune(lineStr)
		// start of line
		if quoted {
			builder.WriteString("\r\n")
		}
		// parse line
		for i := 0; i < len(line); i++ {
			if string(line[i]) == sep && !quoted {
				// end of field
				fields = append(fields, builder.String())
				builder.Reset()
			} else if line[i] == '"' {
				if quoted {
					if i+1 >= len(line) || line[i+1] != '"' {
						// end of quoted
						quoted = false
					} else {
						i++
						builder.WriteRune('"')
					}
				} else {
					// start of quoted
					quoted = true
				}
			} else {
				builder.WriteRune(line[i])
			}
		}
		// end of line
		if !quoted {
			fields = append(fields, builder.String())
			builder.Reset()
			if !handler(lineCount, fields) {
				return nil
			}
			fields = []string{}
		}
		// increase line count
		lineCount++
	}
	return sc.Err()
}

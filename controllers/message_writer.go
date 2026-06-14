// Copyright 2023 The OpenAgent Authors. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package controllers

import (
	"bytes"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/beego/beego/context"
	"github.com/the-open-agent/openagent/model"
)

var _ model.SSEEventWriter = (*RefinedWriter)(nil)

func encodeSSEFrame(eventType, data string) []byte {
	if eventType == "" {
		eventType = "message"
	}
	var buf bytes.Buffer
	buf.WriteString("event: ")
	buf.WriteString(eventType)
	buf.WriteByte('\n')
	lines := strings.Split(data, "\n")
	for _, line := range lines {
		buf.WriteString("data: ")
		buf.WriteString(line)
		buf.WriteByte('\n')
	}
	buf.WriteByte('\n')
	return buf.Bytes()
}

type RefinedWriter struct {
	context.Response
	writerCleaner Cleaner
	buf           []byte
	messageBuf    []byte
	reasonBuf     []byte
	toolBuf       []byte
	searchBuf     []byte
}

func newRefinedWriter(w context.Response) *RefinedWriter {
	return &RefinedWriter{
		Response:      w,
		writerCleaner: *NewCleaner(6),
		buf:           []byte{},
		messageBuf:    []byte{},
		reasonBuf:     []byte{},
		toolBuf:       []byte{},
		searchBuf:     []byte{},
	}
}

func (w *RefinedWriter) Write(p []byte) (n int, err error) {
	originalLen := len(p)
	if len(p) > 0 && p[0] == ':' {
		_, err = w.ResponseWriter.Write(p)
		if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
			flusher.Flush()
		}
		return originalLen, err
	}
	_, err = w.ResponseWriter.Write(p)
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
	return originalLen, err
}

func (w *RefinedWriter) Flush() {
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (w *RefinedWriter) WriteSSEEvent(eventType string, data string) error {
	if eventType == "" {
		eventType = "message"
	}

	if eventType == "tool-delta" || eventType == "tool-start" {
		return w.writeSSEFrameToResponse(eventType, data)
	}

	w.buf = append(w.buf, []byte(data)...)
	switch eventType {
	case "message":
		w.messageBuf = append(w.messageBuf, []byte(data)...)
	case "reason":
		w.reasonBuf = append(w.reasonBuf, []byte(data)...)
	case "tool":
		if len(w.toolBuf) > 0 {
			w.toolBuf = append(w.toolBuf, '\n')
		}
		w.toolBuf = append(w.toolBuf, []byte(data)...)
	case "search":
		w.searchBuf = append(w.searchBuf, []byte(data)...)
	}

	if eventType == "tool" || eventType == "search" {
		return w.writeSSEFrameToResponse(eventType, data)
	}

	if w.writerCleaner.cleaned == false && w.writerCleaner.dataTimes < w.writerCleaner.bufferSize {
		w.writerCleaner.AddData(data)
		if w.writerCleaner.dataTimes == w.writerCleaner.bufferSize {
			cleanedData := w.writerCleaner.GetCleanedData()
			return w.flushCleanedEvent(eventType, cleanedData)
		}
		return nil
	}

	return w.flushCleanedEvent(eventType, data)
}

func (w *RefinedWriter) flushCleanedEvent(eventType, data string) error {
	switch eventType {
	case "message", "reason":
		fmt.Print(data)
		jsonData, err := ConvertMessageDataToJSON(data)
		if err != nil {
			return err
		}
		return w.writeSSEFrameToResponse(eventType, string(jsonData))
	default:
		fmt.Print(data)
		return w.writeSSEFrameToResponse(eventType, data)
	}
}

func (w *RefinedWriter) writeSSEFrameToResponse(eventType, data string) error {
	_, err := w.ResponseWriter.Write(encodeSSEFrame(eventType, data))
	if err != nil {
		return err
	}
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
	return nil
}

func (w *RefinedWriter) FlushRemaining() error {
	return nil
}

func (w *RefinedWriter) String() string {
	return string(w.buf)
}

func (w *RefinedWriter) MessageString() string {
	return string(w.messageBuf)
}

func (w *RefinedWriter) ReasonString() string {
	return string(w.reasonBuf)
}

func (w *RefinedWriter) ToolString() string {
	return string(w.toolBuf)
}

func (w *RefinedWriter) SearchString() string {
	return string(w.searchBuf)
}

type Cleaner struct {
	dataTimes  int
	buffer     []string
	bufferSize int
	cleaned    bool
}

func NewCleaner(bufferSize int) *Cleaner {
	return &Cleaner{
		dataTimes:  0,
		buffer:     make([]string, 0, bufferSize),
		bufferSize: bufferSize,
		cleaned:    false,
	}
}

func (c *Cleaner) AddData(data string) {
	c.buffer = append(c.buffer, data)
	c.dataTimes++
}

func (c *Cleaner) GetCleanedData() string {
	c.cleaned = true
	return cleanString(strings.Join(c.buffer, ""))
}

func (c *Cleaner) CleanString(data string) string {
	return cleanString(data)
}

func cleanString(data string) string {
	img := regexp.MustCompile(`<img[^>]+>`)
	if img.MatchString(data) {
		return data
	}

	data = strings.Replace(data, "?", "", -1)
	data = strings.Replace(data, "？", "", -1)
	data = strings.Replace(data, "-", "", -1)
	data = strings.Replace(data, "——", "", -1)

	keywords := []string{"问", "用户", "q", "user", "question"}

	if strings.Contains(data, ":") {
		parts := strings.Split(data, ":")
		data = checkFirstPart(parts[0], parts[len(parts)-1], keywords)
	} else if strings.Contains(data, "：") {
		parts := strings.Split(data, "：")
		data = checkFirstPart(parts[0], parts[len(parts)-1], keywords)
	}

	return data
}

func checkFirstPart(firstPart, secondPart string, keywords []string) string {
	for _, keyword := range keywords {
		if strings.Contains(firstPart, keyword) {
			return secondPart
		}
	}
	return firstPart
}

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
	"encoding/json"
	"fmt"
	"log"
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

type sseFrame struct {
	event string
	data  string
}

type sseBuffer struct {
	raw []byte
}

func newSSEBuffer() *sseBuffer {
	return &sseBuffer{raw: make([]byte, 0, 4096)}
}

func (b *sseBuffer) Write(p []byte) {
	b.raw = append(b.raw, p...)
}

func (b *sseBuffer) ReadFrames() []sseFrame {
	var frames []sseFrame
	for {
		idx := bytes.Index(b.raw, []byte("\n\n"))
		if idx == -1 {
			break
		}
		frameBytes := b.raw[:idx]
		b.raw = b.raw[idx+2:]
		if len(frameBytes) == 0 {
			continue
		}
		frame := parseSSEFrame(frameBytes)
		if frame.event != "" || frame.data != "" {
			frames = append(frames, frame)
		}
	}
	return frames
}

func (b *sseBuffer) Remaining() []byte {
	return b.raw
}

func (b *sseBuffer) Reset() {
	b.raw = b.raw[:0]
}

func parseSSEFrame(frameBytes []byte) sseFrame {
	frame := sseFrame{}
	lines := bytes.Split(frameBytes, []byte("\n"))
	var dataLines []string
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}
		if line[0] == ':' {
			continue
		}
		sepIdx := bytes.Index(line, []byte(": "))
		if sepIdx == -1 {
			sepIdx = bytes.IndexByte(line, ':')
			if sepIdx == -1 {
				continue
			}
		}
		field := string(line[:sepIdx])
		var value string
		if sepIdx+1 < len(line) && line[sepIdx+1] == ' ' {
			value = string(line[sepIdx+2:])
		} else {
			value = string(line[sepIdx+1:])
		}
		switch field {
		case "event":
			frame.event = value
		case "data":
			dataLines = append(dataLines, value)
		}
	}
	frame.data = strings.Join(dataLines, "\n")
	if frame.event == "" && len(dataLines) > 0 {
		frame.event = "message"
	}
	return frame
}

type pendingToolCall struct {
	index               int
	id                  string
	name                string
	arguments           string
	generatingArguments bool
}

type RefinedWriter struct {
	context.Response
	writerCleaner     Cleaner
	sseBuf            *sseBuffer
	buf               []byte
	messageBuf        []byte
	reasonBuf         []byte
	toolBuf           []byte
	searchBuf         []byte
	pendingToolCalls  []*pendingToolCall
}

func newRefinedWriter(w context.Response) *RefinedWriter {
	return &RefinedWriter{
		Response:       w,
		writerCleaner:  *NewCleaner(6),
		sseBuf:         newSSEBuffer(),
		buf:            []byte{},
		messageBuf:     []byte{},
		reasonBuf:      []byte{},
		toolBuf:        []byte{},
		searchBuf:      []byte{},
		pendingToolCalls: []*pendingToolCall{},
	}
}

func (w *RefinedWriter) Write(p []byte) (n int, err error) {
	originalLen := len(p)
	if len(p) == 0 {
		return 0, nil
	}

	w.sseBuf.Write(p)
	frames := w.sseBuf.ReadFrames()
	for _, frame := range frames {
		if err := w.WriteSSEEvent(frame.event, frame.data); err != nil {
			return originalLen, err
		}
	}
	return originalLen, nil
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

	switch eventType {
	case "message", "reason":
		return w.handleTextEvent(eventType, data)
	case "tool":
		return w.handleFinalToolEvent(data)
	case "search":
		return w.handleSearchEvent(data)
	case "tool-start":
		return w.handleToolStart(data)
	case "tool-delta":
		return w.handleToolDelta(data)
	default:
		return w.writeSSEFrameToResponse(eventType, data)
	}
}

func (w *RefinedWriter) handleTextEvent(eventType, data string) error {
	if data == "" {
		return w.writeSSEFrameToResponse(eventType, data)
	}

	w.buf = append(w.buf, []byte(data)...)
	switch eventType {
	case "message":
		w.messageBuf = append(w.messageBuf, []byte(data)...)
	case "reason":
		w.reasonBuf = append(w.reasonBuf, []byte(data)...)
	}

	if w.writerCleaner.cleaned == false && w.writerCleaner.dataTimes < w.writerCleaner.bufferSize {
		w.writerCleaner.AddData(data)
		if w.writerCleaner.dataTimes == w.writerCleaner.bufferSize {
			cleanedData := w.writerCleaner.GetCleanedData()
			return w.flushCleanedEvent(eventType, cleanedData)
		}
		return w.writeSSEFrameToResponse(eventType, data)
	}

	return w.flushCleanedEvent(eventType, data)
}

func (w *RefinedWriter) handleFinalToolEvent(data string) error {
	if data == "" {
		return w.writeSSEFrameToResponse("tool", data)
	}

	var toolEvent struct {
		Index     int    `json:"index"`
		ID        string `json:"id"`
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
		Content   string `json:"content"`
		IsError   bool   `json:"isError"`
	}
	if err := json.Unmarshal([]byte(data), &toolEvent); err != nil {
		log.Printf("[RefinedWriter] invalid tool event JSON: %q, error: %v", data[:min(len(data), 200)], err)
		return w.writeSSEFrameToResponse("tool", data)
	}

	for i, pending := range w.pendingToolCalls {
		if pending.index == toolEvent.Index ||
			(pending.name != "" && pending.name == toolEvent.Name) ||
			(pending.id != "" && pending.id == toolEvent.ID) {
			if toolEvent.Name == "" {
				toolEvent.Name = pending.name
			}
			if toolEvent.Arguments == "" && pending.arguments != "" {
				toolEvent.Arguments = pending.arguments
			}
			if toolEvent.ID == "" {
				toolEvent.ID = pending.id
			}
			w.pendingToolCalls = append(w.pendingToolCalls[:i], w.pendingToolCalls[i+1:]...)
			break
		}
	}

	toolCall := model.ToolCall{
		Name:      toolEvent.Name,
		Arguments: toolEvent.Arguments,
		Content:   toolEvent.Content,
		IsError:   toolEvent.IsError,
	}

	validatedJSON, err := json.Marshal(toolCall)
	if err != nil {
		log.Printf("[RefinedWriter] failed to marshal validated tool call: %v", err)
		return w.writeSSEFrameToResponse("tool", data)
	}

	w.buf = append(w.buf, validatedJSON...)
	if len(w.toolBuf) > 0 {
		w.toolBuf = append(w.toolBuf, '\n')
	}
	w.toolBuf = append(w.toolBuf, validatedJSON...)

	return w.writeSSEFrameToResponse("tool", string(validatedJSON))
}

func (w *RefinedWriter) handleSearchEvent(data string) error {
	if data == "" {
		return w.writeSSEFrameToResponse("search", data)
	}

	var searchResults []model.SearchResult
	if err := json.Unmarshal([]byte(data), &searchResults); err != nil {
		log.Printf("[RefinedWriter] invalid search event JSON: %q, error: %v", data[:min(len(data), 200)], err)
		return w.writeSSEFrameToResponse("search", data)
	}

	validatedJSON, err := json.Marshal(searchResults)
	if err != nil {
		log.Printf("[RefinedWriter] failed to marshal validated search results: %v", err)
		return w.writeSSEFrameToResponse("search", data)
	}

	w.buf = append(w.buf, validatedJSON...)
	w.searchBuf = append(w.searchBuf, validatedJSON...)

	return w.writeSSEFrameToResponse("search", string(validatedJSON))
}

func (w *RefinedWriter) handleToolStart(data string) error {
	var delta struct {
		Index int    `json:"index"`
		ID    string `json:"id"`
		Name  string `json:"name"`
	}
	if err := json.Unmarshal([]byte(data), &delta); err != nil {
		log.Printf("[RefinedWriter] invalid tool-start JSON: %q, error: %v", data[:min(len(data), 200)], err)
		return w.writeSSEFrameToResponse("tool-start", data)
	}

	pending := &pendingToolCall{
		index:               delta.Index,
		id:                  delta.ID,
		name:                delta.Name,
		generatingArguments: true,
	}

	for i, existing := range w.pendingToolCalls {
		if existing.index == pending.index && existing.generatingArguments {
			w.pendingToolCalls[i] = pending
			return w.writeSSEFrameToResponse("tool-start", data)
		}
	}
	w.pendingToolCalls = append(w.pendingToolCalls, pending)
	return w.writeSSEFrameToResponse("tool-start", data)
}

func (w *RefinedWriter) handleToolDelta(data string) error {
	var delta struct {
		Index          int    `json:"index"`
		ID             string `json:"id"`
		Name           string `json:"name"`
		ArgumentsDelta string `json:"argumentsDelta"`
	}
	if err := json.Unmarshal([]byte(data), &delta); err != nil {
		log.Printf("[RefinedWriter] invalid tool-delta JSON: %q, error: %v", data[:min(len(data), 200)], err)
		return w.writeSSEFrameToResponse("tool-delta", data)
	}

	if delta.Name == "" && delta.ArgumentsDelta == "" {
		return w.writeSSEFrameToResponse("tool-delta", data)
	}

	for _, pending := range w.pendingToolCalls {
		if pending.generatingArguments && pending.index == delta.Index {
			if delta.ID != "" {
				pending.id = delta.ID
			}
			if delta.Name != "" {
				pending.name = delta.Name
			}
			pending.arguments += delta.ArgumentsDelta
			break
		}
	}

	return w.writeSSEFrameToResponse("tool-delta", data)
}

func (w *RefinedWriter) FinalizePendingTools() {
	for _, pending := range w.pendingToolCalls {
		if pending.name == "" {
			pending.name = "tool"
		}
		validatedJSON, err := json.Marshal(model.ToolCall{
			Name:      pending.name,
			Arguments: pending.arguments,
		})
		if err != nil {
			continue
		}
		w.buf = append(w.buf, validatedJSON...)
		if len(w.toolBuf) > 0 {
			w.toolBuf = append(w.toolBuf, '\n')
		}
		w.toolBuf = append(w.toolBuf, validatedJSON...)
	}
	w.pendingToolCalls = nil
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
	remaining := w.sseBuf.Remaining()
	if len(remaining) == 0 {
		return nil
	}

	log.Printf("[RefinedWriter] FlushRemaining: discarding %d bytes of incomplete SSE frame: %q", len(remaining), string(remaining)[:min(len(remaining), 200)])
	w.sseBuf.Reset()
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

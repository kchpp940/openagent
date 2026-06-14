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
	"net/http"
	"regexp"
	"strings"

	"github.com/beego/beego/context"
	"github.com/the-open-agent/openagent/model"
	"github.com/the-open-agent/openagent/object"
)

var _ model.ExecutionRecorder = (*RefinedWriter)(nil)

type RefinedWriter struct {
	context.Response
	writerCleaner   Cleaner
	buf             []byte
	messageBuf      []byte
	reasonBuf       []byte
	toolBuf         []byte
	searchBuf       []byte
	ExecutionTracer *object.ExecutionTracer
	reasonStepId    string
}

func newRefinedWriter(w context.Response, tracer *object.ExecutionTracer) *RefinedWriter {
	return &RefinedWriter{w, *NewCleaner(6), []byte{}, []byte{}, []byte{}, []byte{}, []byte{}, tracer, ""}
}

func (w *RefinedWriter) Write(p []byte) (n int, err error) {
	if len(p) > 0 && p[0] == ':' {
		n, err = w.ResponseWriter.Write(p)
		if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
			flusher.Flush()
		}
		return n, err
	}

	var eventType string
	var data string

	if bytes.HasPrefix(p, []byte("event: reason")) {
		eventType = "reason"
		prefix := []byte("event: reason\ndata: ")
		suffix := []byte("\n\n")
		data = string(bytes.TrimSuffix(bytes.TrimPrefix(p, prefix), suffix))
	} else if bytes.HasPrefix(p, []byte("event: tool-delta")) {
		eventType = "tool-delta"
		prefix := []byte("event: tool-delta\ndata: ")
		suffix := []byte("\n\n")
		data = string(bytes.TrimSuffix(bytes.TrimPrefix(p, prefix), suffix))
	} else if bytes.HasPrefix(p, []byte("event: tool-start")) {
		eventType = "tool-start"
		prefix := []byte("event: tool-start\ndata: ")
		suffix := []byte("\n\n")
		data = string(bytes.TrimSuffix(bytes.TrimPrefix(p, prefix), suffix))
	} else if bytes.HasPrefix(p, []byte("event: tool")) {
		eventType = "tool"
		prefix := []byte("event: tool\ndata: ")
		suffix := []byte("\n\n")
		data = string(bytes.TrimSuffix(bytes.TrimPrefix(p, prefix), suffix))
	} else if bytes.HasPrefix(p, []byte("event: search")) {
		eventType = "search"
		prefix := []byte("event: search\ndata: ")
		suffix := []byte("\n\n")
		data = string(bytes.TrimSuffix(bytes.TrimPrefix(p, prefix), suffix))
	} else {
		eventType = "message"
		prefix := []byte("event: message\ndata: ")
		suffix := []byte("\n\n")
		data = string(bytes.TrimSuffix(bytes.TrimPrefix(p, prefix), suffix))
	}

	// Tool progress events are UI-only and should not be saved in the final message.
	if eventType == "tool-delta" || eventType == "tool-start" {
		n, err := w.ResponseWriter.Write([]byte(fmt.Sprintf("event: %s\ndata: %s\n\n", eventType, data)))
		if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
			flusher.Flush()
		}
		return n, err
	}

	// Add data to the buffer
	w.buf = append(w.buf, []byte(data)...)
	if eventType == "message" {
		w.messageBuf = append(w.messageBuf, []byte(data)...)
	} else if eventType == "reason" {
		w.reasonBuf = append(w.reasonBuf, []byte(data)...)
		w.recordReasoningStep(data)
	} else if eventType == "tool" {
		if len(w.toolBuf) > 0 {
			w.toolBuf = append(w.toolBuf, '\n')
		}
		w.toolBuf = append(w.toolBuf, []byte(data)...)
		w.recordToolResult(data)
	} else if eventType == "search" {
		w.searchBuf = append(w.searchBuf, []byte(data)...)
		w.recordSearchResult(data)
	}

	if eventType == "tool" || eventType == "search" {
		fmt.Print(data)
		n, err := w.ResponseWriter.Write([]byte(fmt.Sprintf("event: %s\ndata: %s\n\n", eventType, data)))
		if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
			flusher.Flush()
		}
		return n, err
	}

	if eventType == "reason" {
		fmt.Print(data)
		jsonData, err := ConvertMessageDataToJSON(data)
		if err != nil {
			return 0, err
		}
		return w.ResponseWriter.Write([]byte(fmt.Sprintf("event: %s\ndata: %s\n\n", eventType, jsonData)))
	}

	if w.writerCleaner.cleaned == false && w.writerCleaner.dataTimes < w.writerCleaner.bufferSize {
		w.writerCleaner.AddData(data)
		if w.writerCleaner.dataTimes == w.writerCleaner.bufferSize {
			cleanedData := w.writerCleaner.GetCleanedData()
			fmt.Print(cleanedData)
			jsonData, err := ConvertMessageDataToJSON(cleanedData)
			if err != nil {
				return 0, err
			}
			return w.ResponseWriter.Write([]byte(fmt.Sprintf("event: %s\ndata: %s\n\n", eventType, jsonData)))
		}
		return 0, nil
	}

	fmt.Print(data)
	jsonData, err := ConvertMessageDataToJSON(data)
	if err != nil {
		return 0, err
	}
	return w.ResponseWriter.Write([]byte(fmt.Sprintf("event: %s\ndata: %s\n\n", eventType, jsonData)))
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
	dataTimes  int      // Number of times data is added
	buffer     []string // Buffer of tokens
	bufferSize int      // Size of the buffer
	cleaned    bool     // Whether the data has been cleaned
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

func (w *RefinedWriter) persistTracer() {
	if w == nil || w.ExecutionTracer == nil {
		return
	}
	w.ExecutionTracer.Persist()
}

func (w *RefinedWriter) StartModelCall(modelName string, round int) string {
	if w.ExecutionTracer == nil {
		return ""
	}
	title := "Model Call"
	if modelName != "" {
		title = fmt.Sprintf("Model: %s", modelName)
	}
	metadata := map[string]interface{}{
		"round": round,
	}
	if modelName != "" {
		metadata["model"] = modelName
	}
	stepId := w.ExecutionTracer.StartStep(object.StepTypeModelStart, title, metadata)
	w.persistTracer()
	return stepId
}

func (w *RefinedWriter) EndModelCall(stepId string, tokenCount int, err error) {
	if w.ExecutionTracer == nil || stepId == "" {
		return
	}
	status := object.StepStatusCompleted
	output := fmt.Sprintf("%d tokens", tokenCount)
	errorMsg := ""
	if err != nil {
		status = object.StepStatusFailed
		errorMsg = err.Error()
	}
	w.ExecutionTracer.EndStep(stepId, status, output, errorMsg)
	w.persistTracer()
}

func (w *RefinedWriter) StartToolCall(toolName string, arguments string, round int) string {
	if w.ExecutionTracer == nil {
		return ""
	}
	title := fmt.Sprintf("Tool: %s", toolName)
	metadata := map[string]interface{}{
		"round":    round,
		"toolName": toolName,
	}
	stepId := w.ExecutionTracer.StartStep(object.StepTypeToolCallStart, title, metadata)
	w.ExecutionTracer.UpdateStep(stepId, map[string]interface{}{
		"input": truncateString(arguments, 500),
	})
	w.persistTracer()
	return stepId
}

func (w *RefinedWriter) EndToolCall(stepId string, result string, err error) {
	if w.ExecutionTracer == nil || stepId == "" {
		return
	}
	status := object.StepStatusCompleted
	output := truncateString(result, 1000)
	errorMsg := ""
	if err != nil {
		status = object.StepStatusFailed
		errorMsg = err.Error()
	}
	w.ExecutionTracer.UpdateStep(stepId, map[string]interface{}{
		"status": status,
		"type":   object.StepTypeToolCallEnd,
	})
	w.ExecutionTracer.EndStep(stepId, status, output, errorMsg)
	w.persistTracer()
}

func (w *RefinedWriter) AddInfoStep(title string, description string) {
	if w.ExecutionTracer == nil {
		return
	}
	w.ExecutionTracer.AddSimpleStep(object.StepTypeInfo, title, description, nil)
	w.persistTracer()
}

func (w *RefinedWriter) AddErrorStep(title string, errMsg string) {
	if w.ExecutionTracer == nil {
		return
	}
	w.ExecutionTracer.AddSimpleStep(object.StepTypeError, title, errMsg, nil)
	w.persistTracer()
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func (w *RefinedWriter) recordReasoningStep(data string) {
	if w.ExecutionTracer == nil {
		return
	}
	if w.reasonStepId == "" {
		w.reasonStepId = w.ExecutionTracer.StartStep(object.StepTypeReasoning, "Reasoning", map[string]interface{}{
			"mode": "streaming",
		})
	}
	w.ExecutionTracer.UpdateStep(w.reasonStepId, map[string]interface{}{
		"output": truncateString(string(w.reasonBuf), 2000),
	})
}

func (w *RefinedWriter) recordSearchResult(data string) {
	if w.ExecutionTracer == nil {
		return
	}
	var searchResults []model.SearchResult
	if err := json.Unmarshal([]byte(data), &searchResults); err == nil && len(searchResults) > 0 {
		summary := fmt.Sprintf("%d search results", len(searchResults))
		metadata := make([]map[string]interface{}, 0, len(searchResults))
		for _, r := range searchResults {
			metadata = append(metadata, map[string]interface{}{
				"title":    r.Title,
				"url":      r.URL,
				"siteName": r.SiteName,
				"index":    r.Index,
			})
		}
		w.ExecutionTracer.AddSimpleStep(object.StepTypeInfo, "Web Search", summary, map[string]interface{}{
			"results": metadata,
		})
		w.persistTracer()
	}
}

func (w *RefinedWriter) recordToolResult(data string) {
	if w.ExecutionTracer == nil {
		return
	}
	var toolCall model.ToolCall
	if err := json.Unmarshal([]byte(data), &toolCall); err == nil {
		if toolCall.IsError {
			errorMsg := toolCall.Content
			if errorMsg == "" {
				errorMsg = "Tool call failed"
			}
			w.ExecutionTracer.AddSimpleStep(object.StepTypeToolError, fmt.Sprintf("Tool Error: %s", toolCall.Name), errorMsg, map[string]interface{}{
				"toolName":  toolCall.Name,
				"arguments": toolCall.Arguments,
			})
			w.persistTracer()
		}
	}
}

func (w *RefinedWriter) WriteVectorEvent(vectorScores []object.VectorScore, knowledge []*model.RawMessage) error {
	if w.ExecutionTracer != nil && (len(vectorScores) > 0 || len(knowledge) > 0) {
		total := len(knowledge)
		if len(vectorScores) > total {
			total = len(vectorScores)
		}
		chunks := make([]map[string]interface{}, 0, total)
		for i := 0; i < total; i++ {
			item := map[string]interface{}{
				"index": i,
			}
			if i < len(knowledge) {
				item["text"] = truncateString(knowledge[i].Text, 500)
				item["tokenCount"] = knowledge[i].TextTokenCount
			}
			if i < len(vectorScores) {
				item["vector"] = vectorScores[i].Vector
				item["score"] = vectorScores[i].Score
			}
			chunks = append(chunks, item)
		}
		summary := fmt.Sprintf("%d chunks", total)
		w.ExecutionTracer.AddSimpleStep(object.StepTypeKnowledgeRetrieval, "Knowledge Retrieval", summary, map[string]interface{}{
			"chunks": chunks,
		})
		w.persistTracer()
	}
	bytes, err := json.Marshal(vectorScores)
	if err != nil {
		return err
	}
	_, err = w.ResponseWriter.Write([]byte(fmt.Sprintf("event: vector\ndata: %s\n\n", string(bytes))))
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
	return err
}

func (w *RefinedWriter) RecordStreamingError(errorText string) {
	if w.ExecutionTracer == nil {
		return
	}
	w.ExecutionTracer.AddSimpleStep(object.StepTypeError, "Streaming Error", errorText, nil)
	w.persistTracer()
}

func (w *RefinedWriter) FinalizeReasoningStep() {
	if w.ExecutionTracer == nil || w.reasonStepId == "" {
		return
	}
	w.ExecutionTracer.EndStep(w.reasonStepId, object.StepStatusCompleted, fmt.Sprintf("%d chars", len(w.reasonBuf)), "")
	w.reasonStepId = ""
	w.persistTracer()
}

func (w *RefinedWriter) WriteMyErrorEvent(errorText string) error {
	if w.ExecutionTracer != nil {
		w.ExecutionTracer.AddSimpleStep(object.StepTypeError, "Generation Failed", errorText, nil)
		w.persistTracer()
	}
	sseData, err := ConvertMessageDataToJSON(errorText)
	if err != nil {
		return err
	}
	_, err = w.ResponseWriter.Write([]byte(fmt.Sprintf("event: myerror\ndata: %s\n\n", sseData)))
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
	return err
}

func (w *RefinedWriter) WriteEndEvent(data string) error {
	if w.ExecutionTracer != nil {
		w.ExecutionTracer.AddSimpleStep(object.StepTypeInfo, "Stream Ended", data, nil)
		w.persistTracer()
	}
	_, err := w.ResponseWriter.Write([]byte(fmt.Sprintf("event: end\ndata: %s\n\n", data)))
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
	return err
}

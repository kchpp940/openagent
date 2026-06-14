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
)

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
	sseBuf        *sseBuffer
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
		sseBuf:        newSSEBuffer(),
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
		n, err = w.ResponseWriter.Write(p)
		if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
			flusher.Flush()
		}
		return originalLen, err
	}

	w.sseBuf.Write(p)
	frames := w.sseBuf.ReadFrames()

	for _, frame := range frames {
		if err := w.processFrame(frame); err != nil {
			return originalLen, err
		}
	}

	return originalLen, nil
}

func (w *RefinedWriter) processFrame(frame sseFrame) error {
	eventType := frame.event
	data := frame.data

	if eventType == "" {
		eventType = "message"
	}

	if eventType == "tool-delta" || eventType == "tool-start" {
		return w.flushFrame(eventType, data, false)
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
		return w.flushFrame(eventType, data, true)
	}

	if w.writerCleaner.cleaned == false && w.writerCleaner.dataTimes < w.writerCleaner.bufferSize {
		w.writerCleaner.AddData(data)
		if w.writerCleaner.dataTimes == w.writerCleaner.bufferSize {
			cleanedData := w.writerCleaner.GetCleanedData()
			return w.flushFrame(eventType, cleanedData, true)
		}
		return nil
	}

	return w.flushFrame(eventType, data, true)
}

func (w *RefinedWriter) flushFrame(eventType, data string, useCleaner bool) error {
	if useCleaner {
		switch eventType {
		case "message", "reason":
			fmt.Print(data)
			jsonData, err := ConvertMessageDataToJSON(data)
			if err != nil {
				return err
			}
			_, err = w.ResponseWriter.Write(encodeSSEFrame(eventType, string(jsonData)))
			if err != nil {
				return err
			}
		default:
			fmt.Print(data)
			_, err := w.ResponseWriter.Write(encodeSSEFrame(eventType, data))
			if err != nil {
				return err
			}
		}
	} else {
		_, err := w.ResponseWriter.Write(encodeSSEFrame(eventType, data))
		if err != nil {
			return err
		}
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

	frameStr := string(remaining)
	frame := sseFrame{}

	if strings.HasPrefix(frameStr, "event:") {
		frame = parseSSEFrame(remaining)
	} else {
		frame.event = "message"
		frame.data = frameStr
	}

	if frame.data != "" || frame.event != "" {
		return w.processFrame(frame)
	}
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

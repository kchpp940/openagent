// Copyright 2025 The OpenAgent Authors. All Rights Reserved.
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

package mcp

import (
	"bytes"
	"encoding/base64"
	"encoding/gob"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
)

const (
	safeIdPrefix       = "mcp_"
	oldSeparator       = "__"
	legacyBuiltinHint  = "builtin_"
)

type ToolIdMetadata struct {
	ServerName string `json:"s,omitempty"`
	ToolName   string `json:"t"`
}

type toolIdRegistry struct {
	mu       sync.RWMutex
	byId     map[string]ToolIdMetadata
	byKey    map[string]string
}

var globalToolIdRegistry = &toolIdRegistry{
	byId:  make(map[string]ToolIdMetadata),
	byKey: make(map[string]string),
}

func encodeGobBase64(md ToolIdMetadata) (string, error) {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(md); err != nil {
		return "", fmt.Errorf("gob encode tool id metadata: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf.Bytes()), nil
}

func decodeGobBase64(s string, md *ToolIdMetadata) error {
	data, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return fmt.Errorf("base64 decode tool id: %w", err)
	}
	if err := gob.NewDecoder(bytes.NewReader(data)).Decode(md); err != nil {
		return fmt.Errorf("gob decode tool id: %w", err)
	}
	return nil
}

func metadataKey(serverName, toolName string) string {
	return serverName + "\x00" + toolName
}

func (r *toolIdRegistry) getOrCreate(serverName, toolName string) (string, error) {
	key := metadataKey(serverName, toolName)
	r.mu.RLock()
	if id, ok := r.byKey[key]; ok {
		r.mu.RUnlock()
		return id, nil
	}
	r.mu.RUnlock()

	md := ToolIdMetadata{
		ServerName: serverName,
		ToolName:   toolName,
	}
	suffix, err := encodeGobBase64(md)
	if err != nil {
		return "", err
	}
	id := safeIdPrefix + suffix

	r.mu.Lock()
	defer r.mu.Unlock()
	if existing, ok := r.byKey[key]; ok {
		return existing, nil
	}
	r.byKey[key] = id
	r.byId[id] = md
	return id, nil
}

func (r *toolIdRegistry) lookup(id string) (ToolIdMetadata, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	md, ok := r.byId[id]
	return md, ok
}

func (r *toolIdRegistry) register(id string, md ToolIdMetadata) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.byId[id] = md
	r.byKey[metadataKey(md.ServerName, md.ToolName)] = id
}

func GetIdFromServerNameAndToolName(serverName, toolName string) (string, error) {
	if toolName == "" {
		return "", errors.New("tool name cannot be empty when constructing tool id")
	}
	return globalToolIdRegistry.getOrCreate(serverName, toolName)
}

func GetServerNameAndToolNameFromId(id string) (string, string, error) {
	if id == "" {
		return "", "", errors.New("tool id is empty")
	}

	if strings.HasPrefix(id, safeIdPrefix) {
		suffix := strings.TrimPrefix(id, safeIdPrefix)

		if md, ok := globalToolIdRegistry.lookup(id); ok {
			if md.ToolName == "" {
				return "", "", fmt.Errorf("registered tool id has empty tool name: %s", id)
			}
			return md.ServerName, md.ToolName, nil
		}

		var md ToolIdMetadata
		if err := decodeGobBase64(suffix, &md); err == nil {
			if md.ToolName == "" {
				return "", "", fmt.Errorf("decoded tool id has empty tool name: %s", id)
			}
			globalToolIdRegistry.register(id, md)
			return md.ServerName, md.ToolName, nil
		}

		if md, err := decodeJSONFallback(suffix); err == nil {
			if md.ToolName != "" {
				globalToolIdRegistry.register(id, md)
				return md.ServerName, md.ToolName, nil
			}
		}

		return "", "", fmt.Errorf("invalid mcp_ tool id, cannot decode: %s", id)
	}

	if strings.Count(id, oldSeparator) == 1 {
		idx := strings.Index(id, oldSeparator)
		serverName := id[:idx]
		toolName := id[idx+len(oldSeparator):]
		if toolName == "" {
			return "", "", fmt.Errorf("tool name is empty after splitting legacy id: %s", id)
		}
		return serverName, toolName, nil
	}

	if !strings.Contains(id, oldSeparator) && !strings.Contains(id, "{") {
		return "", id, nil
	}

	if strings.HasPrefix(id, "[") && strings.HasSuffix(id, "]") {
		var md ToolIdMetadata
		if err := json.Unmarshal([]byte(id), &md); err == nil {
			if md.ToolName == "" {
				return "", "", fmt.Errorf("decoded legacy JSON tool id has empty tool name: %s", id)
			}
			return md.ServerName, md.ToolName, nil
		}
	}

	return "", "", fmt.Errorf("invalid tool id format, cannot parse: %s", id)
}

func decodeJSONFallback(suffix string) (ToolIdMetadata, error) {
	var md ToolIdMetadata
	jsonBytes, err := base64.RawURLEncoding.DecodeString(suffix)
	if err != nil {
		return md, err
	}
	if err := json.Unmarshal(jsonBytes, &md); err != nil {
		return md, err
	}
	return md, nil
}

func RegisterToolIdMetadata(id string, md ToolIdMetadata) {
	if id == "" || md.ToolName == "" {
		return
	}
	globalToolIdRegistry.register(id, md)
}

func GetToolIdMetadata(id string) (ToolIdMetadata, bool) {
	return globalToolIdRegistry.lookup(id)
}

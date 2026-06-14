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
	"errors"
	"fmt"
	"hash/fnv"
	"strconv"
	"strings"
	"sync"
)

const (
	safeIdPrefix = "mcp_"
	oldSeparator = "__"
)

type ToolIdMetadata struct {
	ServerName string `json:"s,omitempty"`
	ToolName   string `json:"t"`
}

type toolIdRegistry struct {
	mu        sync.RWMutex
	byId      map[string]ToolIdMetadata
	byKey     map[string]string
	collision map[uint64]int
}

var globalToolIdRegistry = &toolIdRegistry{
	byId:      make(map[string]ToolIdMetadata),
	byKey:     make(map[string]string),
	collision: make(map[uint64]int),
}

func hashFnv1a64(s string) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(s))
	return h.Sum64()
}

func encodeBase36(v uint64) string {
	return strconv.FormatUint(v, 36)
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

	baseHash := hashFnv1a64(key)

	r.mu.Lock()
	defer r.mu.Unlock()

	if existing, ok := r.byKey[key]; ok {
		return existing, nil
	}

	attempt := 0
	for {
		var id string
		if attempt == 0 {
			id = safeIdPrefix + encodeBase36(baseHash)
		} else {
			combined := baseHash ^ (uint64(attempt) * 0x9E3779B97F4A7C15)
			id = safeIdPrefix + encodeBase36(combined)
		}
		if existing, exists := r.byId[id]; !exists {
			r.byId[id] = md
			r.byKey[key] = id
			return id, nil
		} else if existing.ServerName == serverName && existing.ToolName == toolName {
			r.byKey[key] = id
			return id, nil
		}
		attempt++
		r.collision[baseHash] = attempt
		if attempt > 1000 {
			return "", fmt.Errorf("tool id hash collision exceeded 1000 attempts for key=%q", key)
		}
	}
}

func (r *toolIdRegistry) lookup(id string) (ToolIdMetadata, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	md, ok := r.byId[id]
	return md, ok
}

func (r *toolIdRegistry) register(id string, md ToolIdMetadata) bool {
	if id == "" || md.ToolName == "" {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if existing, ok := r.byId[id]; ok {
		if existing.ServerName == md.ServerName && existing.ToolName == md.ToolName {
			return true
		}
		return false
	}
	r.byId[id] = md
	key := metadataKey(md.ServerName, md.ToolName)
	if _, ok := r.byKey[key]; !ok {
		r.byKey[key] = id
	}
	return true
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
		if md, ok := globalToolIdRegistry.lookup(id); ok {
			if md.ToolName == "" {
				return "", "", fmt.Errorf("registered tool id has empty tool name: %s", id)
			}
			return md.ServerName, md.ToolName, nil
		}
		return "", "", fmt.Errorf("unknown mcp_ tool id (not registered): %s; ensure persisted ToolCall has serverName/toolName fields for cross-process recovery", id)
	}

	if strings.Contains(id, oldSeparator) {
		count := strings.Count(id, oldSeparator)
		if count > 1 {
			return "", "", fmt.Errorf("ambiguous legacy tool id with %d separators; cannot determine server/tool boundary: %s", count, id)
		}
		idx := strings.Index(id, oldSeparator)
		serverName := id[:idx]
		toolName := id[idx+len(oldSeparator):]
		if toolName == "" {
			return "", "", fmt.Errorf("tool name is empty after splitting legacy id: %s", id)
		}
		md := ToolIdMetadata{ServerName: serverName, ToolName: toolName}
		globalToolIdRegistry.register(id, md)
		return serverName, toolName, nil
	}

	if !strings.Contains(id, "{") && !strings.HasPrefix(id, "[") {
		md := ToolIdMetadata{ServerName: "", ToolName: id}
		globalToolIdRegistry.register(id, md)
		return "", id, nil
	}

	return "", "", fmt.Errorf("invalid tool id format, cannot parse: %s", id)
}

func RegisterToolIdMetadata(id string, md ToolIdMetadata) {
	globalToolIdRegistry.register(id, md)
}

func GetToolIdMetadata(id string) (ToolIdMetadata, bool) {
	return globalToolIdRegistry.lookup(id)
}

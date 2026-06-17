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

// ---- 纯编解码工具函数边界 ----
// 本文件仅包含 Tool ID 的纯字符串编解码逻辑，与执行上下文、错误处理、
// Schema 解析无关。若需工具执行 / 错误包装 / Schema 解析，请使用
// execution.go 中的 ToolExecutionContext / ExternalCallResult / SchemaParseResult。
// ------------------------------

package mcp

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

const oldSeparator = "__"

type toolIdPayload struct {
	Server string `json:"s"`
	Tool   string `json:"t"`
}

func GetServerNameAndToolNameFromId(id string) (string, string, error) {
	if id == "" {
		return "", "", errors.New("tool id is empty")
	}

	if strings.HasPrefix(id, "[") && strings.HasSuffix(id, "]") {
		var payload toolIdPayload
		if err := json.Unmarshal([]byte(id), &payload); err == nil {
			if payload.Tool == "" {
				return "", "", fmt.Errorf("decoded tool id has empty tool name: %s", id)
			}
			return payload.Server, payload.Tool, nil
		}
	}

	if strings.Count(id, oldSeparator) == 1 {
		idx := strings.Index(id, oldSeparator)
		serverName := id[:idx]
		toolName := id[idx+len(oldSeparator):]
		if toolName == "" {
			return "", "", fmt.Errorf("tool name is empty after splitting id: %s", id)
		}
		return serverName, toolName, nil
	}

	if !strings.Contains(id, oldSeparator) {
		return "", id, nil
	}

	return "", "", fmt.Errorf("invalid tool id format, cannot parse: %s", id)
}

func GetIdFromServerNameAndToolName(serverName, toolName string) (string, error) {
	if toolName == "" {
		return "", errors.New("tool name cannot be empty when constructing tool id")
	}
	payload := toolIdPayload{
		Server: serverName,
		Tool:   toolName,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to encode tool id: %w", err)
	}
	return string(data), nil
}

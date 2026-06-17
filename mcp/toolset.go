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
	"context"
	"errors"
	"fmt"

	"github.com/ThinkInAIXYZ/go-mcp/client"
	"github.com/ThinkInAIXYZ/go-mcp/protocol"
	"github.com/the-open-agent/openagent/tool"
)

type ToolCallError struct {
	Kind    string
	Message string
	Err     error
}

func (e *ToolCallError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Kind, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Kind, e.Message)
}

func (e *ToolCallError) Unwrap() error {
	return e.Err
}

// ---- 旧 API 兼容层：保留旧常量供遗留代码使用
// 新代码请使用 mcp.ErrKind* 常量，禁止在新代码中新增对 ToolCallErr* 的引用。
const (
	// ---- 旧 API 兼容层 ----
	ToolCallErrInvalidID = "invalid_tool_id"
	// ---- 旧 API 兼容层 ----
	ToolCallErrParseArgs = "parse_arguments"
	// ---- 旧 API 兼容层 ----
	ToolCallErrNoConnection = "no_connection"
	// ---- 旧 API 兼容层 ----
	ToolCallErrNoBuiltinReg = "no_builtin_registry"
	// ---- 旧 API 兼容层 ----
	ToolCallErrBuiltinNotFound = "builtin_tool_not_found"
	// ---- 旧 API 兼容层 ----
	ToolCallErrRemoteCall = "remote_call"
	// ---- 旧 API 兼容层 ----
	ToolCallErrEmptyToolName = "empty_tool_name"
)

// ---- 旧 API 兼容层：保留旧错误类型供遗留代码使用
// 新代码请使用 mcp.ExternalCallResult，禁止在新代码中新增对 ToolCallError 的引用。
func NewToolCallError(kind, message string, err ...error) *ToolCallError {
	tce := &ToolCallError{
		Kind:    kind,
		Message: message,
	}
	if len(err) > 0 {
		tce.Err = err[0]
	}
	return tce
}

type ToolSet struct {
	Connections      map[string]*client.Client
	Tools            []*protocol.Tool
	BuiltinTools     *tool.ToolRegistry
	WebSearchEnabled bool
}

func (ts *ToolSet) ExecuteToolWithContext(tec *ToolExecutionContext, toolId string, arguments map[string]interface{}) *ExternalCallResult {
	serverName, toolName, err := GetServerNameAndToolNameFromId(toolId)
	if err != nil {
		return NewExternalCallResultError(ErrKindInvalidID, err.Error(), err)
	}

	if toolName == "" {
		return NewExternalCallResultError(ErrKindEmptyToolName, "tool name is empty after parsing")
	}

	ctx, cancel := tec.GetTimeoutContext()
	defer cancel()

	if serverName == "" {
		if ts.BuiltinTools == nil {
			return NewExternalCallResultError(ErrKindNoBuiltinReg,
				fmt.Sprintf("builtin tool registry is nil; cannot execute builtin tool: %s", toolName))
		}
		result, execErr := ts.BuiltinTools.ExecuteTool(ctx, toolName, arguments)
		return CallToolResultToExternalResult(result, execErr)
	}

	conn, ok := ts.Connections[serverName]
	if !ok {
		return NewExternalCallResultError(ErrKindNoConnection,
			fmt.Sprintf("no open MCP connection for server: %s (tool: %s)", serverName, toolName))
	}

	req := &protocol.CallToolRequest{
		Name:      toolName,
		Arguments: arguments,
	}
	// ---- SDK 原始调用边界 ----
	result, execErr := conn.CallTool(ctx, req)
	// --------------------------
	return CallToolResultToExternalResult(result, execErr)
}

// ---- 旧 API 兼容层 ----
// 新代码请使用 ExecuteToolWithContext 返回 ExternalCallResult。
// 本函数仅保留向后兼容，内部直接委托给 ExecuteToolWithContext。
func (ts *ToolSet) ExecuteTool(ctx context.Context, toolId string, arguments map[string]interface{}) (*protocol.CallToolResult, error) {
	tec := NewToolExecutionContext(ctx)
	result := ts.ExecuteToolWithContext(tec, toolId, arguments)
	if !result.Success {
		return nil, NewToolCallError(string(result.ErrorKind), result.Error, result.Err)
	}
	return result.RawResult, nil
}

func IsToolCallError(err error) (*ToolCallError, bool) {
	var tce *ToolCallError
	if errors.As(err, &tce) {
		return tce, true
	}
	return nil, false
}

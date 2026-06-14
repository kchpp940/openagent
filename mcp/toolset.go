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

const (
	ToolCallErrInvalidID       = "invalid_tool_id"
	ToolCallErrParseArgs        = "parse_arguments"
	ToolCallErrNoConnection  = "no_connection"
	ToolCallErrNoBuiltinReg   = "no_builtin_registry"
	ToolCallErrBuiltinNotFound = "builtin_tool_not_found"
	ToolCallErrRemoteCall  = "remote_call"
	ToolCallErrEmptyToolName  = "empty_tool_name"
)

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
	Connections map[string]*client.Client
	Tools       []*protocol.Tool
	BuiltinTools     *tool.ToolRegistry
	WebSearchEnabled bool
}

func (ts *ToolSet) ExecuteTool(ctx context.Context, toolId string, arguments map[string]interface{}) (*protocol.CallToolResult, error) {
	serverName, toolName, err := GetServerNameAndToolNameFromId(toolId)
	if err != nil {
		return nil, NewToolCallError(ToolCallErrInvalidID, err.Error())
	}

	if toolName == "" {
		return nil, NewToolCallError(ToolCallErrEmptyToolName, "tool name is empty after parsing")
	}

	if serverName == "" {
		if ts.BuiltinTools == nil {
			return nil, NewToolCallError(ToolCallErrNoBuiltinReg,
				fmt.Sprintf("builtin tool registry is nil; cannot execute builtin tool: %s", toolName))
		}
		result, execErr := ts.BuiltinTools.ExecuteTool(ctx, toolName, arguments)
		if execErr != nil {
			return nil, NewToolCallError(ToolCallErrBuiltinNotFound, execErr.Error(), execErr)
		}
		return result, nil
	}

	conn, ok := ts.Connections[serverName]
	if !ok {
		return nil, NewToolCallError(ToolCallErrNoConnection,
			fmt.Sprintf("no open MCP connection for server: %s (tool: %s)", serverName, toolName))
	}

	req := &protocol.CallToolRequest{
		Name:      toolName,
		Arguments: arguments,
	}
	result, execErr := conn.CallTool(ctx, req)
	if execErr != nil {
		return nil, NewToolCallError(ToolCallErrRemoteCall,
			fmt.Sprintf("remote MCP call failed for server %s tool %s", serverName, toolName),
			execErr)
	}
	return result, nil
}

func IsToolCallError(err error) (*ToolCallError, bool) {
	var tce *ToolCallError
	if errors.As(err, &tce) {
		return tce, true
	}
	return nil, false
}

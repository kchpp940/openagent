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
	"sync"

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
	ToolCallErrInvalidID        = "invalid_tool_id"
	ToolCallErrParseArgs        = "parse_arguments"
	ToolCallErrNoConnection     = "no_connection"
	ToolCallErrNoBuiltinReg     = "no_builtin_registry"
	ToolCallErrBuiltinNotFound  = "builtin_tool_not_found"
	ToolCallErrRemoteCall       = "remote_call"
	ToolCallErrEmptyToolName    = "empty_tool_name"
	ToolCallErrToolNotAvailable = "tool_not_available"
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
	mu               sync.RWMutex
	Connections      map[string]*client.Client
	Tools            []*protocol.Tool
	BuiltinTools     *tool.ToolRegistry
	WebSearchEnabled bool
	toolIdToMeta     map[string]ToolIdMetadata
}

func NewToolSet() *ToolSet {
	return &ToolSet{
		Connections:  make(map[string]*client.Client),
		toolIdToMeta: make(map[string]ToolIdMetadata),
	}
}

func (ts *ToolSet) RegisterToolId(toolId string, md ToolIdMetadata) {
	if ts == nil {
		return
	}
	ts.mu.Lock()
	defer ts.mu.Unlock()
	if ts.toolIdToMeta == nil {
		ts.toolIdToMeta = make(map[string]ToolIdMetadata)
	}
	ts.toolIdToMeta[toolId] = md
	RegisterToolIdMetadata(toolId, md)
}

func (ts *ToolSet) LookupToolId(toolId string) (ToolIdMetadata, bool) {
	if ts == nil {
		return ToolIdMetadata{}, false
	}
	ts.mu.RLock()
	defer ts.mu.RUnlock()
	if ts.toolIdToMeta != nil {
		if md, ok := ts.toolIdToMeta[toolId]; ok {
			return md, true
		}
	}
	md, ok := GetToolIdMetadata(toolId)
	return md, ok
}

func (ts *ToolSet) AddServerTools(serverName string, tools []*protocol.Tool, conn *client.Client) error {
	if ts == nil {
		return errors.New("ToolSet is nil")
	}
	ts.mu.Lock()
	defer ts.mu.Unlock()
	if ts.Connections == nil {
		ts.Connections = make(map[string]*client.Client)
	}
	if ts.toolIdToMeta == nil {
		ts.toolIdToMeta = make(map[string]ToolIdMetadata)
	}
	if conn != nil {
		ts.Connections[serverName] = conn
	}
	for _, t := range tools {
		toolId, err := GetIdFromServerNameAndToolName(serverName, t.Name)
		if err != nil {
			return fmt.Errorf("cannot build tool id for server=%s tool=%s: %w", serverName, t.Name, err)
		}
		toolCopy := *t
		toolCopy.Name = toolId
		ts.Tools = append(ts.Tools, &toolCopy)
		md := ToolIdMetadata{ServerName: serverName, ToolName: t.Name}
		ts.toolIdToMeta[toolId] = md
		RegisterToolIdMetadata(toolId, md)
	}
	return nil
}

func (ts *ToolSet) AddBuiltinTools(builtinReg *tool.ToolRegistry) {
	if ts == nil {
		return
	}
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.BuiltinTools = builtinReg
	if ts.toolIdToMeta == nil {
		ts.toolIdToMeta = make(map[string]ToolIdMetadata)
	}
	if builtinReg == nil {
		return
	}
	protoTools := builtinReg.GetToolsAsProtocolTools()
	for _, pt := range protoTools {
		toolId, err := GetIdFromServerNameAndToolName("", pt.Name)
		if err != nil {
			continue
		}
		toolCopy := *pt
		toolCopy.Name = toolId
		alreadyExists := false
		for _, existing := range ts.Tools {
			if existing.Name == toolId {
				alreadyExists = true
				break
			}
		}
		if !alreadyExists {
			ts.Tools = append(ts.Tools, &toolCopy)
		}
		md := ToolIdMetadata{ServerName: "", ToolName: pt.Name}
		ts.toolIdToMeta[toolId] = md
		RegisterToolIdMetadata(toolId, md)
	}
}

func (ts *ToolSet) resolveToolId(toolId string) (string, string, error) {
	if md, ok := ts.LookupToolId(toolId); ok && md.ToolName != "" {
		return md.ServerName, md.ToolName, nil
	}
	serverName, toolName, err := GetServerNameAndToolNameFromId(toolId)
	if err != nil {
		return "", "", err
	}
	if toolName != "" {
		ts.RegisterToolId(toolId, ToolIdMetadata{ServerName: serverName, ToolName: toolName})
	}
	return serverName, toolName, nil
}

func (ts *ToolSet) ExecuteTool(ctx context.Context, toolId string, arguments map[string]interface{}) (*protocol.CallToolResult, error) {
	if ts == nil {
		return nil, NewToolCallError(ToolCallErrToolNotAvailable, "ToolSet is not initialized")
	}

	serverName, toolName, err := ts.resolveToolId(toolId)
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

	ts.mu.RLock()
	conn, ok := ts.Connections[serverName]
	ts.mu.RUnlock()
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

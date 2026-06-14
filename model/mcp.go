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

package model

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/ThinkInAIXYZ/go-mcp/protocol"
	"github.com/openai/openai-go/v2/responses"
	"github.com/sashabaranov/go-openai"
	"github.com/the-open-agent/openagent/i18n"
	"github.com/the-open-agent/openagent/mcp"
)

type ToolMessages struct {
	Messages         []*RawMessage
	ReasoningContent string
	ToolCalls        any
}

type ToolSession struct {
	McpToolSet   *mcp.ToolSet
	ToolMessages *ToolMessages
}

type ToolCallResponse struct {
	Success    bool              `json:"success"`
	Data       interface{}       `json:"data"`
	Error      string            `json:"error,omitempty"`
	ToolName   string            `json:"toolName"`
	ServerName string            `json:"serverName,omitempty"`
	ToolMeta   map[string]string `json:"toolMeta,omitempty"`
}

type ToolCall struct {
	Name       string            `json:"name"`
	Arguments  string            `json:"arguments"`
	Content    string            `json:"content"`
	IsError    bool              `json:"isError"`
	ServerName string            `json:"serverName,omitempty"`
	ToolName   string            `json:"toolName,omitempty"`
	ToolMeta   map[string]string `json:"toolMeta,omitempty"`
}

const toolErrorRecoveryPrompt = "The previous tool call failed. Do not finish as if the task is complete. If possible, recover by calling the appropriate tool again with corrected arguments or a different tool. If recovery is not possible, clearly explain what is blocked and what input or condition is needed."

type ToolCallDelta struct {
	Index          int    `json:"index"`
	ID             string `json:"id,omitempty"`
	Name           string `json:"name,omitempty"`
	ArgumentsDelta string `json:"argumentsDelta,omitempty"`
}

func flushToolCallDelta(index int, id string, name string, argumentsDelta string, writer io.Writer, lang string) error {
	if name == "" && argumentsDelta == "" {
		return nil
	}

	payload, err := json.Marshal(ToolCallDelta{
		Index:          index,
		ID:             id,
		Name:           name,
		ArgumentsDelta: argumentsDelta,
	})
	if err != nil {
		return err
	}
	return flushDataThink(string(payload), "tool-delta", writer, lang)
}

func reverseToolsToOpenAi(tools []*protocol.Tool) ([]openai.Tool, error) {
	var openaiTools []openai.Tool
	for _, tool := range tools {
		schemaBytes, err := json.Marshal(tool.InputSchema)
		if err != nil {
			return nil, err
		}

		var parameters map[string]interface{}
		if err := json.Unmarshal(schemaBytes, &parameters); err != nil {
			return nil, err
		}
		normalizeToolParametersSchema(parameters)
		openaiTools = append(openaiTools, openai.Tool{
			Type: "function",
			Function: &openai.FunctionDefinition{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  parameters,
			},
		})
	}
	return openaiTools, nil
}

func normalizeToolParametersSchema(parameters map[string]interface{}) {
	if parameters["type"] == "object" {
		if _, ok := parameters["properties"]; !ok {
			parameters["properties"] = map[string]interface{}{}
		}
	}
}

func handleToolCallsParameters(toolCall openai.ToolCall, toolCalls []openai.ToolCall, toolCallsMap map[int]int) ([]openai.ToolCall, map[int]int) {
	if toolCallsMap == nil {
		toolCallsMap = make(map[int]int)
	}

	idx := *toolCall.Index
	if existingIdx, exists := toolCallsMap[idx]; exists {
		if toolCall.Function.Name != "" {
			toolCalls[existingIdx].Function.Name = toolCall.Function.Name
		}
		if toolCall.Function.Arguments != "" {
			toolCalls[existingIdx].Function.Arguments += toolCall.Function.Arguments
		}
	} else {
		newIdx := len(toolCalls)
		toolCallsMap[idx] = newIdx
		toolCalls = append(toolCalls, toolCall)
	}
	return toolCalls, toolCallsMap
}

func normalizeToolCalls(toolSession *ToolSession) []openai.ToolCall {
	if toolSession.ToolMessages.ToolCalls == nil {
		return nil
	}
	toolCalls, ok := toolSession.ToolMessages.ToolCalls.([]openai.ToolCall)
	if ok {
		return toolCalls
	}
	responseFunctionToolCalls, ok := toolSession.ToolMessages.ToolCalls.([]responses.ResponseFunctionToolCall)
	if !ok {
		return nil
	}
	result := make([]openai.ToolCall, 0, len(responseFunctionToolCalls))
	for _, tc := range responseFunctionToolCalls {
		result = append(result, openai.ToolCall{
			ID:       tc.ID,
			Type:     "function",
			Function: openai.FunctionCall{Name: tc.Name, Arguments: tc.Arguments},
		})
	}
	return result
}

func resolveAndEnrichToolCall(toolCall openai.ToolCall, mcpToolSet *mcp.ToolSet, extraMeta ...map[string]string) (string, string, map[string]string) {
	var serverName, toolName string
	toolMeta := make(map[string]string)

	for _, m := range extraMeta {
		for k, v := range m {
			if v != "" {
				toolMeta[k] = v
			}
		}
	}

	if s, ok := toolMeta["serverName"]; ok && s != "" {
		serverName = s
	}
	if t, ok := toolMeta["toolName"]; ok && t != "" {
		toolName = t
	}

	id := toolCall.Function.Name
	if id == "" {
		id = toolMeta["toolId"]
	}
	if id != "" {
		toolMeta["toolId"] = id
	}

	if serverName != "" && toolName != "" && id != "" {
		if mcpToolSet != nil {
			mcpToolSet.HydrateFromMetadata(id, mcp.ToolIdMetadata{
				ServerName: serverName,
				ToolName:   toolName,
			})
		} else {
			mcp.RegisterToolIdMetadata(id, mcp.ToolIdMetadata{
				ServerName: serverName,
				ToolName:   toolName,
			})
		}
	}

	if (serverName == "" || toolName == "") && mcpToolSet != nil && id != "" {
		if md, ok := mcpToolSet.LookupToolId(id); ok && md.ToolName != "" {
			if serverName == "" {
				serverName = md.ServerName
			}
			if toolName == "" {
				toolName = md.ToolName
			}
		}
	}

	if (serverName == "" || toolName == "") && id != "" {
		if md, ok := mcp.GetToolIdMetadata(id); ok && md.ToolName != "" {
			if serverName == "" {
				serverName = md.ServerName
			}
			if toolName == "" {
				toolName = md.ToolName
			}
		}
	}

	if toolName == "" && id != "" {
		s, t, parseErr := mcp.GetServerNameAndToolNameFromId(id)
		if parseErr == nil && t != "" {
			if serverName == "" {
				serverName = s
			}
			toolName = t
			md := mcp.ToolIdMetadata{ServerName: serverName, ToolName: toolName}
			if mcpToolSet != nil {
				mcpToolSet.RegisterToolId(id, md)
			} else {
				mcp.RegisterToolIdMetadata(id, md)
			}
		}
	}

	if serverName != "" {
		toolMeta["serverName"] = serverName
	}
	if toolName != "" {
		toolMeta["toolName"] = toolName
	}
	return serverName, toolName, toolMeta
}

func QueryTextWithTools(p ModelProvider, question string, writer io.Writer, history []*RawMessage, prompt string, knowledgeMessages []*RawMessage, toolSession *ToolSession, lang string) (*ModelResult, error) {
	var messages []*RawMessage

	toolCount := 0
	if toolSession.McpToolSet != nil {
		toolCount = len(toolSession.McpToolSet.Tools)
		if toolSession.McpToolSet.WebSearchEnabled {
			toolCount++
		}
	}
	fmt.Printf("\n--- LLM Call (Round 0) | Tools available: [%d] ---\n", toolCount)

	modelResult, err := p.QueryText(question, writer, history, prompt, knowledgeMessages, toolSession, lang)
	if err != nil {
		return nil, err
	}

	toolCalls := normalizeToolCalls(toolSession)
	if len(toolCalls) == 0 {
		fmt.Printf("LLM Decision: [Final Answer — no tool calls]\n")
		return modelResult, nil
	}

	round := 0
	for len(toolCalls) > 0 {
		round++
		fmt.Printf("\n--- Agent Round %d | LLM Decision: [%d tool call(s)] ---\n", round, len(toolCalls))
		for i, tc := range toolCalls {
			fmt.Printf("  Tool %d: [%s] args: %s\n", i+1, tc.Function.Name, tc.Function.Arguments)
		}

		roundHasToolError := false
		for _, toolCall := range toolCalls {
			serverName, toolName, toolMeta := resolveAndEnrichToolCall(toolCall, toolSession.McpToolSet)

			messages = append(messages, &RawMessage{
				Text:             "Call result from " + toolCall.Function.Name,
				Author:           "AI",
				ReasoningContent: toolSession.ToolMessages.ReasoningContent,
				ToolCall:         toolCall,
			})

			var toolFailed bool
			messages, toolFailed, err = callMcpTool(toolCall, serverName, toolName, toolMeta, toolSession.McpToolSet, messages, writer, lang)
			if err != nil {
				return nil, err
			}
			if toolFailed {
				roundHasToolError = true
			}
		}

		toolSession.ToolMessages.Messages = messages
		fmt.Printf("\n--- LLM Call (Round %d) | Tool results fed back ---\n", round)
		modelResult, err = p.QueryText(question, writer, history, prompt, knowledgeMessages, toolSession, lang)
		if err != nil {
			return nil, err
		}

		toolCalls = normalizeToolCalls(toolSession)
		if len(toolCalls) == 0 && roundHasToolError {
			messages = append(messages, &RawMessage{
				Text:   toolErrorRecoveryPrompt,
				Author: "System",
			})
			toolSession.ToolMessages.Messages = messages

			fmt.Printf("\n--- LLM Call (Round %d recovery) | Tool error recovery prompt added ---\n", round)
			modelResult, err = p.QueryText(question, writer, history, prompt, knowledgeMessages, toolSession, lang)
			if err != nil {
				return nil, err
			}

			toolCalls = normalizeToolCalls(toolSession)
		}
	}

	fmt.Printf("LLM Decision: [Final Answer — no more tool calls after round %d]\n", round)

	if toolSession.McpToolSet != nil {
		for _, conn := range toolSession.McpToolSet.Connections {
			conn.Close()
		}
	}
	return modelResult, nil
}

func createToolMessage(toolCall openai.ToolCall, text string) *RawMessage {
	return &RawMessage{
		Text:       text,
		Author:     "Tool",
		ToolCallID: toolCall.ID,
	}
}

func startHeartbeat(writer io.Writer, mu *sync.Mutex) chan<- struct{} {
	stop := make(chan struct{})
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				mu.Lock()
				if flusher, ok := writer.(http.Flusher); ok {
					_, _ = fmt.Fprint(writer, ":keepalive\n\n")
					flusher.Flush()
				}
				mu.Unlock()
			case <-stop:
				return
			}
		}
	}()
	return stop
}

func callMcpTool(toolCall openai.ToolCall, serverName, toolName string, toolMeta map[string]string, mcpToolSet *mcp.ToolSet, messages []*RawMessage, writer io.Writer, lang string) ([]*RawMessage, bool, error) {
	var arguments map[string]interface{}
	ctx := context.Background()

	displayToolName := toolName
	if displayToolName == "" {
		displayToolName = toolCall.Function.Name
	}

	toolStartData := ToolCall{
		Name:       toolCall.Function.Name,
		Arguments:  toolCall.Function.Arguments,
		Content:    "",
		IsError:    false,
		ServerName: serverName,
		ToolName:   toolName,
		ToolMeta:   toolMeta,
	}
	toolStartJSON, _ := json.Marshal(toolStartData)
	if len(toolStartJSON) > 0 {
		_ = flushDataThink(string(toolStartJSON), "tool-start", writer, lang)
	}

	if toolName == "" {
		errMsg := fmt.Sprintf(i18n.Translate(lang, "model:invalid tool id: cannot resolve server/tool name from id %s"), toolCall.Function.Name)
		return emitToolError(toolCall, serverName, toolName, toolMeta, mcp.ToolCallErrInvalidID, errMsg, messages, writer, lang)
	}

	if parseErr := json.Unmarshal([]byte(toolCall.Function.Arguments), &arguments); parseErr != nil {
		errMsg := fmt.Sprintf(i18n.Translate(lang, "model:failed to parse tool arguments: %v"), parseErr)
		return emitToolError(toolCall, serverName, toolName, toolMeta, mcp.ToolCallErrParseArgs, errMsg, messages, writer, lang)
	}

	var mu sync.Mutex
	var result *protocol.CallToolResult
	var execErr error

	heartbeat := startHeartbeat(writer, &mu)
	defer close(heartbeat)

	if mcpToolSet == nil {
		errMsg := i18n.Translate(lang, "model:MCP toolset is not initialized")
		return emitToolError(toolCall, serverName, toolName, toolMeta, mcp.ToolCallErrToolNotAvailable, errMsg, messages, writer, lang)
	}

	result, execErr = mcpToolSet.ExecuteTool(ctx, toolCall.Function.Name, arguments)

	response := &ToolCallResponse{
		ToolName:   displayToolName,
		ServerName: serverName,
		ToolMeta:   toolMeta,
	}

	if execErr != nil {
		response.Success = false
		if tce, ok := mcp.IsToolCallError(execErr); ok {
			response.Error = fmt.Sprintf("[%s] %s", tce.Kind, tce.Message)
		} else {
			response.Error = execErr.Error()
		}
	} else if result == nil {
		response.Success = false
		response.Error = i18n.Translate(lang, "model:tool returned nil result")
	} else if result.IsError {
		response.Success = false
		contentBytes, marshalErr := json.Marshal(result.Content)
		if marshalErr != nil {
			response.Error = fmt.Sprintf(i18n.Translate(lang, "model:failed to marshal error content: %v"), marshalErr)
		} else {
			response.Error = string(contentBytes)
		}
	} else {
		response.Success = true
		contentBytes, marshalErr := json.Marshal(result.Content)
		if marshalErr != nil {
			response.Success = false
			response.Error = fmt.Sprintf(i18n.Translate(lang, "model:failed to marshal content: %v"), marshalErr)
		} else {
			response.Data = string(contentBytes)
		}
	}

	responseJson, marshalErr := json.Marshal(response)
	if marshalErr != nil {
		return nil, false, fmt.Errorf(i18n.Translate(lang, "model:failed to marshal tool response: %v"), marshalErr)
	}

	var contentStr string
	if !response.Success {
		contentStr = response.Error
	} else {
		if dataStr, ok := response.Data.(string); ok {
			contentStr = dataStr
		} else {
			dataBytes, _ := json.Marshal(response.Data)
			contentStr = string(dataBytes)
		}
	}

	fmt.Printf("Tool Result: [%s] server=%s tool=%s\n", contentStr, serverName, toolName)
	isError := !response.Success

	toolData := ToolCall{
		Name:       toolCall.Function.Name,
		Arguments:  toolCall.Function.Arguments,
		Content:    contentStr,
		IsError:    isError,
		ServerName: serverName,
		ToolName:   toolName,
		ToolMeta:   toolMeta,
	}
	toolJSON, _ := json.Marshal(toolData)
	if len(toolJSON) > 0 {
		mu.Lock()
		_ = flushDataThink(string(toolJSON), "tool", writer, lang)
		mu.Unlock()
	}

	messages = append(messages, createToolMessage(toolCall, string(responseJson)))
	return messages, !response.Success, nil
}

func emitToolError(toolCall openai.ToolCall, serverName, toolName string, toolMeta map[string]string, errKind, errMsg string, messages []*RawMessage, writer io.Writer, lang string) ([]*RawMessage, bool, error) {
	displayToolName := toolName
	if displayToolName == "" {
		displayToolName = toolCall.Function.Name
	}
	response := &ToolCallResponse{
		Success:    false,
		ToolName:   displayToolName,
		ServerName: serverName,
		ToolMeta:   toolMeta,
		Error:      fmt.Sprintf("[%s] %s", errKind, errMsg),
	}

	responseJson, marshalErr := json.Marshal(response)
	if marshalErr != nil {
		return nil, false, fmt.Errorf(i18n.Translate(lang, "model:failed to marshal tool error response: %v"), marshalErr)
	}

	contentStr := response.Error
	fmt.Printf("Tool Error [%s] server=%s tool=%s: %s\n", errKind, serverName, toolName, contentStr)

	toolData := ToolCall{
		Name:       toolCall.Function.Name,
		Arguments:  toolCall.Function.Arguments,
		Content:    contentStr,
		IsError:    true,
		ServerName: serverName,
		ToolName:   toolName,
		ToolMeta:   toolMeta,
	}
	toolJSON, _ := json.Marshal(toolData)
	if len(toolJSON) > 0 {
		_ = flushDataThink(string(toolJSON), "tool", writer, lang)
	}

	messages = append(messages, createToolMessage(toolCall, string(responseJson)))
	return messages, true, nil
}

func GetToolCallsFromWriter(toolMessage string) []ToolCall {
	if toolMessage == "" {
		return nil
	}
	var toolCalls []ToolCall
	toolCallLines := strings.Split(toolMessage, "\n")
	for _, line := range toolCallLines {
		if line == "" {
			continue
		}
		var toolCall ToolCall
		if err := json.Unmarshal([]byte(line), &toolCall); err == nil {
			if toolCall.ToolName == "" || toolCall.ServerName == "" {
				serverName, toolName, parseErr := mcp.GetServerNameAndToolNameFromId(toolCall.Name)
				if parseErr == nil && toolName != "" {
					if toolCall.ToolName == "" {
						toolCall.ToolName = toolName
					}
					if toolCall.ServerName == "" {
						toolCall.ServerName = serverName
					}
					if toolCall.ToolMeta == nil {
						toolCall.ToolMeta = make(map[string]string)
					}
					if _, ok := toolCall.ToolMeta["toolId"]; !ok {
						toolCall.ToolMeta["toolId"] = toolCall.Name
					}
					toolCall.ToolMeta["serverName"] = serverName
					toolCall.ToolMeta["toolName"] = toolName
					mcp.RegisterToolIdMetadata(toolCall.Name, mcp.ToolIdMetadata{
						ServerName: serverName,
						ToolName:   toolName,
					})
				}
			}
			toolCalls = append(toolCalls, toolCall)
		}
	}
	return toolCalls
}

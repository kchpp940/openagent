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
	Success  bool        `json:"success"`
	Data     interface{} `json:"data"`
	Error    string      `json:"error,omitempty"`
	ToolName string      `json:"toolName"`
}

type ToolCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
	Content   string `json:"content"`
	IsError   bool   `json:"isError"`
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
			serverName, toolName, parseErr := mcp.GetServerNameAndToolNameFromId(toolCall.Function.Name)

			messages = append(messages, &RawMessage{
				Text:             "Call result from " + toolCall.Function.Name,
				Author:           "AI",
				ReasoningContent: toolSession.ToolMessages.ReasoningContent,
				ToolCall:         toolCall,
			})

			var toolFailed bool
			if parseErr != nil {
				messages, toolFailed, err = handleToolIdParseError(toolCall, parseErr, messages, writer, lang)
				if err != nil {
					return nil, err
				}
				if toolFailed {
					roundHasToolError = true
				}
			} else {
				messages, toolFailed, err = callMcpTool(toolCall, serverName, toolName, toolSession.McpToolSet, messages, writer, lang)
				if err != nil {
					return nil, err
				}
				if toolFailed {
					roundHasToolError = true
				}
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

func callMcpTool(toolCall openai.ToolCall, serverName, toolName string, mcpToolSet *mcp.ToolSet, messages []*RawMessage, writer io.Writer, lang string) ([]*RawMessage, bool, error) {
	ctx := context.Background()

	// ---- 流式输出边界：tool-start 事件 ----
	toolStartData := ToolCall{
		Name:      toolCall.Function.Name,
		Arguments: toolCall.Function.Arguments,
		Content:   "",
		IsError:   false,
	}
	toolStartJSON, _ := json.Marshal(toolStartData)
	if len(toolStartJSON) > 0 {
		_ = flushDataThink(string(toolStartJSON), "tool-start", writer, lang)
	}
	// ---- 流式输出边界结束 ----

	arguments, parseErr := mcp.ParseToolArguments(toolCall.Function.Arguments)
	if parseErr != nil {
		extResult := mcp.NewExternalCallResultError(mcp.ErrKindParseArgs,
			fmt.Sprintf(i18n.Translate(lang, "model:failed to parse tool arguments: %v"), parseErr), parseErr)
		return emitToolResult(toolCall, extResult, messages, writer, lang)
	}

	var mu sync.Mutex

	// ---- 流式输出边界：heartbeat 保活 ----
	heartbeat := startHeartbeat(writer, &mu)
	defer close(heartbeat)
	// ---- 流式输出边界结束 ----

	var extResult *mcp.ExternalCallResult

	if mcpToolSet == nil {
		extResult = mcp.NewExternalCallResultError(mcp.ErrKindNoBuiltinReg,
			i18n.Translate(lang, "model:MCP toolset is not initialized"))
	} else if serverName == "" {
		if mcpToolSet.BuiltinTools == nil {
			extResult = mcp.NewExternalCallResultError(mcp.ErrKindNoBuiltinReg,
				fmt.Sprintf(i18n.Translate(lang, "model:builtin tool registry is not available; cannot execute tool: %s"), toolName))
		} else {
			result, execErr := mcpToolSet.BuiltinTools.ExecuteTool(ctx, toolName, arguments)
			extResult = mcp.CallToolResultToExternalResult(result, execErr)
		}
	} else {
		conn, ok := mcpToolSet.Connections[serverName]
		if !ok {
			extResult = mcp.NewExternalCallResultError(mcp.ErrKindNoConnection,
				fmt.Sprintf(i18n.Translate(lang, "model:no open MCP connection for server: %s (tool: %s)"), serverName, toolName))
		} else {
			req := &protocol.CallToolRequest{
				Name:      toolName,
				Arguments: arguments,
			}
			result, execErr := conn.CallTool(ctx, req)
			extResult = mcp.CallToolResultToExternalResult(result, execErr)
		}
	}

	return emitToolResult(toolCall, extResult, messages, writer, lang)
}

func emitToolResult(toolCall openai.ToolCall, extResult *mcp.ExternalCallResult, messages []*RawMessage, writer io.Writer, lang string) ([]*RawMessage, bool, error) {
	var mu sync.Mutex

	response := &ToolCallResponse{
		Success:  extResult.Success,
		ToolName: toolCall.Function.Name,
	}
	if extResult.Success {
		response.Data = extResult.Data
	} else {
		response.Error = extResult.ErrorWithKind()
	}

	responseJson, marshalErr := json.Marshal(response)
	if marshalErr != nil {
		return nil, false, fmt.Errorf(i18n.Translate(lang, "model:failed to marshal tool response: %v"), marshalErr)
	}

	var contentStr string
	if !extResult.Success {
		contentStr = extResult.ErrorWithKind()
	} else {
		contentStr = extResult.Data
	}

	fmt.Printf("Tool Result: [%s]\n", contentStr)

	// ---- 流式输出边界：tool 进度事件 ----
	toolData := ToolCall{
		Name:      toolCall.Function.Name,
		Arguments: toolCall.Function.Arguments,
		Content:   contentStr,
		IsError:   !extResult.Success,
	}
	toolJSON, _ := json.Marshal(toolData)
	if len(toolJSON) > 0 {
		mu.Lock()
		_ = flushDataThink(string(toolJSON), "tool", writer, lang)
		mu.Unlock()
	}
	// ---- 流式输出边界结束 ----

	messages = append(messages, createToolMessage(toolCall, string(responseJson)))
	return messages, !extResult.Success, nil
}

func handleToolIdParseError(toolCall openai.ToolCall, parseErr error, messages []*RawMessage, writer io.Writer, lang string) ([]*RawMessage, bool, error) {
	// ---- 流式输出边界：tool-start 事件 ----
	toolStartData := ToolCall{
		Name:      toolCall.Function.Name,
		Arguments: toolCall.Function.Arguments,
		Content:   "",
		IsError:   false,
	}
	toolStartJSON, _ := json.Marshal(toolStartData)
	if len(toolStartJSON) > 0 {
		_ = flushDataThink(string(toolStartJSON), "tool-start", writer, lang)
	}
	// ---- 流式输出边界结束 ----

	extResult := mcp.NewExternalCallResultError(mcp.ErrKindInvalidID,
		fmt.Sprintf(i18n.Translate(lang, "model:invalid tool id: %v"), parseErr), parseErr)
	return emitToolResult(toolCall, extResult, messages, writer, lang)
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
			toolCalls = append(toolCalls, toolCall)
		}
	}
	return toolCalls
}

// Copyright 2026 The OpenAgent Authors. All Rights Reserved.
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

// ----------------------------------------------------------------------------
// MCP/Tool/Skill 统一执行上下文 —— 使用边界与开发规范
// ----------------------------------------------------------------------------
//
// 本模块提供 ToolExecutionContext、ExternalCallResult、SchemaParseResult
// 三类核心抽象，用于统一 MCP 工具调用、Schema 解析、错误包装、结果提取
// 的口径。所有业务入口（Server 同步、Tool 测试、Skill 加载、模型工具调用）
// 都必须通过本模块提供的上下文和结果类型完成，禁止在业务层重复实现
// 以下逻辑：
//
//   [必须走上下文] — 业务入口层（object/*, controllers/*, model/*）
//   - 远程 MCP 工具调用：使用 CallToolWithContext + ExternalCallResult
//   - MCP 工具列表同步：使用 ListToolsWithContext + ListToolsResult
//   - ToolSet 内工具执行：使用 ToolSet.ExecuteToolWithContext
//   - 工具参数 / 测试内容 JSON 解析：使用 ParseToolArguments / ParseTestContent
//   - JSON Schema 解析校验：使用 ParseInputSchema + SchemaParseResult
//   - 工具错误包装：使用 NewExternalCallResultError + ErrorKind 枚举
//   - 工具结果提取：使用 CallToolResultToExternalResult / ExtractTextFromResult
//   - 技能加载：使用 LoadSkillWithContext + SkillLoadResult
//
//   [允许保留] — 底层 SDK / 流式输出边界（必须用注释标注边界）
//   1. SDK 原始调用边界
//      - go-mcp 库的 client.Client.CallTool / ListTools 直接调用
//      - 位置：mcp/client.go, mcp/toolset.go, mcp/tools.go
//      - 标注：// ---- SDK 原始调用边界 ----
//   2. 流式输出边界
//      - model/mcp.go 中的 flushDataThink / heartbeat / tool-start / tool 事件
//      - 位置：model/mcp.go 的 callMcpTool / emitToolResult / handleToolIdParseError
//      - 标注：// ---- 流式输出边界：xxx ----
//   3. 工具定义层 schema 构造
//      - tool/builtin_tool/* 和 tool/*.go 中 GetInputSchema 的内存对象构造
//      - 位置：tool/builtin_tool/builtin_tool.go:GetToolsAsProtocolTools
//      - 标注：// ---- 工具定义层边界：interface{} -> protocol.InputSchema ----
//   4. Tool ID 编解码
//      - mcp/util.go 中的纯字符串编解码，与执行/错误处理无关
//      - 位置：mcp/util.go
//      - 标注：// ---- 纯编解码工具函数边界 ----
//   5. 旧 API 兼容层
//      - mcp.CallTool / mcp.GetToolsFromURL / object.LoadSkill 等旧签名
//      - 仅作为向后兼容包装，内部必须调用对应的 *WithContext 版本
//
//   [禁止新增] — 任何业务入口中出现以下模式均属于反模式
//   - 直接手写 json.Unmarshal(toolCall.Function.Arguments, &arguments)
//   - 直接手写 json.Marshal(result.Content) + 错误判断分支
//   - 直接手写 fmt.Errorf("[%s] %s", kind, msg) 式错误包装
//   - 直接手写 protocol.CallToolResult 的 Success / IsError 分支
//   - 新增一套 ErrorKind 字符串常量（必须复用本模块的 ErrorKind）
// ----------------------------------------------------------------------------

package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/ThinkInAIXYZ/go-mcp/protocol"
)

const (
	DefaultToolCallTimeout  = 90 * time.Second
	DefaultListToolsTimeout = 30 * time.Second
	DefaultSyncTimeout      = 30 * time.Second
)

type ErrorKind string

const (
	ErrKindInvalidID       ErrorKind = "invalid_tool_id"
	ErrKindParseArgs       ErrorKind = "parse_arguments"
	ErrKindNoConnection    ErrorKind = "no_connection"
	ErrKindNoBuiltinReg    ErrorKind = "no_builtin_registry"
	ErrKindBuiltinNotFound ErrorKind = "builtin_tool_not_found"
	ErrKindRemoteCall      ErrorKind = "remote_call"
	ErrKindEmptyToolName   ErrorKind = "empty_tool_name"
	ErrKindTimeout         ErrorKind = "timeout"
	ErrKindSchemaParse     ErrorKind = "schema_parse"
	ErrKindInvalidConfig   ErrorKind = "invalid_config"
	ErrKindNotFound        ErrorKind = "not_found"
)

type ToolExecutionContext struct {
	Ctx        context.Context
	Lang       string
	Owner      string
	ToolName   string
	ServerName string
	Timeout    time.Duration
	Extra      map[string]interface{}
}

func NewToolExecutionContext(ctx context.Context) *ToolExecutionContext {
	if ctx == nil {
		ctx = context.Background()
	}
	return &ToolExecutionContext{
		Ctx:   ctx,
		Extra: make(map[string]interface{}),
	}
}

func (tec *ToolExecutionContext) WithTimeout(timeout time.Duration) *ToolExecutionContext {
	tec.Timeout = timeout
	return tec
}

func (tec *ToolExecutionContext) WithLang(lang string) *ToolExecutionContext {
	tec.Lang = lang
	return tec
}

func (tec *ToolExecutionContext) WithOwner(owner string) *ToolExecutionContext {
	tec.Owner = owner
	return tec
}

func (tec *ToolExecutionContext) WithToolName(toolName string) *ToolExecutionContext {
	tec.ToolName = toolName
	return tec
}

func (tec *ToolExecutionContext) WithServerName(serverName string) *ToolExecutionContext {
	tec.ServerName = serverName
	return tec
}

func (tec *ToolExecutionContext) GetTimeoutContext() (context.Context, context.CancelFunc) {
	timeout := tec.Timeout
	if timeout <= 0 {
		timeout = DefaultToolCallTimeout
	}
	return context.WithTimeout(tec.Ctx, timeout)
}

type ExternalCallResult struct {
	Success   bool
	Data      string
	Error     string
	ErrorKind ErrorKind
	RawResult *protocol.CallToolResult
	Err       error
}

func NewExternalCallResultSuccess(data string) *ExternalCallResult {
	return &ExternalCallResult{
		Success: true,
		Data:    data,
	}
}

func NewExternalCallResultError(kind ErrorKind, message string, err ...error) *ExternalCallResult {
	res := &ExternalCallResult{
		Success:   false,
		ErrorKind: kind,
		Error:     message,
	}
	if len(err) > 0 {
		res.Err = err[0]
		if message == "" {
			res.Error = err[0].Error()
		}
	}
	return res
}

func (r *ExternalCallResult) ToJSON() (string, error) {
	type jsonResult struct {
		Success   bool      `json:"success"`
		Data      string    `json:"data,omitempty"`
		Error     string    `json:"error,omitempty"`
		ErrorKind ErrorKind `json:"errorKind,omitempty"`
	}
	j := jsonResult{
		Success:   r.Success,
		Data:      r.Data,
		Error:     r.Error,
		ErrorKind: r.ErrorKind,
	}
	b, err := json.Marshal(j)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (r *ExternalCallResult) ErrorWithKind() string {
	if r.ErrorKind == "" {
		return r.Error
	}
	return fmt.Sprintf("[%s] %s", r.ErrorKind, r.Error)
}

func (r *ExternalCallResult) Unwrap() error {
	return r.Err
}

type SchemaParseResult struct {
	Schema  interface{}
	RawJSON string
	Error   error
	IsValid bool
}

func ParseInputSchema(rawJSON string) *SchemaParseResult {
	result := &SchemaParseResult{
		RawJSON: rawJSON,
		IsValid: false,
	}

	if strings.TrimSpace(rawJSON) == "" {
		result.Error = fmt.Errorf("empty schema JSON")
		return result
	}

	var schema interface{}
	if err := json.Unmarshal([]byte(rawJSON), &schema); err != nil {
		result.Error = fmt.Errorf("failed to parse schema JSON: %w", err)
		return result
	}

	result.Schema = schema
	result.IsValid = true
	return result
}

func (r *SchemaParseResult) ToProtocolInputSchema() (*protocol.InputSchema, error) {
	if !r.IsValid || r.Error != nil {
		return nil, r.Error
	}

	schemaBytes, err := json.Marshal(r.Schema)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal schema: %w", err)
	}

	var inputSchema protocol.InputSchema
	if err := json.Unmarshal(schemaBytes, &inputSchema); err != nil {
		return nil, fmt.Errorf("failed to convert to protocol.InputSchema: %w", err)
	}

	return &inputSchema, nil
}

func WrapExternalError(kind ErrorKind, message string, err error) error {
	if err == nil {
		return NewToolCallError(string(kind), message)
	}
	return NewToolCallError(string(kind), fmt.Sprintf("%s: %v", message, err), err)
}

// ---- 公共模块内部实现：工具结果统一转换入口 ----
// 本函数是所有 MCP 工具调用结果的收口点，业务层禁止重复实现该逻辑。
func CallToolResultToExternalResult(result *protocol.CallToolResult, execErr error) *ExternalCallResult {
	if execErr != nil {
		var errKind ErrorKind
		if tce, ok := IsToolCallError(execErr); ok {
			errKind = ErrorKind(tce.Kind)
		} else {
			errKind = ErrKindRemoteCall
		}
		return NewExternalCallResultError(errKind, execErr.Error(), execErr)
	}

	if result == nil {
		return NewExternalCallResultError(ErrKindRemoteCall, "tool returned nil result")
	}

	// ---- 公共模块内部实现 ----
	if result.IsError {
		// ---- 公共模块内部实现 ----
		contentBytes, marshalErr := json.Marshal(result.Content)
		if marshalErr != nil {
			return NewExternalCallResultError(ErrKindRemoteCall,
				fmt.Sprintf("failed to marshal error content: %v", marshalErr), marshalErr)
		}
		return &ExternalCallResult{
			Success:   false,
			Error:     string(contentBytes),
			ErrorKind: ErrKindRemoteCall,
			RawResult: result,
		}
	}

	// ---- 公共模块内部实现 ----
	contentBytes, marshalErr := json.Marshal(result.Content)
	if marshalErr != nil {
		return NewExternalCallResultError(ErrKindRemoteCall,
			fmt.Sprintf("failed to marshal content: %v", marshalErr), marshalErr)
	}

	return &ExternalCallResult{
		Success:   true,
		Data:      string(contentBytes),
		RawResult: result,
	}
}

func ExtractTextFromResult(result *protocol.CallToolResult) string {
	if result == nil {
		return ""
	}
	var texts []string
	for _, c := range result.Content {
		if tc, ok := c.(*protocol.TextContent); ok {
			texts = append(texts, tc.Text)
		}
	}
	return strings.Join(texts, "\n")
}

type TestContentPayload struct {
	Tool      string                 `json:"tool"`
	Arguments map[string]interface{} `json:"arguments"`
}

// ---- 公共模块内部实现：测试内容统一解析入口 ----
// 本函数是所有测试内容 JSON 的统一解析入口，业务层禁止重复实现该逻辑。
func ParseTestContent(testContent string) (*TestContentPayload, error) {
	if strings.TrimSpace(testContent) == "" {
		return nil, fmt.Errorf("testContent is empty")
	}

	var payload TestContentPayload
	// ---- 公共模块内部实现 ----
	if err := json.Unmarshal([]byte(testContent), &payload); err != nil {
		return nil, fmt.Errorf("invalid test JSON: %w", err)
	}

	if strings.TrimSpace(payload.Tool) == "" {
		return nil, fmt.Errorf("test JSON must include non-empty \"tool\"")
	}

	if payload.Arguments == nil {
		payload.Arguments = map[string]interface{}{}
	}

	return &payload, nil
}

// ---- 公共模块内部实现：工具参数统一解析入口 ----
// 本函数是所有工具参数字符串的统一解析入口，业务层禁止重复实现该逻辑。
func ParseToolArguments(argumentsStr string) (map[string]interface{}, error) {
	if strings.TrimSpace(argumentsStr) == "" {
		return nil, fmt.Errorf("arguments string is empty")
	}

	var args map[string]interface{}
	if err := json.Unmarshal([]byte(argumentsStr), &args); err != nil {
		return nil, fmt.Errorf("failed to parse tool arguments: %w", err)
	}
	return args, nil
}

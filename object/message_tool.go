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

package object

import (
	"fmt"
	"strings"

	"github.com/the-open-agent/openagent/mcp"
	"github.com/the-open-agent/openagent/model"
	"github.com/the-open-agent/openagent/tool"
	"github.com/the-open-agent/openagent/util"
)

func buildToolSetForBuiltinTool(toolName, user, origin, lang string) (*mcp.ToolSet, error) {
	if toolName == "" {
		return nil, nil
	}

	id := util.GetIdFromOwnerAndName("admin", toolName)
	t, err := GetTool(id)
	if err != nil {
		return nil, err
	}
	if t == nil {
		return nil, nil
	}

	tp, err := tool.New(getToolConfig(t), lang)
	if err != nil {
		return nil, err
	}

	reg := tool.NewToolRegistry()
	for _, t := range tp.BuiltinTools() {
		wrapped := wrapSnapshotBuiltin("admin", t)
		wrapped = wrapGeneratedResourceBuiltin(wrapped, "admin", user, origin)
		reg.RegisterTool(wrapped)
	}

	allTools := reg.GetToolsAsProtocolTools()
	if len(allTools) == 0 {
		return nil, nil
	}

	toolSet := mcp.NewToolSet()
	toolSet.AddBuiltinTools(reg)
	return toolSet, nil
}

func HydrateToolCallMetadata(tcs []model.ToolCall) {
	for i := range tcs {
		tc := &tcs[i]
		if tc.ToolMeta == nil {
			tc.ToolMeta = make(map[string]string)
		}
		if tc.Name != "" {
			if _, ok := tc.ToolMeta["toolId"]; !ok {
				tc.ToolMeta["toolId"] = tc.Name
			}
		}

		if tc.ServerName != "" && tc.ToolName != "" {
			mcp.RegisterToolIdMetadata(tc.Name, mcp.ToolIdMetadata{
				ServerName: tc.ServerName,
				ToolName:   tc.ToolName,
			})
			tc.ToolMeta["serverName"] = tc.ServerName
			tc.ToolMeta["toolName"] = tc.ToolName
			continue
		}

		if s, ok := tc.ToolMeta["serverName"]; ok && s != "" && tc.ServerName == "" {
			tc.ServerName = s
		}
		if t, ok := tc.ToolMeta["toolName"]; ok && t != "" && tc.ToolName == "" {
			tc.ToolName = t
		}

		if tc.ServerName != "" && tc.ToolName != "" {
			mcp.RegisterToolIdMetadata(tc.Name, mcp.ToolIdMetadata{
				ServerName: tc.ServerName,
				ToolName:   tc.ToolName,
			})
			continue
		}

		serverName, toolName, parseErr := mcp.GetServerNameAndToolNameFromId(tc.Name)
		if parseErr == nil && toolName != "" {
			if tc.ServerName == "" {
				tc.ServerName = serverName
			}
			if tc.ToolName == "" {
				tc.ToolName = toolName
			}
			tc.ToolMeta["serverName"] = tc.ServerName
			tc.ToolMeta["toolName"] = tc.ToolName
			mcp.RegisterToolIdMetadata(tc.Name, mcp.ToolIdMetadata{
				ServerName: tc.ServerName,
				ToolName:   tc.ToolName,
			})
		}
	}
}

type ToolCallValidationIssue struct {
	Index   int
	ToolID  string
	Message string
}

func ValidateAndEnrichToolCalls(tcs []model.ToolCall, mcpToolSet *mcp.ToolSet) ([]ToolCallValidationIssue, bool) {
	var issues []ToolCallValidationIssue
	hasFatal := false

	HydrateToolCallMetadata(tcs)

	for i := range tcs {
		tc := &tcs[i]
		if tc.ToolMeta == nil {
			tc.ToolMeta = make(map[string]string)
		}
		if tc.Name != "" {
			tc.ToolMeta["toolId"] = tc.Name
		}

		missing := []string{}
		if tc.Name == "" {
			missing = append(missing, "toolId(name)")
		}
		if tc.ToolName == "" {
			missing = append(missing, "toolName")
		}

		if mcpToolSet != nil && tc.Name != "" {
			if md, ok := mcpToolSet.LookupToolId(tc.Name); ok && md.ToolName != "" {
				if tc.ServerName == "" {
					tc.ServerName = md.ServerName
				}
				if tc.ToolName == "" {
					tc.ToolName = md.ToolName
				}
			}
			mcpToolSet.HydrateFromMetadata(tc.Name, mcp.ToolIdMetadata{
				ServerName: tc.ServerName,
				ToolName:   tc.ToolName,
			})
		}

		if len(missing) > 0 {
			msg := fmt.Sprintf("missing metadata fields: %s", strings.Join(missing, ", "))
			if tc.Content == "" {
				tc.Content = fmt.Sprintf("[%s] %s for toolId=%s",
					mcp.ToolCallErrMissingMetadata, msg, tc.Name)
				tc.IsError = true
			}
			if tc.ToolName == "" && tc.Name != "" {
				tc.ToolMeta["toolName"] = "(unknown)"
			}
			hasFatal = true
			issues = append(issues, ToolCallValidationIssue{
				Index:   i,
				ToolID:  tc.Name,
				Message: msg,
			})
		}

		tc.ToolMeta["serverName"] = tc.ServerName
		tc.ToolMeta["toolName"] = tc.ToolName
		if tc.Name != "" {
			tc.ToolMeta["toolId"] = tc.Name
		}
	}

	return issues, hasFatal
}

func GetAnswerWithTool(modelProviderName, toolName, question, user, origin, lang string) (string, *model.ModelResult, error) {
	_, modelProviderObj, err := GetModelProviderFromContext("admin", modelProviderName, lang)
	if err != nil {
		return "", nil, err
	}

	mcpToolSet, err := buildToolSetForBuiltinTool(toolName, user, origin, lang)
	if err != nil {
		return "", nil, err
	}

	prompt := "You are an expert in your field and you specialize in using your knowledge to answer or solve people's problems."
	history := []*model.RawMessage{}
	knowledge := []*model.RawMessage{}

	var writer MyWriter
	var modelResult *model.ModelResult

	if mcpToolSet != nil {
		messages := &model.ToolMessages{
			Messages:  []*model.RawMessage{},
			ToolCalls: nil,
		}
		toolSession := &model.ToolSession{
			McpToolSet:   mcpToolSet,
			ToolMessages: messages,
		}
		modelResult, err = model.QueryTextWithTools(modelProviderObj, question, &writer, history, prompt, knowledge, toolSession, lang)
	} else {
		modelResult, err = modelProviderObj.QueryText(question, &writer, history, prompt, knowledge, nil, lang)
	}
	if err != nil {
		return "", nil, err
	}

	res := writer.String()
	res = strings.Trim(res, "\"")
	return res, modelResult, nil
}

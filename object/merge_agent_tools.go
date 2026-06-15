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
	"github.com/the-open-agent/openagent/tool"
	"github.com/the-open-agent/openagent/util"
)

func buildMergedBuiltinRegistry(store *Store, user, origin, lang string) (*tool.ToolRegistry, error) {
	reg := tool.NewToolRegistry()

	if store == nil {
		return reg, nil
	}

	if len(store.Skills) > 0 {
		if bt := tool.NewLoadSkillBuiltin(store.Owner, store.Skills, skillLoader{}); bt != nil {
			reg.RegisterTool(bt)
		}
	}

	toolNames := store.Tools
	if len(toolNames) == 1 && toolNames[0] == "All" {
		allTools, err := GetTools(store.Owner)
		if err == nil {
			toolNames = make([]string, 0, len(allTools))
			for _, t := range allTools {
				toolNames = append(toolNames, t.Name)
			}
		}
	}

	for _, tname := range toolNames {
		id := util.GetIdFromOwnerAndName(store.Owner, tname)
		t, err := GetTool(id)
		if err != nil || t == nil {
			continue
		}
		if t.LatestCapabilityStatus == string(CapabilityStatusFailed) {
			return nil, fmt.Errorf("tool '%s' failed capability check, cannot be used in agent. Please fix its configuration first", t.Name)
		}
		tp, err := tool.New(getToolConfig(t), lang)
		if err != nil {
			continue
		}
		for _, bt := range tp.BuiltinTools() {
			wrapped := wrapSnapshotBuiltin(store.Owner, bt)
			wrapped = wrapGeneratedResourceBuiltin(wrapped, store.Owner, user, origin)
			reg.RegisterTool(wrapped)
		}
	}

	return reg, nil
}

// MergeMcpTools merges builtin tools (from the store's tool list) and the
// web-search flag into an existing McpToolSet, creating one if needed.
func MergeMcpTools(mcpToolSet *mcp.ToolSet, store *Store, webSearchEnabled bool, user, origin, lang string) (*mcp.ToolSet, error) {
	if store != nil {
		validation, err := ValidateStoreTools(store)
		if err != nil {
			return nil, fmt.Errorf("failed to validate store tools: %v", err)
		}
		if validation != nil && !validation.Ok {
			var failedNames []string
			for _, f := range validation.Failed {
				failedNames = append(failedNames, fmt.Sprintf("%s (%s)", f.Name, f.Type))
			}
			return nil, fmt.Errorf("cannot build agent toolset: some tools failed capability check: %s", strings.Join(failedNames, ", "))
		}
	}

	if webSearchEnabled {
		if mcpToolSet == nil {
			mcpToolSet = &mcp.ToolSet{}
		}
		mcpToolSet.WebSearchEnabled = true
	}

	reg, err := buildMergedBuiltinRegistry(store, user, origin, lang)
	if err != nil {
		return nil, err
	}
	allTools := reg.GetToolsAsProtocolTools()
	if len(allTools) == 0 {
		return mcpToolSet, nil
	}

	if mcpToolSet == nil {
		return &mcp.ToolSet{
			Tools:        allTools,
			BuiltinTools: reg,
		}, nil
	}

	mcpToolSet.Tools = append(mcpToolSet.Tools, allTools...)
	mcpToolSet.BuiltinTools = reg
	return mcpToolSet, nil
}

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

func buildMergedBuiltinRegistry(store *Store, user, origin, lang string) (*tool.ToolRegistry, []CapabilityViolation, []CapabilityViolation) {
	reg := tool.NewToolRegistry()
	var blocked []CapabilityViolation
	var warnings []CapabilityViolation

	if store == nil {
		return reg, nil, nil
	}

	if len(store.Skills) > 0 {
		if bt := tool.NewLoadSkillBuiltin(store.Owner, store.Skills, skillLoader{}); bt != nil {
			reg.RegisterTool(bt)
		}
	}

	toolNames := store.Tools
	isAllMode := len(toolNames) == 1 && toolNames[0] == "All"
	if isAllMode {
		allTools, err := GetTools(store.Owner)
		if err == nil {
			toolNames = make([]string, 0, len(allTools))
			for _, t := range allTools {
				avail, err := GetToolCapabilityAvailability(t)
				if err != nil {
					continue
				}
				if avail.IsBlocked() {
					warnings = append(warnings, CapabilityViolation{
						Kind:       "tool",
						Name:       t.Name,
						Status:     string(avail.Status),
						Reason:     avail.BlockReason(lang),
						ConfigHash: avail.ConfigHash,
					})
					continue
				}
				toolNames = append(toolNames, t.Name)
			}
		}
	}

	for _, tname := range toolNames {
		if isAllMode && tname == "All" {
			continue
		}
		id := util.GetIdFromOwnerAndName(store.Owner, tname)
		t, err := GetTool(id)
		if err != nil || t == nil {
			continue
		}
		avail, err := GetToolCapabilityAvailability(t)
		if err != nil {
			continue
		}
		if avail.IsBlocked() {
			if isAllMode {
				warnings = append(warnings, CapabilityViolation{
					Kind:       "tool",
					Name:       t.Name,
					Status:     string(avail.Status),
					Reason:     avail.BlockReason(lang),
					ConfigHash: avail.ConfigHash,
				})
			} else {
				blocked = append(blocked, CapabilityViolation{
					Kind:       "tool",
					Name:       t.Name,
					Status:     string(avail.Status),
					Reason:     avail.BlockReason(lang),
					ConfigHash: avail.ConfigHash,
				})
			}
			continue
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

	return reg, blocked, warnings
}

// MergeMcpTools merges builtin tools (from the store's tool list) and the
// web-search flag into an existing McpToolSet, creating one if needed.
// Returns the tool set, a list of warning violations (from All-mode skips),
// and an error if explicitly-selected tools are blocked.
func MergeMcpTools(mcpToolSet *mcp.ToolSet, store *Store, webSearchEnabled bool, user, origin, lang string) (*mcp.ToolSet, []CapabilityViolation, error) {
	if webSearchEnabled {
		if mcpToolSet == nil {
			mcpToolSet = &mcp.ToolSet{}
		}
		mcpToolSet.WebSearchEnabled = true
	}

	if store == nil {
		return mcpToolSet, nil, nil
	}

	reg, blocked, warnings := buildMergedBuiltinRegistry(store, user, origin, lang)
	if len(blocked) > 0 {
		var names []string
		for _, v := range blocked {
			names = append(names, v.Name)
		}
		return nil, warnings, fmt.Errorf("some tools are unavailable: %s", strings.Join(names, ", "))
	}

	allTools := reg.GetToolsAsProtocolTools()
	if len(allTools) == 0 {
		return mcpToolSet, warnings, nil
	}

	if mcpToolSet == nil {
		return &mcp.ToolSet{
			Tools:        allTools,
			BuiltinTools: reg,
		}, warnings, nil
	}

	mcpToolSet.Tools = append(mcpToolSet.Tools, allTools...)
	mcpToolSet.BuiltinTools = reg
	return mcpToolSet, warnings, nil
}

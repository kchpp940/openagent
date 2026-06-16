package object

import (
	"strings"

	"github.com/the-open-agent/openagent/util"
)

type CapabilityState string

const (
	CapabilityStateActive    CapabilityState = "Active"
	CapabilityStateInactive  CapabilityState = "Inactive"
	CapabilityStatePending   CapabilityState = "Pending"
	CapabilityStateStale     CapabilityState = "Stale"
	CapabilityStateError     CapabilityState = "Error"
)

func (s CapabilityState) IsValid() bool {
	switch s {
	case CapabilityStateActive, CapabilityStateInactive, CapabilityStatePending, CapabilityStateStale, CapabilityStateError:
		return true
	}
	return false
}

func (s CapabilityState) CanMountToAgent() bool {
	return s == CapabilityStateActive
}

func (s CapabilityState) NeedsRecheck() bool {
	return s == CapabilityStateStale || s == CapabilityStatePending
}

type CapabilityInfo struct {
	State        CapabilityState `json:"state"`
	CanMount     bool            `json:"canMount"`
	NeedsRecheck bool            `json:"needsRecheck"`
	Reason       string          `json:"reason,omitempty"`
}

func normalizeCapabilityState(raw string) CapabilityState {
	s := CapabilityState(raw)
	if s.IsValid() {
		return s
	}
	return CapabilityStateActive
}

func GetServerCapabilityInfo(server *Server) *CapabilityInfo {
	if server == nil {
		return &CapabilityInfo{State: CapabilityStateInactive, CanMount: false, NeedsRecheck: false, Reason: "server is nil"}
	}
	state := normalizeCapabilityState(server.State)
	if server.Url == "" {
		return &CapabilityInfo{State: CapabilityStateError, CanMount: false, NeedsRecheck: false, Reason: "server URL is empty"}
	}
	if len(server.Tools) == 0 && state == CapabilityStateActive {
		return &CapabilityInfo{State: CapabilityStatePending, CanMount: false, NeedsRecheck: true, Reason: "server tools not synced yet"}
	}
	return &CapabilityInfo{State: state, CanMount: state.CanMountToAgent(), NeedsRecheck: state.NeedsRecheck()}
}

func GetSkillCapabilityInfo(skill *Skill) *CapabilityInfo {
	if skill == nil {
		return &CapabilityInfo{State: CapabilityStateInactive, CanMount: false, NeedsRecheck: false, Reason: "skill is nil"}
	}
	state := normalizeCapabilityState(skill.State)
	if strings.TrimSpace(skill.Content) == "" && state == CapabilityStateActive {
		return &CapabilityInfo{State: CapabilityStatePending, CanMount: false, NeedsRecheck: true, Reason: "skill content is empty"}
	}
	return &CapabilityInfo{State: state, CanMount: state.CanMountToAgent(), NeedsRecheck: state.NeedsRecheck()}
}

func GetToolCapabilityInfo(t *Tool) *CapabilityInfo {
	if t == nil {
		return &CapabilityInfo{State: CapabilityStateInactive, CanMount: false, NeedsRecheck: false, Reason: "tool is nil"}
	}
	state := normalizeCapabilityState(t.State)
	if strings.TrimSpace(t.Type) == "" {
		return &CapabilityInfo{State: CapabilityStateError, CanMount: false, NeedsRecheck: false, Reason: "tool type is empty"}
	}
	return &CapabilityInfo{State: state, CanMount: state.CanMountToAgent(), NeedsRecheck: state.NeedsRecheck()}
}

func CanSaveStore(store *Store) *CapabilityInfo {
	if store == nil {
		return &CapabilityInfo{State: CapabilityStateInactive, CanMount: false, NeedsRecheck: false, Reason: "store is nil"}
	}
	state := normalizeCapabilityState(store.State)
	if strings.TrimSpace(store.Name) == "" {
		return &CapabilityInfo{State: CapabilityStateError, CanMount: false, NeedsRecheck: false, Reason: "store name is empty"}
	}
	if strings.TrimSpace(store.ModelProvider) == "" {
		return &CapabilityInfo{State: CapabilityStatePending, CanMount: false, NeedsRecheck: true, Reason: "model provider is not configured"}
	}
	return &CapabilityInfo{State: state, CanMount: state.CanMountToAgent(), NeedsRecheck: state.NeedsRecheck()}
}

func ValidateStoreCapabilities(store *Store) []CapabilityInfo {
	var infos []CapabilityInfo

	if store.McpServer != "" {
		server, err := GetServerByOwnerAndName(store.Owner, store.McpServer)
		if err != nil || server == nil {
			infos = append(infos, CapabilityInfo{State: CapabilityStateError, CanMount: false, NeedsRecheck: false, Reason: "MCP server not found: " + store.McpServer})
		} else {
			info := GetServerCapabilityInfo(server)
			if !info.CanMount {
				infos = append(infos, *info)
			}
		}
	}

	skills, err := resolveEnabledSkills(store.Owner, store.Skills)
	if err == nil {
		for _, s := range skills {
			info := GetSkillCapabilityInfo(s)
			if !info.CanMount {
				infos = append(infos, *info)
			}
		}
	}

	toolNames := store.Tools
	if len(toolNames) == 1 && toolNames[0] == "All" {
		allTools, err2 := GetTools(store.Owner)
		if err2 == nil {
			toolNames = make([]string, 0, len(allTools))
			for _, t := range allTools {
				toolNames = append(toolNames, t.Name)
			}
		}
	}
	for _, tname := range toolNames {
		id := util.GetIdFromOwnerAndName(store.Owner, tname)
		t, err2 := GetTool(id)
		if err2 != nil || t == nil {
			infos = append(infos, CapabilityInfo{State: CapabilityStateError, CanMount: false, NeedsRecheck: false, Reason: "tool not found: " + tname})
			continue
		}
		info := GetToolCapabilityInfo(t)
		if !info.CanMount {
			infos = append(infos, *info)
		}
	}

	return infos
}

func GetCapabilityStateOptions() []map[string]interface{} {
	return []map[string]interface{}{
		{"value": string(CapabilityStateActive), "label": "Active"},
		{"value": string(CapabilityStateInactive), "label": "Inactive"},
	}
}

func PopulateServerCapabilityInfo(server *Server) {
	if server != nil {
		server.CapabilityInfo = GetServerCapabilityInfo(server)
	}
}

func PopulateServersCapabilityInfo(servers []*Server) {
	for _, s := range servers {
		PopulateServerCapabilityInfo(s)
	}
}

func PopulateSkillCapabilityInfo(skill *Skill) {
	if skill != nil {
		skill.CapabilityInfo = GetSkillCapabilityInfo(skill)
	}
}

func PopulateSkillsCapabilityInfo(skills []*Skill) {
	for _, s := range skills {
		PopulateSkillCapabilityInfo(s)
	}
}

func PopulateToolCapabilityInfo(t *Tool) {
	if t != nil {
		t.CapabilityInfo = GetToolCapabilityInfo(t)
	}
}

func PopulateToolsCapabilityInfo(tools []*Tool) {
	for _, t := range tools {
		PopulateToolCapabilityInfo(t)
	}
}

func PopulateStoreCapabilityInfo(store *Store) {
	if store != nil {
		store.CapabilityInfo = CanSaveStore(store)
	}
}

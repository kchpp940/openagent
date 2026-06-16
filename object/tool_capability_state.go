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

	"github.com/the-open-agent/openagent/i18n"
	"github.com/the-open-agent/openagent/util"
)

type CapabilityStatus string

const (
	CapabilityStatusPass    CapabilityStatus = "Pass"
	CapabilityStatusWarning CapabilityStatus = "Warning"
	CapabilityStatusFail    CapabilityStatus = "Fail"
	CapabilityStatusPending CapabilityStatus = "Pending"
	CapabilityStatusStale   CapabilityStatus = "Stale"
)

type CapabilityEntityType string

const (
	EntityTypeTool   CapabilityEntityType = "Tool"
	EntityTypeSkill  CapabilityEntityType = "Skill"
	EntityTypeServer CapabilityEntityType = "Server"
	EntityTypeStore  CapabilityEntityType = "Store"
)

type LegacyState string

const (
	LegacyStateActive   LegacyState = "Active"
	LegacyStateInactive LegacyState = "Inactive"
)

type CapabilityIssue struct {
	Level   CapabilityStatus `json:"level"`
	Code    string           `json:"code"`
	Message string           `json:"message"`
	Field   string           `json:"field,omitempty"`
}

type CapabilityResult struct {
	EntityType   CapabilityEntityType `json:"entityType"`
	EntityId     string               `json:"entityId"`
	EntityName   string               `json:"entityName"`
	Status       CapabilityStatus     `json:"status"`
	LegacyState  LegacyState          `json:"legacyState"`
	CanMount     bool                 `json:"canMount"`
	CanSaveStore bool                 `json:"canSaveStore"`
	NeedReview   bool                 `json:"needReview"`
	Issues       []CapabilityIssue    `json:"issues"`
	Summary      string               `json:"summary"`
}

func (s CapabilityStatus) IsAtLeast(target CapabilityStatus) bool {
	order := map[CapabilityStatus]int{
		CapabilityStatusPass:    5,
		CapabilityStatusStale:   4,
		CapabilityStatusWarning: 3,
		CapabilityStatusPending: 2,
		CapabilityStatusFail:    1,
	}
	return order[s] >= order[target]
}

func (s CapabilityStatus) DisplayText() string {
	switch s {
	case CapabilityStatusPass:
		return "通过"
	case CapabilityStatusWarning:
		return "警告"
	case CapabilityStatusFail:
		return "失败"
	case CapabilityStatusPending:
		return "待检测"
	case CapabilityStatusStale:
		return "需刷新"
	default:
		return string(s)
	}
}

func (s CapabilityStatus) TagColor() string {
	switch s {
	case CapabilityStatusPass:
		return "success"
	case CapabilityStatusWarning:
		return "warning"
	case CapabilityStatusFail:
		return "error"
	case CapabilityStatusPending:
		return "default"
	case CapabilityStatusStale:
		return "processing"
	default:
		return "default"
	}
}

func legacyStateToStatus(state LegacyState) CapabilityStatus {
	switch state {
	case LegacyStateActive:
		return CapabilityStatusPass
	case LegacyStateInactive:
		return CapabilityStatusFail
	default:
		if state == "" {
			return CapabilityStatusPending
		}
		return CapabilityStatusWarning
	}
}

func statusToLegacyState(status CapabilityStatus) LegacyState {
	switch status {
	case CapabilityStatusPass, CapabilityStatusStale, CapabilityStatusWarning:
		return LegacyStateActive
	case CapabilityStatusFail, CapabilityStatusPending:
		return LegacyStateInactive
	default:
		return LegacyStateInactive
	}
}

func (c *CapabilityResult) addIssue(level CapabilityStatus, code, message, field string) {
	c.Issues = append(c.Issues, CapabilityIssue{
		Level:   level,
		Code:    code,
		Message: message,
		Field:   field,
	})
}

func (c *CapabilityResult) finalize() {
	if len(c.Issues) == 0 {
		if c.Status == "" {
			c.Status = CapabilityStatusPass
		}
	} else {
		minStatus := CapabilityStatusPass
		for _, issue := range c.Issues {
			if !issue.Level.IsAtLeast(minStatus) {
				minStatus = issue.Level
			}
		}
		if c.Status == "" || !minStatus.IsAtLeast(c.Status) {
			c.Status = minStatus
		}
	}

	c.LegacyState = statusToLegacyState(c.Status)
	c.CanMount = c.computeCanMount()
	c.CanSaveStore = c.computeCanSaveStore()
	c.NeedReview = c.computeNeedReview()
	c.Summary = c.buildSummary()
}

func (c *CapabilityResult) computeCanMount() bool {
	return c.Status.IsAtLeast(CapabilityStatusWarning)
}

func (c *CapabilityResult) computeCanSaveStore() bool {
	return c.Status.IsAtLeast(CapabilityStatusWarning)
}

func (c *CapabilityResult) computeNeedReview() bool {
	return c.Status == CapabilityStatusStale || c.Status == CapabilityStatusWarning
}

func (c *CapabilityResult) buildSummary() string {
	if len(c.Issues) == 0 {
		switch c.EntityType {
		case EntityTypeTool:
			return "工具配置正常，可挂载到 Agent"
		case EntityTypeSkill:
			return "技能配置正常，可挂载到 Agent"
		case EntityTypeServer:
			return "MCP 服务器连接正常"
		case EntityTypeStore:
			return "Store 配置正常"
		}
		return "配置正常"
	}

	var warnings, failures int
	for _, issue := range c.Issues {
		switch issue.Level {
		case CapabilityStatusWarning:
			warnings++
		case CapabilityStatusFail:
			failures++
		}
	}

	parts := []string{}
	if failures > 0 {
		parts = append(parts, fmt.Sprintf("%d 个错误", failures))
	}
	if warnings > 0 {
		parts = append(parts, fmt.Sprintf("%d 个警告", warnings))
	}
	return strings.Join(parts, "，")
}

func CheckToolCapability(t *Tool) *CapabilityResult {
	result := &CapabilityResult{
		EntityType: EntityTypeTool,
		EntityId:   t.GetId(),
		EntityName: t.Name,
		Status:     legacyStateToStatus(LegacyState(t.State)),
	}

	if t.Name == "" {
		result.addIssue(CapabilityStatusFail, "TOOL_NAME_EMPTY", "工具名称不能为空", "name")
	}
	if t.Type == "" {
		result.addIssue(CapabilityStatusFail, "TOOL_TYPE_EMPTY", "工具类型不能为空", "type")
	}

	switch t.Type {
	case "web_search":
		if t.SubType == "Google" {
			if strings.TrimSpace(t.ClientId) == "" {
				result.addIssue(CapabilityStatusWarning, "SEARCH_ENGINE_ID_MISSING", "Google 搜索缺少搜索引擎 ID (cx)", "clientId")
			}
			if strings.TrimSpace(t.ClientSecret) == "" || t.ClientSecret == "***" {
				result.addIssue(CapabilityStatusWarning, "GOOGLE_API_KEY_MISSING", "Google 搜索缺少 API Key", "clientSecret")
			}
		}
	case "web_fetch", "web_browser", "local_file":
		if t.ProviderUrl != "" && !util.IsValidUrl(t.ProviderUrl) {
			result.addIssue(CapabilityStatusWarning, "INVALID_PROVIDER_URL", "Provider URL 格式不合法", "providerUrl")
		}
	}

	if LegacyState(t.State) == LegacyStateInactive {
		result.addIssue(CapabilityStatusFail, "TOOL_INACTIVE", "工具已被标记为未启用", "state")
	}

	result.finalize()
	return result
}

func CheckSkillCapability(s *Skill) *CapabilityResult {
	result := &CapabilityResult{
		EntityType: EntityTypeSkill,
		EntityId:   s.GetId(),
		EntityName: s.Name,
		Status:     legacyStateToStatus(LegacyState(s.State)),
	}

	if s.Name == "" {
		result.addIssue(CapabilityStatusFail, "SKILL_NAME_EMPTY", "技能名称不能为空", "name")
	}
	if strings.TrimSpace(s.Content) == "" {
		result.addIssue(CapabilityStatusWarning, "SKILL_CONTENT_EMPTY", "技能内容为空，可能无法提供有效指导", "content")
	}

	if LegacyState(s.State) == LegacyStateInactive {
		result.addIssue(CapabilityStatusFail, "SKILL_INACTIVE", "技能已被标记为未启用", "state")
	}

	result.finalize()
	return result
}

func CheckServerCapability(s *Server) *CapabilityResult {
	result := &CapabilityResult{
		EntityType: EntityTypeServer,
		EntityId:   s.GetId(),
		EntityName: s.Name,
	}

	if s.Name == "" {
		result.addIssue(CapabilityStatusFail, "SERVER_NAME_EMPTY", "服务器名称不能为空", "name")
	}
	if strings.TrimSpace(s.Url) == "" {
		result.addIssue(CapabilityStatusFail, "SERVER_URL_EMPTY", "服务器 URL 不能为空", "url")
		result.Status = CapabilityStatusFail
	} else if !util.IsValidUrl(s.Url) {
		result.addIssue(CapabilityStatusWarning, "INVALID_SERVER_URL", "服务器 URL 格式不合法", "url")
	}

	if len(s.Tools) == 0 && strings.TrimSpace(s.Url) != "" {
		result.addIssue(CapabilityStatusPending, "TOOLS_NOT_SYNCED", "尚未从服务器同步工具列表，请点击同步按钮", "tools")
		result.Status = CapabilityStatusPending
	} else {
		allowedCount := 0
		for _, t := range s.Tools {
			if t.IsAllowed {
				allowedCount++
			}
		}
		if len(s.Tools) > 0 && allowedCount == 0 {
			result.addIssue(CapabilityStatusWarning, "NO_TOOLS_ALLOWED", "所有 MCP 工具均未被允许", "tools")
		}
	}

	if result.Status == "" {
		result.Status = CapabilityStatusPass
	}

	result.finalize()
	return result
}

func CheckStoreCapability(store *Store) *CapabilityResult {
	result := &CapabilityResult{
		EntityType: EntityTypeStore,
		EntityId:   store.GetId(),
		EntityName: store.Name,
		Status:     legacyStateToStatus(LegacyState(store.State)),
	}

	if store.Name == "" {
		result.addIssue(CapabilityStatusFail, "STORE_NAME_EMPTY", "Store 名称不能为空", "name")
	}

	if LegacyState(store.State) == LegacyStateInactive {
		result.addIssue(CapabilityStatusFail, "STORE_INACTIVE", "Store 已被标记为未启用", "state")
	}

	result.finalize()
	return result
}

func CheckToolMountable(t *Tool) bool {
	return CheckToolCapability(t).CanMount
}

func CheckSkillMountable(s *Skill) bool {
	return CheckSkillCapability(s).CanMount
}

func CheckServerMountable(s *Server) bool {
	return CheckServerCapability(s).CanMount
}

func CheckStoreSavable(store *Store) bool {
	return CheckStoreCapability(store).CanSaveStore
}

func ValidateAndNormalizeToolState(t *Tool) error {
	result := CheckToolCapability(t)
	if t.State == "" {
		t.State = string(result.LegacyState)
	}
	if LegacyState(t.State) != LegacyStateActive && LegacyState(t.State) != LegacyStateInactive {
		t.State = string(result.LegacyState)
	}
	return nil
}

func ValidateAndNormalizeSkillState(s *Skill) error {
	result := CheckSkillCapability(s)
	if s.State == "" {
		s.State = string(result.LegacyState)
	}
	if LegacyState(s.State) != LegacyStateActive && LegacyState(s.State) != LegacyStateInactive {
		s.State = string(result.LegacyState)
	}
	return nil
}

func ValidateAndNormalizeServerState(s *Server) (string, error) {
	result := CheckServerCapability(s)
	return string(result.LegacyState), nil
}

func ValidateAndNormalizeStoreState(store *Store) error {
	result := CheckStoreCapability(store)
	if store.State == "" {
		store.State = string(result.LegacyState)
	}
	if LegacyState(store.State) != LegacyStateActive && LegacyState(store.State) != LegacyStateInactive {
		store.State = string(result.LegacyState)
	}
	return nil
}

func GetToolCapabilityList(tools []*Tool) []*CapabilityResult {
	results := make([]*CapabilityResult, 0, len(tools))
	for _, t := range tools {
		results = append(results, CheckToolCapability(t))
	}
	return results
}

func GetSkillCapabilityList(skills []*Skill) []*CapabilityResult {
	results := make([]*CapabilityResult, 0, len(skills))
	for _, s := range skills {
		results = append(results, CheckSkillCapability(s))
	}
	return results
}

func GetServerCapabilityList(servers []*Server) []*CapabilityResult {
	results := make([]*CapabilityResult, 0, len(servers))
	for _, s := range servers {
		results = append(results, CheckServerCapability(s))
	}
	return results
}

func FormatStateForDisplay(state string, lang string) string {
	switch LegacyState(state) {
	case LegacyStateActive:
		return i18n.Translate(lang, "general:Active")
	case LegacyStateInactive:
		return i18n.Translate(lang, "general:Inactive")
	default:
		if state == "" {
			return i18n.Translate(lang, "general:Pending")
		}
		return state
	}
}

type ToolWithCapability struct {
	*Tool
	Capability *CapabilityResult `json:"capability"`
}

type SkillWithCapability struct {
	*Skill
	Capability *CapabilityResult `json:"capability"`
}

type ServerWithCapability struct {
	*Server
	State      string            `json:"state"`
	Capability *CapabilityResult `json:"capability"`
}

func EnrichToolWithCapability(t *Tool) *ToolWithCapability {
	if t == nil {
		return nil
	}
	return &ToolWithCapability{
		Tool:       t,
		Capability: CheckToolCapability(t),
	}
}

func EnrichSkillWithCapability(s *Skill) *SkillWithCapability {
	if s == nil {
		return nil
	}
	return &SkillWithCapability{
		Skill:      s,
		Capability: CheckSkillCapability(s),
	}
}

func EnrichServerWithCapability(s *Server) *ServerWithCapability {
	if s == nil {
		return nil
	}
	cap := CheckServerCapability(s)
	return &ServerWithCapability{
		Server:     s,
		State:      string(cap.LegacyState),
		Capability: cap,
	}
}

func EnrichToolsWithCapability(tools []*Tool) []*ToolWithCapability {
	result := make([]*ToolWithCapability, 0, len(tools))
	for _, t := range tools {
		result = append(result, EnrichToolWithCapability(t))
	}
	return result
}

func EnrichSkillsWithCapability(skills []*Skill) []*SkillWithCapability {
	result := make([]*SkillWithCapability, 0, len(skills))
	for _, s := range skills {
		result = append(result, EnrichSkillWithCapability(s))
	}
	return result
}

func EnrichServersWithCapability(servers []*Server) []*ServerWithCapability {
	result := make([]*ServerWithCapability, 0, len(servers))
	for _, s := range servers {
		result = append(result, EnrichServerWithCapability(s))
	}
	return result
}

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
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/the-open-agent/openagent/util"
)

type CapabilityEntityType string

const (
	EntityTypeTool   CapabilityEntityType = "Tool"
	EntityTypeSkill  CapabilityEntityType = "Skill"
	EntityTypeServer CapabilityEntityType = "Server"
	EntityTypeStore  CapabilityEntityType = "Store"
)

type CapabilityError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Field   string `json:"field,omitempty"`
}

type FailedResource struct {
	EntityType CapabilityEntityType `json:"entityType"`
	EntityId   string               `json:"entityId"`
	Name       string               `json:"name"`
	Errors     []CapabilityError    `json:"errors"`
}

type CapabilityDecision struct {
	EntityType       CapabilityEntityType `json:"entityType"`
	EntityId         string               `json:"entityId"`
	EntityName       string               `json:"entityName"`
	ConfigHash       string               `json:"configHash"`
	LastCheckedAt    string               `json:"lastCheckedAt"`
	CanMount         bool                 `json:"canMount"`
	CanSaveStore     bool                 `json:"canSaveStore"`
	NeedsRecheck     bool                 `json:"needsRecheck"`
	BlockReason      string               `json:"blockReason,omitempty"`
	Warnings         []CapabilityError    `json:"warnings,omitempty"`
	Errors           []CapabilityError    `json:"errors,omitempty"`
	FailedResources  []FailedResource     `json:"failedResources,omitempty"`
	RecheckAction    string               `json:"recheckAction,omitempty"`
	RecheckPayload   string               `json:"recheckPayload,omitempty"`
}

func (d *CapabilityDecision) addError(code, message, field string) {
	d.Errors = append(d.Errors, CapabilityError{
		Code:    code,
		Message: message,
		Field:   field,
	})
}

func (d *CapabilityDecision) addWarning(code, message, field string) {
	d.Warnings = append(d.Warnings, CapabilityError{
		Code:    code,
		Message: message,
		Field:   field,
	})
}

func (d *CapabilityDecision) addFailedResource(entityType CapabilityEntityType, entityId, name string, errors []CapabilityError) {
	d.FailedResources = append(d.FailedResources, FailedResource{
		EntityType: entityType,
		EntityId:   entityId,
		Name:       name,
		Errors:     errors,
	})
}

func (d *CapabilityDecision) finalize() {
	if len(d.Errors) > 0 {
		d.CanMount = false
		d.CanSaveStore = false
		if d.BlockReason == "" {
			d.BlockReason = fmt.Sprintf("%d 个配置错误需要修复", len(d.Errors))
		}
	}

	if len(d.Warnings) > 0 && len(d.Errors) == 0 {
		d.NeedsRecheck = true
	}

	if d.NeedsRecheck && d.RecheckAction == "" {
		d.RecheckAction = "recheck"
	}
}

func (d *CapabilityDecision) HasErrors() bool {
	return len(d.Errors) > 0
}

func (d *CapabilityDecision) HasWarnings() bool {
	return len(d.Warnings) > 0
}

func (d *CapabilityDecision) ErrorCount() int {
	return len(d.Errors)
}

func (d *CapabilityDecision) WarningCount() int {
	return len(d.Warnings)
}

func (d *CapabilityDecision) Summary() string {
	parts := []string{}
	if len(d.Errors) > 0 {
		parts = append(parts, fmt.Sprintf("%d 个错误", len(d.Errors)))
	}
	if len(d.Warnings) > 0 {
		parts = append(parts, fmt.Sprintf("%d 个警告", len(d.Warnings)))
	}
	if len(d.FailedResources) > 0 {
		parts = append(parts, fmt.Sprintf("%d 个依赖资源不可用", len(d.FailedResources)))
	}
	if len(parts) == 0 {
		return "配置正常"
	}
	return strings.Join(parts, "，")
}

func ComputeConfigHash(v interface{}) string {
	bytes, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	hash := sha256.Sum256(bytes)
	return hex.EncodeToString(hash[:])
}

func ComputeToolConfigHash(t *Tool) string {
	config := map[string]interface{}{
		"name":         t.Name,
		"type":         t.Type,
		"subType":      t.SubType,
		"clientId":     t.ClientId,
		"providerUrl":  t.ProviderUrl,
		"enableProxy":  t.EnableProxy,
		"modelProvider": t.ModelProvider,
		"mode":         t.Mode,
	}
	return ComputeConfigHash(config)
}

func ComputeSkillConfigHash(s *Skill) string {
	config := map[string]interface{}{
		"name":        s.Name,
		"type":        s.Type,
		"description": s.Description,
		"content":     s.Content,
		"metadata":    s.Metadata,
		"references":  s.References,
	}
	return ComputeConfigHash(config)
}

func ComputeServerConfigHash(s *Server) string {
	config := map[string]interface{}{
		"name":  s.Name,
		"url":   s.Url,
		"tools": s.Tools,
	}
	return ComputeConfigHash(config)
}

func ComputeStoreConfigHash(store *Store) string {
	config := map[string]interface{}{
		"name":            store.Name,
		"tools":           store.Tools,
		"skills":          store.Skills,
		"modelProvider":   store.ModelProvider,
		"storageProvider": store.StorageProvider,
		"searchProvider":  store.SearchProvider,
		"prompt":          store.Prompt,
		"isDefault":       store.IsDefault,
	}
	return ComputeConfigHash(config)
}

func CheckToolCapability(t *Tool) *CapabilityDecision {
	decision := &CapabilityDecision{
		EntityType:    EntityTypeTool,
		EntityId:      t.GetId(),
		EntityName:    t.Name,
		ConfigHash:    ComputeToolConfigHash(t),
		LastCheckedAt: util.GetCurrentTime(),
		CanMount:      true,
		CanSaveStore:  true,
	}

	if t.Name == "" {
		decision.addError("TOOL_NAME_EMPTY", "工具名称不能为空", "name")
	}
	if t.Type == "" {
		decision.addError("TOOL_TYPE_EMPTY", "工具类型不能为空", "type")
	}

	switch t.Type {
	case "web_search":
		if t.SubType == "Google" {
			if strings.TrimSpace(t.ClientId) == "" {
				decision.addWarning("SEARCH_ENGINE_ID_MISSING", "Google 搜索缺少搜索引擎 ID (cx)，可能导致搜索结果为空", "clientId")
			}
			if strings.TrimSpace(t.ClientSecret) == "" || t.ClientSecret == "***" {
				decision.addWarning("GOOGLE_API_KEY_MISSING", "Google 搜索缺少 API Key，可能导致搜索失败", "clientSecret")
			}
		}
	case "web_fetch", "web_browser", "local_file":
		if t.ProviderUrl != "" && !util.IsValidUrl(t.ProviderUrl) {
			decision.addWarning("INVALID_PROVIDER_URL", "Provider URL 格式不合法，请检查是否包含协议头和主机名", "providerUrl")
		}
	}

	if strings.TrimSpace(t.State) != "" && strings.ToLower(t.State) != "active" && t.State != "1" {
		decision.addError("TOOL_DISABLED", "工具已被禁用", "state")
		decision.BlockReason = "工具已被禁用，请先启用"
	}

	decision.finalize()
	return decision
}

func CheckSkillCapability(s *Skill) *CapabilityDecision {
	decision := &CapabilityDecision{
		EntityType:    EntityTypeSkill,
		EntityId:      s.GetId(),
		EntityName:    s.Name,
		ConfigHash:    ComputeSkillConfigHash(s),
		LastCheckedAt: util.GetCurrentTime(),
		CanMount:      true,
		CanSaveStore:  true,
	}

	if s.Name == "" {
		decision.addError("SKILL_NAME_EMPTY", "技能名称不能为空", "name")
	}
	if strings.TrimSpace(s.Content) == "" {
		decision.addWarning("SKILL_CONTENT_EMPTY", "技能内容为空，可能无法提供有效指导", "content")
	}

	if strings.TrimSpace(s.State) != "" && strings.ToLower(s.State) != "active" && s.State != "1" {
		decision.addError("SKILL_DISABLED", "技能已被禁用", "state")
		decision.BlockReason = "技能已被禁用，请先启用"
	}

	decision.finalize()
	return decision
}

func CheckServerCapability(s *Server) *CapabilityDecision {
	decision := &CapabilityDecision{
		EntityType:    EntityTypeServer,
		EntityId:      s.GetId(),
		EntityName:    s.Name,
		ConfigHash:    ComputeServerConfigHash(s),
		LastCheckedAt: util.GetCurrentTime(),
		CanMount:      true,
		CanSaveStore:  true,
	}

	if s.Name == "" {
		decision.addError("SERVER_NAME_EMPTY", "服务器名称不能为空", "name")
	}
	if strings.TrimSpace(s.Url) == "" {
		decision.addError("SERVER_URL_EMPTY", "服务器 URL 不能为空", "url")
		decision.BlockReason = "缺少服务器 URL"
		decision.NeedsRecheck = true
		decision.RecheckAction = "sync"
	} else if !util.IsValidUrl(s.Url) {
		decision.addWarning("INVALID_SERVER_URL", "服务器 URL 格式不合法，请检查是否包含 http/https 协议", "url")
	}

	if len(s.Tools) == 0 && strings.TrimSpace(s.Url) != "" {
		decision.addError("TOOLS_NOT_SYNCED", "尚未从服务器同步工具列表，请点击同步按钮", "tools")
		decision.BlockReason = "MCP 工具未同步"
		decision.NeedsRecheck = true
		decision.RecheckAction = "sync"
	} else {
		allowedCount := 0
		for _, t := range s.Tools {
			if t.IsAllowed {
				allowedCount++
			}
		}
		if len(s.Tools) > 0 && allowedCount == 0 {
			decision.addWarning("NO_TOOLS_ALLOWED", "所有 MCP 工具均未被允许，Agent 将无法使用该服务器的任何功能", "tools")
			decision.NeedsRecheck = true
		}
	}

	decision.finalize()
	return decision
}

func CheckStoreCapability(store *Store) *CapabilityDecision {
	decision := &CapabilityDecision{
		EntityType:    EntityTypeStore,
		EntityId:      store.GetId(),
		EntityName:    store.Name,
		ConfigHash:    ComputeStoreConfigHash(store),
		LastCheckedAt: util.GetCurrentTime(),
		CanMount:      true,
		CanSaveStore:  true,
	}

	if store.Name == "" {
		decision.addError("STORE_NAME_EMPTY", "Store 名称不能为空", "name")
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
			decision.addFailedResource(EntityTypeTool, id, tname, []CapabilityError{
				{Code: "TOOL_NOT_FOUND", Message: fmt.Sprintf("工具不存在: %s", tname)},
			})
			continue
		}
		toolDecision := CheckToolCapability(t)
		if toolDecision.HasErrors() {
			decision.addFailedResource(EntityTypeTool, t.GetId(), t.Name, toolDecision.Errors)
		}
	}

	for _, sname := range store.Skills {
		id := util.GetIdFromOwnerAndName(store.Owner, sname)
		s, err := GetSkill(id)
		if err != nil || s == nil {
			decision.addFailedResource(EntityTypeSkill, id, sname, []CapabilityError{
				{Code: "SKILL_NOT_FOUND", Message: fmt.Sprintf("技能不存在: %s", sname)},
			})
			continue
		}
		skillDecision := CheckSkillCapability(s)
		if skillDecision.HasErrors() {
			decision.addFailedResource(EntityTypeSkill, s.GetId(), s.Name, skillDecision.Errors)
		}
	}

	if strings.TrimSpace(store.State) != "" && strings.ToLower(store.State) != "active" && store.State != "1" {
		decision.addError("STORE_DISABLED", "Store 已被禁用", "state")
		decision.BlockReason = "Store 已被禁用，请先启用"
	}

	if len(decision.FailedResources) > 0 {
		decision.CanSaveStore = false
		decision.BlockReason = fmt.Sprintf("%d 个依赖资源存在错误", len(decision.FailedResources))
	}

	decision.finalize()
	return decision
}

func RecheckToolCapability(t *Tool, savedHash string) (*CapabilityDecision, bool) {
	decision := CheckToolCapability(t)
	needsRecheck := savedHash != "" && decision.ConfigHash != savedHash
	decision.NeedsRecheck = decision.NeedsRecheck || needsRecheck
	if needsRecheck {
		decision.RecheckAction = "recheck"
	}
	return decision, needsRecheck
}

func RecheckSkillCapability(s *Skill, savedHash string) (*CapabilityDecision, bool) {
	decision := CheckSkillCapability(s)
	needsRecheck := savedHash != "" && decision.ConfigHash != savedHash
	decision.NeedsRecheck = decision.NeedsRecheck || needsRecheck
	if needsRecheck {
		decision.RecheckAction = "recheck"
	}
	return decision, needsRecheck
}

func RecheckServerCapability(s *Server, savedHash string) (*CapabilityDecision, bool) {
	decision := CheckServerCapability(s)
	needsRecheck := savedHash != "" && decision.ConfigHash != savedHash
	decision.NeedsRecheck = decision.NeedsRecheck || needsRecheck
	if needsRecheck {
		decision.RecheckAction = "recheck"
	}
	return decision, needsRecheck
}

func CheckToolMountable(t *Tool) (bool, *CapabilityDecision) {
	decision := CheckToolCapability(t)
	return decision.CanMount, decision
}

func CheckSkillMountable(s *Skill) (bool, *CapabilityDecision) {
	decision := CheckSkillCapability(s)
	return decision.CanMount, decision
}

func CheckServerMountable(s *Server) (bool, *CapabilityDecision) {
	decision := CheckServerCapability(s)
	return decision.CanMount, decision
}

func CheckStoreSavable(store *Store) (bool, *CapabilityDecision) {
	decision := CheckStoreCapability(store)
	return decision.CanSaveStore, decision
}

func ValidateToolBeforeSave(t *Tool) (*CapabilityDecision, error) {
	decision := CheckToolCapability(t)
	if decision.HasErrors() {
		return decision, fmt.Errorf("tool validation failed: %s", decision.Summary())
	}
	t.ConfigHash = decision.ConfigHash
	return decision, nil
}

func ValidateSkillBeforeSave(s *Skill) (*CapabilityDecision, error) {
	decision := CheckSkillCapability(s)
	if decision.HasErrors() {
		return decision, fmt.Errorf("skill validation failed: %s", decision.Summary())
	}
	s.ConfigHash = decision.ConfigHash
	return decision, nil
}

func ValidateServerBeforeSave(s *Server) (*CapabilityDecision, error) {
	decision := CheckServerCapability(s)
	s.ConfigHash = decision.ConfigHash
	return decision, nil
}

func ValidateStoreBeforeSave(store *Store) (*CapabilityDecision, error) {
	decision := CheckStoreCapability(store)
	if !decision.CanSaveStore {
		return decision, fmt.Errorf("store validation failed: %s", decision.Summary())
	}
	store.ConfigHash = decision.ConfigHash
	return decision, nil
}

func FilterMountableTools(tools []*Tool) []*Tool {
	result := make([]*Tool, 0, len(tools))
	for _, t := range tools {
		if mountable, _ := CheckToolMountable(t); mountable {
			result = append(result, t)
		}
	}
	return result
}

func FilterMountableSkills(skills []*Skill) []*Skill {
	result := make([]*Skill, 0, len(skills))
	for _, s := range skills {
		if mountable, _ := CheckSkillMountable(s); mountable {
			result = append(result, s)
		}
	}
	return result
}

type ToolWithDecision struct {
	*Tool
	Decision *CapabilityDecision `json:"decision"`
}

type SkillWithDecision struct {
	*Skill
	Decision *CapabilityDecision `json:"decision"`
}

type ServerWithDecision struct {
	*Server
	ConfigHash string             `json:"configHash"`
	State      string             `json:"state"`
	Decision   *CapabilityDecision `json:"decision"`
}

func EnrichToolWithDecision(t *Tool) *ToolWithDecision {
	if t == nil {
		return nil
	}
	return &ToolWithDecision{
		Tool:     t,
		Decision: CheckToolCapability(t),
	}
}

func EnrichSkillWithDecision(s *Skill) *SkillWithDecision {
	if s == nil {
		return nil
	}
	return &SkillWithDecision{
		Skill:    s,
		Decision: CheckSkillCapability(s),
	}
}

func EnrichServerWithDecision(s *Server) *ServerWithDecision {
	if s == nil {
		return nil
	}
	decision := CheckServerCapability(s)
	state := "Active"
	if !decision.CanMount {
		state = "Inactive"
	}
	return &ServerWithDecision{
		Server:     s,
		ConfigHash: decision.ConfigHash,
		State:      state,
		Decision:   decision,
	}
}

func EnrichToolsWithDecision(tools []*Tool) []*ToolWithDecision {
	result := make([]*ToolWithDecision, 0, len(tools))
	for _, t := range tools {
		result = append(result, EnrichToolWithDecision(t))
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Decision.CanMount != result[j].Decision.CanMount {
			return result[i].Decision.CanMount
		}
		if result[i].Decision.HasErrors() != result[j].Decision.HasErrors() {
			return !result[i].Decision.HasErrors()
		}
		return result[i].Tool.Name < result[j].Tool.Name
	})
	return result
}

func EnrichSkillsWithDecision(skills []*Skill) []*SkillWithDecision {
	result := make([]*SkillWithDecision, 0, len(skills))
	for _, s := range skills {
		result = append(result, EnrichSkillWithDecision(s))
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Decision.CanMount != result[j].Decision.CanMount {
			return result[i].Decision.CanMount
		}
		if result[i].Decision.HasErrors() != result[j].Decision.HasErrors() {
			return !result[i].Decision.HasErrors()
		}
		return result[i].Skill.Name < result[j].Skill.Name
	})
	return result
}

func EnrichServersWithDecision(servers []*Server) []*ServerWithDecision {
	result := make([]*ServerWithDecision, 0, len(servers))
	for _, s := range servers {
		result = append(result, EnrichServerWithDecision(s))
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Decision.CanMount != result[j].Decision.CanMount {
			return result[i].Decision.CanMount
		}
		if result[i].Decision.HasErrors() != result[j].Decision.HasErrors() {
			return !result[i].Decision.HasErrors()
		}
		return result[i].Server.Name < result[j].Server.Name
	})
	return result
}

type CapabilityCheckRequest struct {
	EntityType CapabilityEntityType `json:"entityType"`
	EntityId   string               `json:"entityId"`
	Force      bool                 `json:"force"`
}

type CapabilityCheckResponse struct {
	Decision *CapabilityDecision `json:"decision"`
	Changed  bool                `json:"changed"`
}

func HandleCapabilityCheck(req *CapabilityCheckRequest) (*CapabilityCheckResponse, error) {
	switch req.EntityType {
	case EntityTypeTool:
		t, err := GetTool(req.EntityId)
		if err != nil {
			return nil, err
		}
		decision, changed := RecheckToolCapability(t, t.ConfigHash)
		return &CapabilityCheckResponse{Decision: decision, Changed: changed}, nil
	case EntityTypeSkill:
		s, err := GetSkill(req.EntityId)
		if err != nil {
			return nil, err
		}
		decision, changed := RecheckSkillCapability(s, s.ConfigHash)
		return &CapabilityCheckResponse{Decision: decision, Changed: changed}, nil
	case EntityTypeServer:
		s, err := GetServer(req.EntityId)
		if err != nil {
			return nil, err
		}
		decision, changed := RecheckServerCapability(s, s.ConfigHash)
		return &CapabilityCheckResponse{Decision: decision, Changed: changed}, nil
	case EntityTypeStore:
		store, err := GetStore(req.EntityId)
		if err != nil {
			return nil, err
		}
		decision := CheckStoreCapability(store)
		return &CapabilityCheckResponse{Decision: decision, Changed: false}, nil
	default:
		return nil, fmt.Errorf("unknown entity type: %s", req.EntityType)
	}
}

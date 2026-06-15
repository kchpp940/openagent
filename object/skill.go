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
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/ThinkInAIXYZ/go-mcp/protocol"
	"github.com/the-open-agent/openagent/tool"
	"github.com/the-open-agent/openagent/util"
	"xorm.io/core"
)

// SkillReference represents a single file inside a skill's references/ directory.
type SkillReference struct {
	Name    string `json:"name"`    // filename, e.g. "get-started.md"
	Content string `json:"content"` // full file content
}

// Skill is a reusable capability definition.
//
// When loaded from a standard skill folder the fields map as follows:
//   - SkillMd     ← full raw SKILL.md text (front matter + body)
//   - Content     ← markdown body of SKILL.md (after front matter), injected into system prompt
//   - Description ← "description" field from front matter
//   - Homepage    ← "homepage" field from front matter
//   - Emoji       ← metadata.openclaw.emoji extracted from front matter
//   - Metadata    ← raw "metadata:" block text from front matter
//   - References  ← every file found in references/ directory
type Skill struct {
	Owner       string `xorm:"varchar(100) notnull pk" json:"owner"`
	Name        string `xorm:"varchar(100) notnull pk" json:"name"`
	CreatedTime string `xorm:"varchar(100)" json:"createdTime"`

	DisplayName string           `xorm:"varchar(200)" json:"displayName"`
	Type        string           `xorm:"varchar(100)" json:"type"`
	Description string           `xorm:"mediumtext" json:"description"`
	Homepage    string           `xorm:"varchar(500)" json:"homepage"`
	Emoji       string           `xorm:"varchar(50)" json:"emoji"`
	Metadata    string           `xorm:"mediumtext" json:"metadata"`
	Content     string           `xorm:"mediumtext" json:"content"`
	SkillMd     string           `xorm:"mediumtext" json:"skillMd"`
	References  []SkillReference `xorm:"mediumtext" json:"references"`

	State string `xorm:"varchar(100)" json:"state"`

	LatestCapabilityStatus string `xorm:"varchar(50)" json:"latestCapabilityStatus"`
	LatestCheckedAt        string `xorm:"varchar(100)" json:"latestCheckedAt"`
}

func (s *Skill) GetId() string {
	return fmt.Sprintf("%s/%s", s.Owner, s.Name)
}

// ---------------------------------------------------------------------------
// SKILL.md front-matter parser
// ---------------------------------------------------------------------------

// parseSkillMd parses a raw SKILL.md file and returns its structured fields.
// Front-matter format:
//
//	---
//	name: <name>
//	description: '<desc>'   # may be single/double quoted or bare
//	homepage: <url>         # optional
//	metadata:               # optional JSON5-ish block
//	  { ... }
//	---
//	<markdown body>
func parseSkillMd(raw string) (name, description, homepage, metadata, emoji, body string) {
	// Must start with "---"
	trimmed := strings.TrimLeft(raw, " \t")
	if !strings.HasPrefix(trimmed, "---") {
		body = raw
		return
	}

	// Skip the opening "---" line
	afterOpen := raw[strings.Index(raw, "---")+3:]
	newlineIdx := strings.Index(afterOpen, "\n")
	if newlineIdx >= 0 {
		afterOpen = afterOpen[newlineIdx+1:]
	}

	// Find closing "---"
	closingIdx := strings.Index(afterOpen, "\n---")
	if closingIdx < 0 {
		body = raw
		return
	}

	frontMatter := afterOpen[:closingIdx]
	body = strings.TrimSpace(afterOpen[closingIdx+4:]) // skip "\n---"

	// -----------------------------------------------------------------------
	// Parse front-matter line by line.
	// We handle:
	//   key: bare value
	//   key: 'single-quoted value'
	//   key: "double-quoted value"
	//   metadata: <multi-line block until next top-level key or EOF>
	// -----------------------------------------------------------------------
	lines := strings.Split(frontMatter, "\n")
	var metaLines []string
	inMetadata := false

	for _, line := range lines {
		// A top-level key starts at column 0 with no leading whitespace
		// and contains at least one word character before the colon.
		isTopKey := len(line) > 0 && line[0] != ' ' && line[0] != '\t' && strings.Contains(line, ":")

		if isTopKey {
			key, val, _ := strings.Cut(line, ":")
			key = strings.TrimSpace(key)
			val = strings.TrimSpace(val)

			switch key {
			case "name":
				name = unquote(val)
				inMetadata = false
			case "description":
				description = unquote(val)
				inMetadata = false
			case "homepage":
				homepage = unquote(val)
				inMetadata = false
			case "metadata":
				inMetadata = true
				metaLines = nil
				if val != "" {
					metaLines = append(metaLines, val)
				}
			default:
				inMetadata = false
			}
		} else if inMetadata {
			metaLines = append(metaLines, line)
		}
	}

	metadata = strings.Join(metaLines, "\n")

	// Extract emoji: look for  "emoji": "<value>"
	if m := regexp.MustCompile(`"emoji"\s*:\s*"([^"]+)"`).FindStringSubmatch(metadata); len(m) > 1 {
		emoji = m[1]
	}

	return
}

// unquote strips surrounding single or double quotes from a YAML string value.
func unquote(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

// ---------------------------------------------------------------------------
// LoadSkillFromPath reads a skill folder from the server filesystem and
// returns a partially-populated Skill (not yet persisted to the database).
// ---------------------------------------------------------------------------

// LoadSkill reads {dir}/SKILL.md and all {dir}/references/*.md files,
// parses them, and returns a Skill struct ready to be saved with AddSkill.
func LoadSkill(dir string) (*Skill, error) {
	skillMdPath := filepath.Join(dir, "SKILL.md")
	rawBytes, err := os.ReadFile(skillMdPath)
	if err != nil {
		return nil, fmt.Errorf("cannot read SKILL.md at %s: %w", skillMdPath, err)
	}
	raw := string(rawBytes)

	name, description, homepage, metadata, emoji, content := parseSkillMd(raw)
	if name == "" {
		// Fall back to directory base-name
		name = filepath.Base(dir)
	}

	// Read references/
	var refs []SkillReference
	refsDir := filepath.Join(dir, "references")
	if entries, err2 := os.ReadDir(refsDir); err2 == nil {
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			refPath := filepath.Join(refsDir, e.Name())
			refBytes, err3 := os.ReadFile(refPath)
			if err3 != nil {
				continue
			}
			refs = append(refs, SkillReference{
				Name:    e.Name(),
				Content: string(refBytes),
			})
		}
	}

	return &Skill{
		Name:        name,
		DisplayName: name,
		Type:        "built-in",
		Description: description,
		Homepage:    homepage,
		Emoji:       emoji,
		Metadata:    metadata,
		Content:     content,
		SkillMd:     raw,
		References:  refs,
		State:       "Active",
	}, nil
}

// ---------------------------------------------------------------------------
// Standard CRUD
// ---------------------------------------------------------------------------

func GetGlobalSkills() ([]*Skill, error) {
	skills := []*Skill{}
	err := adapter.engine.Asc("owner").Desc("created_time").Find(&skills)
	return skills, err
}

func GetSkills(owner string) ([]*Skill, error) {
	skills := []*Skill{}
	err := adapter.engine.Desc("created_time").Find(&skills, &Skill{Owner: owner})
	return skills, err
}

func getSkill(owner string, name string) (*Skill, error) {
	s := Skill{Owner: owner, Name: name}
	existed, err := adapter.engine.Get(&s)
	if err != nil {
		return &s, err
	}
	if existed {
		return &s, nil
	}
	return nil, nil
}

func GetSkill(id string) (*Skill, error) {
	owner, name, err := util.GetOwnerAndNameFromIdWithError(id)
	if err != nil {
		return nil, err
	}
	return getSkill(owner, name)
}

func GetSkillByOwnerAndName(owner string, nameOrId string) (*Skill, error) {
	if nameOrId == "" {
		return nil, nil
	}
	var id string
	if _, _, err := util.GetOwnerAndNameFromIdWithError(nameOrId); err == nil {
		id = nameOrId
	} else {
		id = util.GetIdFromOwnerAndName(owner, nameOrId)
	}
	s, err := GetSkill(id)
	if err != nil {
		return nil, err
	}
	if s != nil {
		return s, nil
	}
	if owner != "admin" && !strings.Contains(nameOrId, "/") {
		return GetSkill(util.GetIdFromOwnerAndName("admin", nameOrId))
	}
	return nil, nil
}

func GetSkillCount(owner, field, value string) (int64, error) {
	session := GetDbSession(owner, -1, -1, field, value, "", "")
	return session.Count(&Skill{})
}

func GetPaginationSkills(owner string, offset, limit int, field, value, sortField, sortOrder string) ([]*Skill, error) {
	skills := []*Skill{}
	session := GetDbSession(owner, offset, limit, field, value, sortField, sortOrder)
	err := session.Find(&skills)
	return skills, err
}

func UpdateSkill(id string, s *Skill) (bool, error) {
	owner, name, err := util.GetOwnerAndNameFromIdWithError(id)
	if err != nil {
		return false, err
	}
	skillDb, err := getSkill(owner, name)
	if err != nil {
		return false, err
	}
	if s == nil || skillDb == nil {
		return false, nil
	}

	configHash := CalculateConfigHash(s)
	s.LatestCapabilityStatus = string(CapabilityStatusPending)
	s.LatestCheckedAt = time.Now().Format(time.RFC3339)

	_, err = adapter.engine.ID(core.PK{owner, name}).AllCols().Update(s)
	if err != nil {
		return false, err
	}

	go func(skill *Skill, hash string) {
		CheckSkillCapability(skill, "en", hash)
	}(s, configHash)

	return true, nil
}

func AddSkill(s *Skill) (bool, error) {
	configHash := CalculateConfigHash(s)
	s.LatestCapabilityStatus = string(CapabilityStatusPending)
	s.LatestCheckedAt = time.Now().Format(time.RFC3339)
	affected, err := adapter.engine.Insert(s)
	if err != nil {
		return false, err
	}
	if affected > 0 {
		go func(skill *Skill, hash string) {
			CheckSkillCapability(skill, "en", hash)
		}(s, configHash)
	}
	return affected != 0, nil
}

func addSkills(skills []*Skill) (int64, error) {
	if len(skills) == 0 {
		return 0, nil
	}
	// xorm Insert accepts a slice of pointers for batch insert.
	rows := make([]interface{}, len(skills))
	for i, s := range skills {
		rows[i] = s
	}
	return adapter.engine.Insert(rows...)
}

func DeleteSkill(s *Skill) (bool, error) {
	affected, err := adapter.engine.ID(core.PK{s.Owner, s.Name}).Delete(&Skill{})
	if err != nil {
		return false, err
	}
	return affected != 0, nil
}

func resolveEnabledSkills(owner string, skillNames []string) ([]*Skill, error) {
	if len(skillNames) == 0 {
		return nil, nil
	}

	hasAll := false
	var names []string
	for _, name := range skillNames {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if name == "All" {
			hasAll = true
			continue
		}
		names = append(names, name)
	}
	if hasAll {
		return GetSkills(owner)
	}

	var skills []*Skill
	seen := map[string]bool{}
	for _, name := range names {
		s, err := GetSkillByOwnerAndName(owner, name)
		if err != nil {
			return nil, err
		}
		if s == nil {
			continue
		}

		id := s.GetId()
		if seen[id] {
			continue
		}
		seen[id] = true
		skills = append(skills, s)
	}

	return skills, nil
}

func skillNameMatches(s *Skill, skillName string) bool {
	if s == nil {
		return false
	}

	skillName = strings.TrimSpace(skillName)
	if skillName == "" {
		return false
	}

	if skillName == s.Name || skillName == s.GetId() {
		return true
	}

	owner, name, err := util.GetOwnerAndNameFromIdWithError(skillName)
	return err == nil && owner == s.Owner && name == s.Name
}

func GetSkillsCatalog(owner string, skillNames []string) (string, error) {
	if len(skillNames) == 0 {
		return "", nil
	}

	skills, err := resolveEnabledSkills(owner, skillNames)
	if err != nil {
		return "", err
	}

	var items []string
	for _, s := range skills {
		if s == nil || s.State != "Active" {
			continue
		}

		parts := []string{fmt.Sprintf("- %s", s.Name)}
		if strings.TrimSpace(s.Description) != "" {
			parts = append(parts, fmt.Sprintf("description: %s", strings.TrimSpace(s.Description)))
		}
		if strings.TrimSpace(s.Type) != "" {
			parts = append(parts, fmt.Sprintf("type: %s", strings.TrimSpace(s.Type)))
		}

		refNames := make([]string, 0, len(s.References))
		for _, ref := range s.References {
			if strings.TrimSpace(ref.Name) != "" {
				refNames = append(refNames, ref.Name)
			}
		}
		sort.Strings(refNames)
		if len(refNames) > 0 {
			parts = append(parts, fmt.Sprintf("references: %s", strings.Join(refNames, ", ")))
		}

		items = append(items, strings.Join(parts, " | "))
	}

	if len(items) == 0 {
		return "", nil
	}

	return "## Skills Usage Rules\n" +
		"- If the user explicitly mentions a skill by name, you MUST call load_skill for that skill before answering.\n" +
		"- If the user's request is clearly about a listed skill's domain, you MUST load that skill before giving procedural, policy, workflow, or step-by-step guidance.\n" +
		"- Do not answer from general memory when a relevant listed skill exists but has not been loaded.\n" +
		"- If the user asks what skills are available, answer from the catalog below instead of giving a generic summary of your broad abilities.\n\n" +
		"## Skills Catalog\n" +
		"You have access to the following skills. Do not assume all details are already loaded. If a skill looks relevant, call the load_skill tool to load its full instructions before relying on it.\n\n" +
		strings.Join(items, "\n"), nil
}

func LoadSkillPromptContent(owner string, skillName string, referenceName string) (string, error) {
	s, err := GetSkillByOwnerAndName(owner, skillName)
	if err != nil {
		return "", err
	}
	if s == nil {
		return "", fmt.Errorf("skill not found: %s", skillName)
	}
	if s.State != "Active" {
		return "", fmt.Errorf("skill is not active: %s", skillName)
	}

	buf := strings.TrimSpace(s.Content)
	if referenceName == "" {
		if len(s.References) > 0 {
			refNames := make([]string, 0, len(s.References))
			for _, ref := range s.References {
				if strings.TrimSpace(ref.Name) != "" {
					refNames = append(refNames, ref.Name)
				}
			}
			sort.Strings(refNames)
			if len(refNames) > 0 {
				buf += "\n\n## Available References\n"
				for _, name := range refNames {
					buf += "- " + name + "\n"
				}
			}
		}
		return strings.TrimSpace(buf), nil
	}

	for _, ref := range s.References {
		if ref.Name == referenceName {
			if strings.TrimSpace(ref.Content) == "" {
				return "", fmt.Errorf("reference is empty: %s", referenceName)
			}
			if buf != "" {
				buf += "\n\n"
			}
			buf += "## Reference: " + ref.Name + "\n\n" + strings.TrimSpace(ref.Content)
			return strings.TrimSpace(buf), nil
		}
	}

	return "", fmt.Errorf("reference not found: %s", referenceName)
}

type skillLoader struct{}

func (skillLoader) Load(owner string, allowedSkillNames []string, skillName string, referenceName string) (string, error) {
	if len(allowedSkillNames) > 0 {
		skills, err := resolveEnabledSkills(owner, allowedSkillNames)
		if err != nil {
			return "", err
		}

		allowed := false
		for _, s := range skills {
			if skillNameMatches(s, skillName) {
				allowed = true
				break
			}
		}
		if !allowed {
			return "", fmt.Errorf("skill is not enabled for this store: %s", skillName)
		}
	}
	return LoadSkillPromptContent(owner, skillName, referenceName)
}

func CheckSkillCapability(s *Skill, lang string, configHash ...string) *CapabilityCheckResult {
	hash := ""
	if len(configHash) > 0 {
		hash = configHash[0]
	}
	if hash == "" {
		hash = CalculateConfigHash(s)
	}

	if hash != "" {
		existing, err := GetLatestCapabilityCheckRecordByHash(s.Owner, "skill", s.Name, hash)
		if err == nil && existing != nil && existing.Status != string(CapabilityStatusPending) {
			return existing.CheckResult
		}
	}

	result := NewCapabilityCheckResult()

	result.AddCheck(checkSkillBasicConfig(s))
	result.AddCheck(checkSkillContent(s))
	result.AddCheck(checkSkillDescription(s))
	result.AddCheck(checkSkillReferences(s))
	result.AddCheck(checkSkillToolRegistration(s))
	result.AddCheck(checkSkillToolSchema(s))
	result.AddCheck(checkSkillAgentSchemaIntegration(s))
	result.AddCheck(checkSkillDryRun(s))
	result.AddCheck(checkSkillState(s))

	if s.Owner != "" && s.Name != "" {
		_, _ = SaveCapabilityCheckRecord(s.Owner, "skill", s.Name, result, hash)
		_ = UpdateSkillCapabilityStatus(s, result)
	}

	return result
}

type skillCapabilityLoader struct {
	skill *Skill
}

func (l *skillCapabilityLoader) Load(owner string, allowedSkillNames []string, skillName string, referenceName string) (string, error) {
	if l.skill == nil {
		return "", fmt.Errorf("skill is nil")
	}
	if !skillNameMatches(l.skill, skillName) {
		return "", fmt.Errorf("skill not found: %s", skillName)
	}
	return LoadSkillPromptContentFromSkill(l.skill, referenceName)
}

func LoadSkillPromptContentFromSkill(s *Skill, referenceName string) (string, error) {
	if s == nil {
		return "", fmt.Errorf("skill is nil")
	}
	if s.State != "Active" {
		return "", fmt.Errorf("skill is not active: %s", s.Name)
	}

	buf := strings.TrimSpace(s.Content)
	if referenceName == "" {
		if len(s.References) > 0 {
			refNames := make([]string, 0, len(s.References))
			for _, ref := range s.References {
				if strings.TrimSpace(ref.Name) != "" {
					refNames = append(refNames, ref.Name)
				}
			}
			sort.Strings(refNames)
			if len(refNames) > 0 {
				buf += "\n\n## Available References\n"
				for _, name := range refNames {
					buf += "- " + name + "\n"
				}
			}
		}
		return strings.TrimSpace(buf), nil
	}

	for _, ref := range s.References {
		if ref.Name == referenceName {
			if strings.TrimSpace(ref.Content) == "" {
				return "", fmt.Errorf("reference is empty: %s", referenceName)
			}
			if buf != "" {
				buf += "\n\n"
			}
			buf += "## Reference: " + referenceName + "\n\n" + strings.TrimSpace(ref.Content)
			return strings.TrimSpace(buf), nil
		}
	}

	return "", fmt.Errorf("reference not found: %s", referenceName)
}

func checkSkillToolRegistration(s *Skill) *CapabilityCheckItem {
	name := "tool_registration"
	desc := "Check if load_skill tool can be registered"

	if s.Name == "" {
		return SkippedCheck(name, desc, "Skipped: skill name is empty")
	}

	loader := &skillCapabilityLoader{skill: s}
	builtinTool := tool.NewLoadSkillBuiltin(s.Owner, []string{s.Name}, loader)
	if builtinTool == nil {
		return FailedCheck(name, desc,
			"Failed to create load_skill builtin tool",
			"Check skill configuration and dependencies",
		)
	}

	reg := tool.NewToolRegistry()
	reg.RegisterTool(builtinTool)

	tools := reg.GetToolsAsProtocolTools()
	if len(tools) == 0 {
		return FailedCheck(name, desc,
			"load_skill tool failed to register as protocol tool",
			"Tool schema may be invalid, check tool definition",
		)
	}

	return PassedCheck(name, desc, fmt.Sprintf("load_skill tool registered successfully (%d protocol tool)", len(tools)))
}

func checkSkillToolSchema(s *Skill) *CapabilityCheckItem {
	name := "tool_schema"
	desc := "Check if load_skill tool schema is valid"

	if s.Name == "" {
		return SkippedCheck(name, desc, "Skipped: skill name is empty")
	}

	loader := &skillCapabilityLoader{skill: s}
	builtinTool := tool.NewLoadSkillBuiltin(s.Owner, []string{s.Name}, loader)
	if builtinTool == nil {
		return SkippedCheck(name, desc, "Skipped: failed to create load_skill tool")
	}

	reg := tool.NewToolRegistry()
	reg.RegisterTool(builtinTool)

	tools := reg.GetToolsAsProtocolTools()
	if len(tools) == 0 {
		return SkippedCheck(name, desc, "Skipped: no tools registered")
	}

	var schemaTool *protocol.Tool
	for _, t := range tools {
		if t.Name == "load_skill" {
			schemaTool = t
			break
		}
	}

	if schemaTool == nil {
		return FailedCheck(name, desc,
			"load_skill tool not found in registered tools",
			"Check tool registration logic",
		)
	}

	if schemaTool.Description == "" {
		return WarningCheck(name, desc,
			"load_skill tool has no description",
			"Adding a description helps the model understand when to use this tool",
		)
	}

	inputSchema := schemaTool.InputSchema
	if inputSchema.Type == "" && len(inputSchema.Properties) == 0 {
		return WarningCheck(name, desc,
			"load_skill tool has empty input schema",
			"Define input schema so the model knows what parameters to pass",
		)
	}

	return PassedCheck(name, desc,
		fmt.Sprintf("load_skill tool schema is valid (type=%s, properties=%d, required=%d)",
			inputSchema.Type, len(inputSchema.Properties), len(inputSchema.Required)))
}

func checkSkillAgentSchemaIntegration(s *Skill) *CapabilityCheckItem {
	name := "agent_schema_integration"
	desc := "Check if skill integrates correctly into agent's tool schema pipeline"

	if s.Name == "" {
		return SkippedCheck(name, desc, "Skipped: skill name is empty")
	}

	mockStore := &Store{
		Owner:  s.Owner,
		Name:   "capability_check_mock",
		Skills: []string{s.Name},
	}

	reg, err := buildMergedBuiltinRegistry(mockStore, s.Owner, "capability_check", "en")
	if err != nil || reg == nil {
		return FailedCheck(name, desc,
			fmt.Sprintf("Failed to build merged builtin registry for agent: %v", err),
			"Check skill configuration and agent integration code",
		)
	}

	tools := reg.GetToolsAsProtocolTools()
	if len(tools) == 0 {
		return FailedCheck(name, desc,
			"No tools generated from agent schema pipeline",
			"Skill may not be properly integrated into agent's tool building pipeline",
		)
	}

	var loadSkillTool *protocol.Tool
	for _, t := range tools {
		if t.Name == "load_skill" {
			loadSkillTool = t
			break
		}
	}

	if loadSkillTool == nil {
		return FailedCheck(name, desc,
			"load_skill tool not found in agent's generated tool schema",
			"Skill may not be properly registered in the agent tool pipeline",
		)
	}

	return PassedCheck(name, desc,
		fmt.Sprintf("Skill integrates correctly into agent schema pipeline (%d total tools, load_skill verified)",
			len(tools)))
}

func checkSkillDryRun(s *Skill) *CapabilityCheckItem {
	name := "dry_run"
	desc := "Check if load_skill dry-run call succeeds"

	if s.Name == "" || strings.TrimSpace(s.Content) == "" {
		return SkippedCheck(name, desc, "Skipped: skill name or content is empty")
	}

	loader := &skillCapabilityLoader{skill: s}
	builtinTool := tool.NewLoadSkillBuiltin(s.Owner, []string{s.Name}, loader)
	if builtinTool == nil {
		return SkippedCheck(name, desc, "Skipped: failed to create load_skill tool")
	}

	reg := tool.NewToolRegistry()
	reg.RegisterTool(builtinTool)

	result, err := reg.ExecuteTool(context.Background(), "load_skill", map[string]interface{}{
		"skill": s.Name,
	})
	if err != nil {
		return FailedCheck(name, desc,
			fmt.Sprintf("Dry-run call to load_skill failed: %v", err),
			"Check skill content and loader implementation",
		)
	}

	if result == nil {
		return FailedCheck(name, desc,
			"Dry-run call returned nil result",
			"Check tool execution logic",
		)
	}

	if result.IsError {
		var errorText string
		for _, c := range result.Content {
			if tc, ok := c.(*protocol.TextContent); ok {
				errorText += tc.Text + " "
			}
		}
		return FailedCheck(name, desc,
			fmt.Sprintf("Dry-run call returned error: %s", strings.TrimSpace(errorText)),
			"Check skill configuration and content validity",
		)
	}

	var resultText string
	for _, c := range result.Content {
		if tc, ok := c.(*protocol.TextContent); ok {
			resultText += tc.Text + "\n"
		}
	}
	resultText = strings.TrimSpace(resultText)

	if resultText == "" {
		return WarningCheck(name, desc,
			"Dry-run call returned empty content",
			"Skill content may be empty or the loader may have an issue",
		)
	}

	truncatedResult := resultText
	if len(truncatedResult) > 100 {
		truncatedResult = truncatedResult[:100] + "..."
	}
	return PassedCheck(name, desc, fmt.Sprintf("Dry-run load_skill succeeded: %s", truncatedResult))
}

func checkSkillBasicConfig(s *Skill) *CapabilityCheckItem {
	name := "basic_config"
	desc := "Check basic skill configuration"

	if s.Name == "" {
		return FailedCheck(name, desc,
			"Skill name is empty",
			"Please provide a name for the skill",
		)
	}

	if s.DisplayName == "" {
		return WarningCheck(name, desc,
			"Skill display name is empty",
			"Adding a display name makes the skill more user-friendly",
		)
	}

	return PassedCheck(name, desc, fmt.Sprintf("Basic configuration is complete for skill: %s", s.Name))
}

func checkSkillContent(s *Skill) *CapabilityCheckItem {
	name := "skill_content"
	desc := "Check skill content validity"

	if strings.TrimSpace(s.Content) == "" {
		return FailedCheck(name, desc,
			"Skill content is empty",
			"Add instructions or documentation to the skill content",
		)
	}

	contentLen := len(strings.TrimSpace(s.Content))
	if contentLen < 50 {
		return WarningCheck(name, desc,
			fmt.Sprintf("Skill content is very short (%d chars)", contentLen),
			"Consider adding more detailed instructions for better results",
		)
	}

	return PassedCheck(name, desc, fmt.Sprintf("Skill content is present (%d chars)", contentLen))
}

func checkSkillDescription(s *Skill) *CapabilityCheckItem {
	name := "skill_description"
	desc := "Check skill description"

	if strings.TrimSpace(s.Description) == "" {
		return WarningCheck(name, desc,
			"Skill description is empty",
			"Add a brief description to help users understand what this skill does",
		)
	}

	return PassedCheck(name, desc, fmt.Sprintf("Skill description: %s", truncateString(s.Description, 60)))
}

func checkSkillReferences(s *Skill) *CapabilityCheckItem {
	name := "references"
	desc := "Check skill reference files"

	if s.References == nil || len(s.References) == 0 {
		return SkippedCheck(name, desc, "No reference files configured")
	}

	validRefs := 0
	emptyRefs := 0
	for _, ref := range s.References {
		if strings.TrimSpace(ref.Content) != "" {
			validRefs++
		} else {
			emptyRefs++
		}
	}

	if emptyRefs > 0 {
		return WarningCheck(name, desc,
			fmt.Sprintf("%d/%d reference files are empty", emptyRefs, len(s.References)),
			"Consider filling in or removing empty reference files",
		)
	}

	return PassedCheck(name, desc, fmt.Sprintf("%d reference files are valid", validRefs))
}

func checkSkillState(s *Skill) *CapabilityCheckItem {
	name := "skill_state"
	desc := "Check skill state"

	switch s.State {
	case "Active":
		return PassedCheck(name, desc, "Skill is active and ready to use")
	case "Inactive":
		return WarningCheck(name, desc,
			"Skill is inactive",
			"Set the state to 'Active' to enable this skill",
		)
	default:
		return WarningCheck(name, desc,
			fmt.Sprintf("Unknown skill state: %s", s.State),
			"Use 'Active' or 'Inactive' state",
		)
	}
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

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
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/ThinkInAIXYZ/go-mcp/protocol"
	"github.com/the-open-agent/openagent/auth"
	"github.com/the-open-agent/openagent/i18n"
	"github.com/the-open-agent/openagent/tool"
	"github.com/the-open-agent/openagent/util"
	"xorm.io/core"
)

type Tool struct {
	Owner       string `xorm:"varchar(100) notnull pk" json:"owner"`
	Name        string `xorm:"varchar(100) notnull pk" json:"name"`
	CreatedTime string `xorm:"varchar(100)" json:"createdTime"`

	DisplayName  string `xorm:"varchar(100)" json:"displayName"`
	DisplayName2 string `xorm:"varchar(100)" json:"displayName2"`
	Type         string `xorm:"varchar(100)" json:"type"`
	SubType      string `xorm:"varchar(100)" json:"subType"`
	ClientId     string `xorm:"varchar(100)" json:"clientId"`
	ClientSecret string `xorm:"varchar(2000)" json:"clientSecret"`
	ProviderUrl  string `xorm:"varchar(200)" json:"providerUrl"`
	EnableProxy  bool   `json:"enableProxy"`

	Mode           string   `xorm:"varchar(100)" json:"mode"`
	TestContent    string   `xorm:"varchar(500)" json:"testContent"`
	ModelProvider  string   `xorm:"varchar(100)" json:"modelProvider"`
	ResultSummary  string   `xorm:"varchar(500)" json:"resultSummary"`
	PromptExamples []string `xorm:"mediumtext" json:"promptExamples"`

	State string `xorm:"varchar(100)" json:"state"`

	LatestCapabilityStatus string `xorm:"varchar(50)" json:"latestCapabilityStatus"`
	LatestCheckedAt        string `xorm:"varchar(100)" json:"latestCheckedAt"`
}

func (t *Tool) GetId() string {
	return fmt.Sprintf("%s/%s", t.Owner, t.Name)
}

func GetMaskedTool(t *Tool, isMaskEnabled bool, user *auth.User) *Tool {
	if !isMaskEnabled || t == nil {
		return t
	}
	if t.ClientSecret != "" {
		t.ClientSecret = "***"
	}
	return t
}

func GetMaskedTools(tools []*Tool, isMaskEnabled bool, user *auth.User) []*Tool {
	if !isMaskEnabled {
		return tools
	}
	for _, t := range tools {
		t = GetMaskedTool(t, isMaskEnabled, user)
	}
	return tools
}

func GetGlobalTools() ([]*Tool, error) {
	tools := []*Tool{}
	err := adapter.engine.Asc("owner").Desc("created_time").Find(&tools)
	return tools, err
}

func GetTools(owner string) ([]*Tool, error) {
	tools := []*Tool{}
	err := adapter.engine.Desc("created_time").Find(&tools, &Tool{Owner: owner})
	return tools, err
}

func getTool(owner string, name string) (*Tool, error) {
	t := Tool{Owner: owner, Name: name}
	existed, err := adapter.engine.Get(&t)
	if err != nil {
		return &t, err
	}
	if existed {
		return &t, nil
	}
	return nil, nil
}

func GetTool(id string) (*Tool, error) {
	owner, name, err := util.GetOwnerAndNameFromIdWithError(id)
	if err != nil {
		return nil, err
	}
	return getTool(owner, name)
}

func GetToolByOwnerAndName(owner string, nameOrId string) (*Tool, error) {
	if nameOrId == "" {
		return nil, nil
	}
	var id string
	if _, _, err := util.GetOwnerAndNameFromIdWithError(nameOrId); err == nil {
		id = nameOrId
	} else {
		id = util.GetIdFromOwnerAndName(owner, nameOrId)
	}
	t, err := GetTool(id)
	if err != nil {
		return nil, err
	}
	if t != nil {
		return t, nil
	}
	if owner != "admin" && !strings.Contains(nameOrId, "/") {
		return GetTool(util.GetIdFromOwnerAndName("admin", nameOrId))
	}
	return nil, nil
}

func GetToolCount(owner, field, value string) (int64, error) {
	session := GetDbSession(owner, -1, -1, field, value, "", "")
	return session.Count(&Tool{})
}

func GetPaginationTools(owner string, offset, limit int, field, value, sortField, sortOrder string) ([]*Tool, error) {
	tools := []*Tool{}
	session := GetDbSession(owner, offset, limit, field, value, sortField, sortOrder)
	err := session.Find(&tools)
	return tools, err
}

func UpdateTool(id string, t *Tool) (bool, error) {
	owner, name, err := util.GetOwnerAndNameFromIdWithError(id)
	if err != nil {
		return false, err
	}
	toolDb, err := getTool(owner, name)
	if err != nil {
		return false, err
	}
	if t == nil || toolDb == nil {
		return false, nil
	}

	if t.ClientSecret == "***" {
		t.ClientSecret = toolDb.ClientSecret
	}

	t.LatestCapabilityStatus = string(CapabilityStatusPending)
	t.LatestCheckedAt = time.Now().Format(time.RFC3339)

	_, err = adapter.engine.ID(core.PK{owner, name}).AllCols().Update(t)
	if err != nil {
		return false, err
	}

	go func(tool *Tool) {
		CheckToolCapability(tool, "en")
	}(t)

	return true, nil
}

func AddTool(t *Tool) (bool, error) {
	t.LatestCapabilityStatus = string(CapabilityStatusPending)
	t.LatestCheckedAt = time.Now().Format(time.RFC3339)
	affected, err := adapter.engine.Insert(t)
	if err != nil {
		return false, err
	}
	if affected > 0 {
		go func(tool *Tool) {
			CheckToolCapability(tool, "en")
		}(t)
	}
	return affected != 0, nil
}

func DeleteTool(t *Tool) (bool, error) {
	affected, err := adapter.engine.ID(core.PK{t.Owner, t.Name}).Delete(&Tool{})
	if err != nil {
		return false, err
	}
	return affected != 0, nil
}

func getToolConfig(t *Tool) tool.Config {
	return tool.Config{
		Type:         t.Type,
		SubType:      t.SubType,
		ProviderUrl:  t.ProviderUrl,
		ClientId:     t.ClientId,
		ClientSecret: t.ClientSecret,
		EnableProxy:  t.EnableProxy,
		Mode:         t.Mode,
	}
}

func TestTool(t *Tool, lang string) (string, error) {
	return testToolWithLoader(t, lang, getTool)
}

func testToolWithLoader(t *Tool, lang string, loadTool func(owner string, name string) (*Tool, error)) (string, error) {
	if t.ClientSecret == "***" {
		if strings.TrimSpace(t.Owner) == "" || strings.TrimSpace(t.Name) == "" {
			return "", fmt.Errorf("cannot restore masked tool secret without owner and name")
		}
		toolDb, err := loadTool(t.Owner, t.Name)
		if err != nil {
			return "", err
		}
		if toolDb == nil {
			return "", fmt.Errorf("tool not found: %s/%s", t.Owner, t.Name)
		}
		t.ClientSecret = toolDb.ClientSecret
		if t.ClientSecret == "" || t.ClientSecret == "***" {
			return "", fmt.Errorf("masked clientSecret could not be restored")
		}
	}

	var payload struct {
		Tool      string                 `json:"tool"`
		Arguments map[string]interface{} `json:"arguments"`
	}
	if err := json.Unmarshal([]byte(t.TestContent), &payload); err != nil {
		return "", fmt.Errorf(i18n.Translate(lang, "object:invalid tool test JSON in testContent: %v"), err)
	}
	if strings.TrimSpace(payload.Tool) == "" {
		return "", fmt.Errorf(i18n.Translate(lang, "object:tool test JSON must include non-empty \"tool\""))
	}
	if payload.Arguments == nil {
		payload.Arguments = map[string]interface{}{}
	}

	owner := strings.TrimSpace(t.Owner)
	if owner == "" {
		return "", fmt.Errorf("tool owner is required")
	}

	tp, err := tool.New(getToolConfig(t), lang)
	if err != nil {
		return "", err
	}

	var foundTool interface {
		Execute(ctx context.Context, arguments map[string]interface{}) (*protocol.CallToolResult, error)
	}
	for _, bt := range tp.BuiltinTools() {
		if bt.GetName() == payload.Tool {
			foundTool = wrapSnapshotBuiltin(owner, bt)
			break
		}
	}
	if foundTool == nil {
		return "", fmt.Errorf("tool not found: %s", payload.Tool)
	}

	result, err := foundTool.Execute(context.Background(), payload.Arguments)
	if err != nil {
		return "", err
	}

	var texts []string
	for _, c := range result.Content {
		if tc, ok := c.(*protocol.TextContent); ok {
			texts = append(texts, tc.Text)
		}
	}
	output := strings.Join(texts, "\n")
	if result.IsError {
		return "", fmt.Errorf("%s", output)
	}
	return output, nil
}

func CheckToolCapability(t *Tool, lang string) *CapabilityCheckResult {
	result := NewCapabilityCheckResult()

	result.AddCheck(checkToolBasicConfig(t))
	result.AddCheck(checkToolTypeSupport(t))
	result.AddCheck(checkToolEnvVars(t))
	result.AddCheck(checkToolRequiredConfig(t))
	result.AddCheck(checkToolExecutables(t))
	result.AddCheck(checkToolInitialization(t, lang))
	result.AddCheck(checkToolRegistration(t, lang))
	result.AddCheck(checkToolSchemas(t, lang))
	result.AddCheck(checkToolAgentSchemaIntegration(t, lang))
	result.AddCheck(checkToolDryRun(t, lang))
	result.AddCheck(checkToolState(t))

	if t.Owner != "" && t.Name != "" {
		_ = UpdateToolCapabilityStatus(t, result)
	}

	return result
}

var envVarPattern = regexp.MustCompile(`\$\{?[A-Z_][A-Z0-9_]*\}?`)

func checkToolEnvVars(t *Tool) *CapabilityCheckItem {
	name := "env_variables"
	desc := "Check environment variable references in configuration"

	fieldsToCheck := map[string]string{
		"providerUrl":  t.ProviderUrl,
		"clientId":     t.ClientId,
		"clientSecret": t.ClientSecret,
	}

	var unresolvedVars []string
	var resolvedCount int

	for fieldName, value := range fieldsToCheck {
		if value == "" || value == "***" {
			continue
		}
		matches := envVarPattern.FindAllString(value, -1)
		for _, m := range matches {
			varName := strings.Trim(m, "${}")
			if val := getenvSafe(varName); val == "" {
				unresolvedVars = append(unresolvedVars, fmt.Sprintf("%s (in %s)", varName, fieldName))
			} else {
				resolvedCount++
			}
		}
	}

	if len(unresolvedVars) > 0 {
		return WarningCheck(name, desc,
			fmt.Sprintf("%d environment variable(s) may not be set: %s", len(unresolvedVars), strings.Join(unresolvedVars, ", ")),
			fmt.Sprintf("Set the required environment variables or check their spelling"),
			fmt.Sprintf("export %s=your_value", strings.Split(unresolvedVars[0], " ")[0]),
		)
	}

	if resolvedCount > 0 {
		return PassedCheck(name, desc, fmt.Sprintf("All %d environment variable references can be resolved", resolvedCount))
	}

	return PassedCheck(name, desc, "No environment variable references found in configuration")
}

func getenvSafe(name string) string {
	return os.Getenv(name)
}

func checkToolExecutables(t *Tool) *CapabilityCheckItem {
	name := "executables"
	desc := "Check if required external commands are available"

	requiredCmds := getToolRequiredCommands(t)
	if len(requiredCmds) == 0 {
		return SkippedCheck(name, desc, "No external commands required for this tool type")
	}

	var missingCmds []string
	var foundCmds []string

	for _, cmd := range requiredCmds {
		path, err := exec.LookPath(cmd)
		if err != nil {
			missingCmds = append(missingCmds, cmd)
		} else {
			foundCmds = append(foundCmds, fmt.Sprintf("%s (%s)", cmd, path))
		}
	}

	if len(missingCmds) > 0 {
		return FailedCheck(name, desc,
			fmt.Sprintf("Missing required command(s): %s", strings.Join(missingCmds, ", ")),
			fmt.Sprintf("Install the required commands for %s tool", t.Type),
			fmt.Sprintf("brew install %s", missingCmds[0]),
		)
	}

	return PassedCheck(name, desc, fmt.Sprintf("All required commands available: %s", strings.Join(foundCmds, ", ")))
}

func getToolRequiredCommands(t *Tool) []string {
	switch t.Type {
	case "video_download":
		return []string{"yt-dlp"}
	case "shell":
		return []string{}
	case "local_file":
		return []string{}
	default:
		return []string{}
	}
}

func checkToolRegistration(t *Tool, lang string) *CapabilityCheckItem {
	name := "tool_registration"
	desc := "Check if tools can be registered to tool registry"

	tp, err := tool.New(getToolConfig(t), lang)
	if err != nil {
		return SkippedCheck(name, desc, "Skipped: tool initialization failed")
	}

	builtinTools := tp.BuiltinTools()
	if len(builtinTools) == 0 {
		return WarningCheck(name, desc,
			"No built-in tools found for registration",
			"This tool type may not expose any callable functions",
		)
	}

	reg := tool.NewToolRegistry()
	for _, bt := range builtinTools {
		reg.RegisterTool(bt)
	}

	protocolTools := reg.GetToolsAsProtocolTools()
	if len(protocolTools) == 0 {
		return FailedCheck(name, desc,
			"Failed to convert builtin tools to protocol tools",
			"Tool schemas may be invalid, check tool implementation",
		)
	}

	if len(protocolTools) != len(builtinTools) {
		return WarningCheck(name, desc,
			fmt.Sprintf("Only %d/%d tools were successfully converted to protocol tools", len(protocolTools), len(builtinTools)),
			"Some tool schemas may be invalid",
		)
	}

	return PassedCheck(name, desc,
		fmt.Sprintf("Successfully registered %d tool(s) to tool registry", len(protocolTools)))
}

func checkToolBasicConfig(t *Tool) *CapabilityCheckItem {
	name := "basic_config"
	desc := "Check basic tool configuration"

	if t.Name == "" {
		return FailedCheck(name, desc,
			"Tool name is empty",
			"Please provide a name for the tool",
		)
	}

	if t.Type == "" {
		return FailedCheck(name, desc,
			"Tool type is empty",
			"Please select a tool type",
		)
	}

	return PassedCheck(name, desc, fmt.Sprintf("Basic configuration is complete: %s (%s)", t.Name, t.Type))
}

func checkToolTypeSupport(t *Tool) *CapabilityCheckItem {
	name := "type_support"
	desc := "Check if tool type is supported"

	supportedTypes := map[string]bool{
		"time":           true,
		"web_search":     true,
		"shell":          true,
		"local_file":     true,
		"office":         true,
		"web_fetch":      true,
		"web_browser":    true,
		"gui":            true,
		"video_download": true,
		"browser_use":    true,
	}

	if !supportedTypes[t.Type] {
		return FailedCheck(name, desc,
			fmt.Sprintf("Unsupported tool type: %s", t.Type),
			"Choose a supported tool type from the dropdown",
		)
	}

	return PassedCheck(name, desc, fmt.Sprintf("Tool type '%s' is supported", t.Type))
}

func checkToolRequiredConfig(t *Tool) *CapabilityCheckItem {
	name := "required_config"
	desc := "Check required configuration for tool type"

	missingFields := []string{}

	switch t.Type {
	case "web_search":
		if t.SubType == "Google" {
			if t.ClientId == "" {
				missingFields = append(missingFields, "Search engine ID (cx)")
			}
			if t.ClientSecret == "" || t.ClientSecret == "***" {
				missingFields = append(missingFields, "API key")
			}
		}
	case "web_fetch", "web_browser", "browser_use":
		// These tools work without extra config but may need provider URL
	case "local_file":
		// May need OCR endpoint but not strictly required
	}

	if len(missingFields) > 0 {
		return FailedCheck(name, desc,
			fmt.Sprintf("Missing required fields: %s", strings.Join(missingFields, ", ")),
			fmt.Sprintf("Fill in the required configuration fields for %s tool", t.Type),
		)
	}

	return PassedCheck(name, desc, "All required configuration is present")
}

func checkToolInitialization(t *Tool, lang string) *CapabilityCheckItem {
	name := "initialization"
	desc := "Check if tool can be initialized"

	config := getToolConfig(t)
	_, err := tool.New(config, lang)
	if err != nil {
		return FailedCheck(name, desc,
			fmt.Sprintf("Failed to initialize tool: %v", err),
			"Check tool configuration and dependencies",
		)
	}

	return PassedCheck(name, desc, "Tool initialized successfully")
}

func checkToolSchemas(t *Tool, lang string) *CapabilityCheckItem {
	name := "tool_schemas"
	desc := "Check if tool schemas are valid"

	config := getToolConfig(t)
	tp, err := tool.New(config, lang)
	if err != nil {
		return SkippedCheck(name, desc, "Skipped: tool initialization failed")
	}

	builtinTools := tp.BuiltinTools()
	if len(builtinTools) == 0 {
		return WarningCheck(name, desc,
			"No built-in tools found",
			"This tool type may not expose any callable functions",
		)
	}

	validSchemas := 0
	for _, bt := range builtinTools {
		if bt.GetInputSchema() != nil {
			validSchemas++
		}
	}

	return PassedCheck(name, desc,
		fmt.Sprintf("%d built-in tool(s) available, %d with schemas", len(builtinTools), validSchemas),
	)
}

func checkToolAgentSchemaIntegration(t *Tool, lang string) *CapabilityCheckItem {
	name := "agent_schema_integration"
	desc := "Check if tool integrates correctly into agent's tool schema pipeline"

	if t.Name == "" || t.Owner == "" {
		return SkippedCheck(name, desc, "Skipped: tool name or owner is empty")
	}

	config := getToolConfig(t)
	tp, err := tool.New(config, lang)
	if err != nil {
		return SkippedCheck(name, desc, "Skipped: tool initialization failed")
	}

	builtinTools := tp.BuiltinTools()
	if len(builtinTools) == 0 {
		return WarningCheck(name, desc,
			"No built-in tools to register in agent pipeline",
			"This tool type may not expose any callable functions",
		)
	}

	reg := tool.NewToolRegistry()
	for _, bt := range builtinTools {
		wrapped := wrapSnapshotBuiltin(t.Owner, bt)
		wrapped = wrapGeneratedResourceBuiltin(wrapped, t.Owner, t.Owner, "capability_check")
		reg.RegisterTool(wrapped)
	}

	protocolTools := reg.GetToolsAsProtocolTools()
	if len(protocolTools) == 0 {
		return FailedCheck(name, desc,
			"No protocol tools generated from agent schema pipeline",
			"Tool may not be properly integrated into agent's tool building pipeline",
		)
	}

	if len(protocolTools) != len(builtinTools) {
		return WarningCheck(name, desc,
			fmt.Sprintf("Only %d/%d tools were converted to agent protocol tools", len(protocolTools), len(builtinTools)),
			"Some tool schemas may be invalid after agent pipeline wrapping",
		)
	}

	return PassedCheck(name, desc,
		fmt.Sprintf("Tool integrates correctly into agent schema pipeline (%d tools registered)", len(protocolTools)),
	)
}

func checkToolDryRun(t *Tool, lang string) *CapabilityCheckItem {
	name := "dry_run"
	desc := "Check if a dry-run tool call succeeds"

	if strings.TrimSpace(t.TestContent) == "" {
		return WarningCheck(name, desc,
			"No test content configured",
			"Add testContent with a valid tool name and arguments to run a dry-run test",
		)
	}

	var payload struct {
		Tool      string                 `json:"tool"`
		Arguments map[string]interface{} `json:"arguments"`
	}
	if err := json.Unmarshal([]byte(t.TestContent), &payload); err != nil {
		return FailedCheck(name, desc,
			fmt.Sprintf("Invalid test content JSON: %v", err),
			"Fix the testContent JSON format: {\"tool\":\"name\",\"arguments\":{}}",
		)
	}

	if strings.TrimSpace(payload.Tool) == "" {
		return FailedCheck(name, desc,
			"Test content missing 'tool' field",
			"Add the tool name to test in the testContent JSON",
		)
	}

	result, err := TestTool(t, lang)
	if err != nil {
		return FailedCheck(name, desc,
			fmt.Sprintf("Dry-run test failed: %v", err),
			"Check tool configuration and test arguments",
		)
	}

	if result == "" {
		return WarningCheck(name, desc,
			"Dry-run test returned empty result",
			"The tool ran successfully but returned no output",
		)
	}

	truncatedResult := result
	if len(truncatedResult) > 100 {
		truncatedResult = truncatedResult[:100] + "..."
	}
	return PassedCheck(name, desc, fmt.Sprintf("Dry-run test succeeded: %s", truncatedResult))
}

func checkToolState(t *Tool) *CapabilityCheckItem {
	name := "tool_state"
	desc := "Check tool state"

	switch t.State {
	case "Active":
		return PassedCheck(name, desc, "Tool is active and ready to use")
	case "Inactive":
		return WarningCheck(name, desc,
			"Tool is inactive",
			"Set the state to 'Active' to enable this tool",
		)
	default:
		return WarningCheck(name, desc,
			fmt.Sprintf("Unknown tool state: %s", t.State),
			"Use 'Active' or 'Inactive' state",
		)
	}
}

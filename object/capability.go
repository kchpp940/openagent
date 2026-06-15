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
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/ThinkInAIXYZ/go-mcp/protocol"
	"github.com/the-open-agent/openagent/i18n"
	mcppkg "github.com/the-open-agent/openagent/mcp"
	"github.com/the-open-agent/openagent/tool"
	"github.com/the-open-agent/openagent/util"
)

type CapabilityStatus string

const (
	CapabilityStatusPass    CapabilityStatus = "pass"
	CapabilityStatusWarning CapabilityStatus = "warning"
	CapabilityStatusFail    CapabilityStatus = "fail"
	CapabilityStatusSkipped CapabilityStatus = "skipped"
	CapabilityStatusPending CapabilityStatus = "pending"
)

type CapabilityCheckItem struct {
	Id          string           `json:"id"`
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Status      CapabilityStatus `json:"status"`
	Message     string           `json:"message"`
	FixCommand  string           `json:"fixCommand,omitempty"`
	FixHint     string           `json:"fixHint,omitempty"`
	DurationMs  int64            `json:"durationMs"`
}

type CapabilityCheckResult struct {
	OverallStatus CapabilityStatus      `json:"overallStatus"`
	TotalChecks   int                   `json:"totalChecks"`
	PassedChecks  int                   `json:"passedChecks"`
	FailedChecks  int                   `json:"failedChecks"`
	WarningChecks int                   `json:"warningChecks"`
	Items         []*CapabilityCheckItem `json:"items"`
	ToolNames     []string              `json:"toolNames,omitempty"`
}

type CapabilityChecker interface {
	Check(ctx context.Context, lang string) *CapabilityCheckResult
}

func newCapabilityCheckResult() *CapabilityCheckResult {
	return &CapabilityCheckResult{
		OverallStatus: CapabilityStatusPass,
		Items:         []*CapabilityCheckItem{},
	}
}

func (r *CapabilityCheckResult) addItem(item *CapabilityCheckItem) {
	r.Items = append(r.Items, item)
	r.TotalChecks++
	switch item.Status {
	case CapabilityStatusPass:
		r.PassedChecks++
	case CapabilityStatusFail:
		r.FailedChecks++
		if r.OverallStatus != CapabilityStatusFail {
			r.OverallStatus = CapabilityStatusFail
		}
	case CapabilityStatusWarning:
		r.WarningChecks++
		if r.OverallStatus == CapabilityStatusPass {
			r.OverallStatus = CapabilityStatusWarning
		}
	}
}

func (r *CapabilityCheckResult) finalize() {
	if r.OverallStatus == CapabilityStatusPass && r.TotalChecks == 0 {
		r.OverallStatus = CapabilityStatusSkipped
	}
}

type checkFunc func() *CapabilityCheckItem

func runCheck(id, name, desc string, fn checkFunc) *CapabilityCheckItem {
	start := time.Now()
	item := fn()
	item.Id = id
	item.Name = name
	item.Description = desc
	item.DurationMs = time.Since(start).Milliseconds()
	if item.Status == "" {
		item.Status = CapabilityStatusPass
	}
	return item
}

func passItem(msg string) *CapabilityCheckItem {
	return &CapabilityCheckItem{Status: CapabilityStatusPass, Message: msg}
}

func failItem(msg, fixCmd, fixHint string) *CapabilityCheckItem {
	return &CapabilityCheckItem{
		Status:     CapabilityStatusFail,
		Message:    msg,
		FixCommand: fixCmd,
		FixHint:    fixHint,
	}
}

func warnItem(msg, fixCmd, fixHint string) *CapabilityCheckItem {
	return &CapabilityCheckItem{
		Status:     CapabilityStatusWarning,
		Message:    msg,
		FixCommand: fixCmd,
		FixHint:    fixHint,
	}
}

func skipItem(msg string) *CapabilityCheckItem {
	return &CapabilityCheckItem{Status: CapabilityStatusSkipped, Message: msg}
}

// ---------------------------------------------------------------------------
// Server capability checker
// ---------------------------------------------------------------------------

type ServerCapabilityChecker struct {
	Server *Server
}

func NewServerCapabilityChecker(s *Server) *ServerCapabilityChecker {
	return &ServerCapabilityChecker{Server: s}
}

func (c *ServerCapabilityChecker) Check(ctx context.Context, lang string) *CapabilityCheckResult {
	result := newCapabilityCheckResult()
	s := c.Server

	result.addItem(runCheck("url_format",
		i18n.Translate(lang, "capability:URL format"),
		i18n.Translate(lang, "capability:Check if the server URL is well-formed"),
		func() *CapabilityCheckItem {
			if s.Url == "" {
				return failItem(
					i18n.Translate(lang, "capability:Server URL is empty"),
					"",
					i18n.Translate(lang, "capability:Enter a valid MCP server URL (e.g. http://localhost:3000/mcp)"),
				)
			}
			u, err := url.Parse(s.Url)
			if err != nil {
				return failItem(
					fmt.Sprintf(i18n.Translate(lang, "capability:Invalid URL format: %v"), err),
					"",
					i18n.Translate(lang, "capability:URL should start with http:// or https://"),
				)
			}
			if u.Scheme != "http" && u.Scheme != "https" {
				return failItem(
					fmt.Sprintf(i18n.Translate(lang, "capability:Unsupported URL scheme: %s"), u.Scheme),
					"",
					i18n.Translate(lang, "capability:Only http and https schemes are supported"),
				)
			}
			return passItem(i18n.Translate(lang, "capability:URL format is valid"))
		},
	))

	result.addItem(runCheck("env_vars",
		i18n.Translate(lang, "capability:Environment variables"),
		i18n.Translate(lang, "capability:Check required environment variables from URL template"),
		func() *CapabilityCheckItem {
			envVars := extractEnvVars(s.Url)
			if len(envVars) == 0 {
				return passItem(i18n.Translate(lang, "capability:No environment variables required"))
			}

			var missing []string
			for _, v := range envVars {
				if os.Getenv(v) == "" {
					missing = append(missing, v)
				}
			}

			if len(missing) > 0 {
				exportCmd := "export " + strings.Join(missing, "=\"<value>\" export ")
				exportCmd = strings.TrimSuffix(exportCmd, " export ")
				return failItem(
					fmt.Sprintf(i18n.Translate(lang, "capability:Missing environment variables: %s"), strings.Join(missing, ", ")),
					exportCmd,
					i18n.Translate(lang, "capability:Set the required environment variables before starting the server"),
				)
			}
			return passItem(fmt.Sprintf(i18n.Translate(lang, "capability:All %d environment variables are set"), len(envVars)))
		},
	))

	result.addItem(runCheck("mcp_connection",
		i18n.Translate(lang, "capability:MCP connection"),
		i18n.Translate(lang, "capability:Test connection to the MCP server"),
		func() *CapabilityCheckItem {
			if s.Url == "" {
				return skipItem(i18n.Translate(lang, "capability:Skipped (no URL)"))
			}

			cli, err := mcppkg.NewClient(s.Url, s.Token)
			if err != nil {
				return failItem(
					fmt.Sprintf(i18n.Translate(lang, "capability:Failed to connect: %v"), err),
					"",
					i18n.Translate(lang, "capability:Verify the server is running and accessible"),
				)
			}
			defer cli.Close()

			pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()

			_, err = cli.Ping(pingCtx, &protocol.PingRequest{})
			if err != nil {
				return failItem(
					fmt.Sprintf(i18n.Translate(lang, "capability:Ping failed: %v"), err),
					"",
					i18n.Translate(lang, "capability:Server may not be responding correctly"),
				)
			}

			return passItem(i18n.Translate(lang, "capability:MCP connection successful"))
		},
	))

	var toolList []*protocol.Tool
	result.addItem(runCheck("tool_list",
		i18n.Translate(lang, "capability:Tool list retrieval"),
		i18n.Translate(lang, "capability:Fetch the list of tools from the server"),
		func() *CapabilityCheckItem {
			if s.Url == "" {
				return skipItem(i18n.Translate(lang, "capability:Skipped (no URL)"))
			}

			tools, err := mcppkg.GetToolsFromURL(s.Url, s.Token)
			if err != nil {
				return failItem(
					fmt.Sprintf(i18n.Translate(lang, "capability:Failed to list tools: %v"), err),
					"",
					i18n.Translate(lang, "capability:Check if the MCP server implements tools/list correctly"),
				)
			}

			toolList = tools

			if len(tools) == 0 {
				return warnItem(
					i18n.Translate(lang, "capability:Server returned 0 tools"),
					"",
					i18n.Translate(lang, "capability:Verify the server exposes at least one tool"),
				)
			}

			result.ToolNames = make([]string, len(tools))
			for i, t := range tools {
				result.ToolNames[i] = t.Name
			}

			return passItem(fmt.Sprintf(i18n.Translate(lang, "capability:Retrieved %d tools"), len(tools)))
		},
	))

	result.addItem(runCheck("tool_schema",
		i18n.Translate(lang, "capability:Tool schema validation"),
		i18n.Translate(lang, "capability:Validate each tool's inputSchema"),
		func() *CapabilityCheckItem {
			if toolList == nil {
				return skipItem(i18n.Translate(lang, "capability:Skipped (tool list not available)"))
			}

			var invalidTools []string
			for _, t := range toolList {
				schemaBytes, err := json.Marshal(t.InputSchema)
				if err != nil {
					invalidTools = append(invalidTools, fmt.Sprintf("%s (invalid JSON)", t.Name))
					continue
				}

				var schemaObj map[string]interface{}
				if err := json.Unmarshal(schemaBytes, &schemaObj); err != nil {
					invalidTools = append(invalidTools, fmt.Sprintf("%s (malformed schema)", t.Name))
					continue
				}

				if len(schemaObj) == 0 {
					invalidTools = append(invalidTools, fmt.Sprintf("%s (no schema)", t.Name))
					continue
				}

				if _, ok := schemaObj["type"]; !ok {
					invalidTools = append(invalidTools, fmt.Sprintf("%s (missing type)", t.Name))
				}
			}

			if len(invalidTools) > 0 {
				return warnItem(
					fmt.Sprintf(i18n.Translate(lang, "capability:Tools with schema issues: %s"), strings.Join(invalidTools, ", ")),
					"",
					i18n.Translate(lang, "capability:Each tool should have a valid JSON Schema with a type field"),
				)
			}

			return passItem(fmt.Sprintf(i18n.Translate(lang, "capability:All %d tool schemas are valid"), len(toolList)))
		},
	))

	result.addItem(runCheck("dry_run",
		i18n.Translate(lang, "capability:Dry-run call"),
		i18n.Translate(lang, "capability:Attempt a safe tool call with default arguments"),
		func() *CapabilityCheckItem {
			if toolList == nil || len(toolList) == 0 {
				return skipItem(i18n.Translate(lang, "capability:Skipped (no tools available)"))
			}

			if s.TestContent == "" {
				return skipItem(i18n.Translate(lang, "capability:Skipped (no test content configured)"))
			}

			testResult, err := TestMcpServer(s, lang)
			if err != nil {
				return failItem(
					fmt.Sprintf(i18n.Translate(lang, "capability:Dry-run failed: %v"), err),
					"",
					i18n.Translate(lang, "capability:Check testContent JSON format and tool arguments"),
				)
			}

			return passItem(fmt.Sprintf(i18n.Translate(lang, "capability:Dry-run succeeded: %s"), truncate(testResult, 100)))
		},
	))

	result.finalize()
	return result
}

// ---------------------------------------------------------------------------
// Skill capability checker
// ---------------------------------------------------------------------------

type SkillCapabilityChecker struct {
	Skill *Skill
}

func NewSkillCapabilityChecker(s *Skill) *SkillCapabilityChecker {
	return &SkillCapabilityChecker{Skill: s}
}

func (c *SkillCapabilityChecker) Check(ctx context.Context, lang string) *CapabilityCheckResult {
	result := newCapabilityCheckResult()
	s := c.Skill

	result.addItem(runCheck("name_valid",
		i18n.Translate(lang, "capability:Skill name"),
		i18n.Translate(lang, "capability:Check if the skill name is valid"),
		func() *CapabilityCheckItem {
			if s.Name == "" {
				return failItem(
					i18n.Translate(lang, "capability:Skill name is empty"),
					"",
					i18n.Translate(lang, "capability:Enter a unique skill name"),
				)
			}
			if !regexp.MustCompile(`^[a-zA-Z0-9_-]+$`).MatchString(s.Name) {
				return failItem(
					i18n.Translate(lang, "capability:Skill name contains invalid characters"),
					"",
					i18n.Translate(lang, "capability:Use only letters, numbers, underscores and hyphens"),
				)
			}
			return passItem(fmt.Sprintf(i18n.Translate(lang, "capability:Skill name is valid: %s"), s.Name))
		},
	))

	result.addItem(runCheck("content_present",
		i18n.Translate(lang, "capability:Skill content"),
		i18n.Translate(lang, "capability:Check if the skill has content"),
		func() *CapabilityCheckItem {
			content := strings.TrimSpace(s.Content)
			if content == "" {
				return failItem(
					i18n.Translate(lang, "capability:Skill content is empty"),
					"",
					i18n.Translate(lang, "capability:Add instructions or documentation to the skill content"),
				)
			}
			if len(content) < 20 {
				return warnItem(
					i18n.Translate(lang, "capability:Skill content is very short"),
					"",
					i18n.Translate(lang, "capability:Consider adding more detailed instructions"),
				)
			}
			return passItem(fmt.Sprintf(i18n.Translate(lang, "capability:Content has %d characters"), len(content)))
		},
	))

	result.addItem(runCheck("state_active",
		i18n.Translate(lang, "capability:Skill state"),
		i18n.Translate(lang, "capability:Check if the skill is active"),
		func() *CapabilityCheckItem {
			if s.State == "" {
				return warnItem(
					i18n.Translate(lang, "capability:Skill state is not set"),
					"",
					i18n.Translate(lang, "capability:Set state to Active to enable the skill"),
				)
			}
			if s.State != "Active" {
				return warnItem(
					fmt.Sprintf(i18n.Translate(lang, "capability:Skill state is: %s"), s.State),
					"",
					i18n.Translate(lang, "capability:Only Active skills can be used by agents"),
				)
			}
			return passItem(i18n.Translate(lang, "capability:Skill is active"))
		},
	))

	result.addItem(runCheck("references_valid",
		i18n.Translate(lang, "capability:References"),
		i18n.Translate(lang, "capability:Validate reference files"),
		func() *CapabilityCheckItem {
			if len(s.References) == 0 {
				return skipItem(i18n.Translate(lang, "capability:No references configured"))
			}

			var emptyRefs []string
			for _, ref := range s.References {
				if strings.TrimSpace(ref.Content) == "" {
					emptyRefs = append(emptyRefs, ref.Name)
				}
			}

			if len(emptyRefs) > 0 {
				return warnItem(
					fmt.Sprintf(i18n.Translate(lang, "capability:Empty references: %s"), strings.Join(emptyRefs, ", ")),
					"",
					i18n.Translate(lang, "capability:Add content to reference files or remove them"),
				)
			}

			return passItem(fmt.Sprintf(i18n.Translate(lang, "capability:All %d references are valid"), len(s.References)))
		},
	))

	result.addItem(runCheck("metadata_parse",
		i18n.Translate(lang, "capability:Metadata parsing"),
		i18n.Translate(lang, "capability:Check if SKILL.md front matter parses correctly"),
		func() *CapabilityCheckItem {
			if s.SkillMd == "" {
				return skipItem(i18n.Translate(lang, "capability:No SKILL.md content available"))
			}

			name, description, _, _, _, _ := parseSkillMd(s.SkillMd)
			if name == "" {
				return warnItem(
					i18n.Translate(lang, "capability:Could not parse name from SKILL.md"),
					"",
					i18n.Translate(lang, "capability:Ensure SKILL.md has valid YAML front matter with 'name' field"),
				)
			}
			if description == "" {
				return warnItem(
					i18n.Translate(lang, "capability:No description in SKILL.md"),
					"",
					i18n.Translate(lang, "capability:Add a description field to help users understand the skill"),
				)
			}

			return passItem(i18n.Translate(lang, "capability:SKILL.md metadata parsed successfully"))
		},
	))

	result.finalize()
	return result
}

// ---------------------------------------------------------------------------
// Tool capability checker
// ---------------------------------------------------------------------------

type ToolCapabilityChecker struct {
	Tool *Tool
}

func NewToolCapabilityChecker(t *Tool) *ToolCapabilityChecker {
	return &ToolCapabilityChecker{Tool: t}
}

func (c *ToolCapabilityChecker) Check(ctx context.Context, lang string) *CapabilityCheckResult {
	result := newCapabilityCheckResult()
	t := c.Tool

	result.addItem(runCheck("type_valid",
		i18n.Translate(lang, "capability:Tool type"),
		i18n.Translate(lang, "capability:Check if the tool type is supported"),
		func() *CapabilityCheckItem {
			validTypes := map[string]bool{
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
			if t.Type == "" {
				return failItem(
					i18n.Translate(lang, "capability:Tool type is empty"),
					"",
					i18n.Translate(lang, "capability:Select a tool type from the dropdown"),
				)
			}
			if !validTypes[t.Type] {
				return warnItem(
					fmt.Sprintf(i18n.Translate(lang, "capability:Unknown tool type: %s"), t.Type),
					"",
					i18n.Translate(lang, "capability:Ensure the tool type is supported by the system"),
				)
			}
			return passItem(fmt.Sprintf(i18n.Translate(lang, "capability:Tool type is valid: %s"), t.Type))
		},
	))

	result.addItem(runCheck("state_active",
		i18n.Translate(lang, "capability:Tool state"),
		i18n.Translate(lang, "capability:Check if the tool is active"),
		func() *CapabilityCheckItem {
			if t.State == "" {
				return warnItem(
					i18n.Translate(lang, "capability:Tool state is not set"),
					"",
					i18n.Translate(lang, "capability:Set state to Active to enable the tool"),
				)
			}
			if t.State != "Active" {
				return warnItem(
					fmt.Sprintf(i18n.Translate(lang, "capability:Tool state is: %s"), t.State),
					"",
					i18n.Translate(lang, "capability:Only Active tools can be used by agents"),
				)
			}
			return passItem(i18n.Translate(lang, "capability:Tool is active"))
		},
	))

	result.addItem(runCheck("required_config",
		i18n.Translate(lang, "capability:Required configuration"),
		i18n.Translate(lang, "capability:Check required fields for the selected tool type"),
		func() *CapabilityCheckItem {
			var missing []string

			if t.Type == "web_search" && t.SubType == "Google" {
				if t.ClientId == "" {
					missing = append(missing, "Search engine ID (cx)")
				}
				if t.ClientSecret == "" || t.ClientSecret == "***" {
					missing = append(missing, "API key")
				}
			}

			if t.Type == "web_search" && t.SubType == "Baidu" {
				if t.ClientSecret == "" || t.ClientSecret == "***" {
					missing = append(missing, "API key")
				}
			}

			if t.Type == "browser_use" && t.Mode == "" {
				missing = append(missing, "Chrome mode")
			}

			if len(missing) > 0 {
				return failItem(
					fmt.Sprintf(i18n.Translate(lang, "capability:Missing required fields: %s"), strings.Join(missing, ", ")),
					"",
					i18n.Translate(lang, "capability:Fill in all required configuration fields"),
				)
			}

			return passItem(i18n.Translate(lang, "capability:All required fields are present"))
		},
	))

	result.addItem(runCheck("command_exec",
		i18n.Translate(lang, "capability:Command availability"),
		i18n.Translate(lang, "capability:Check if required external commands are available"),
		func() *CapabilityCheckItem {
			var commands []string

			if t.Type == "shell" {
				if _, err := exec.LookPath("bash"); err != nil {
					commands = append(commands, "bash")
				}
			}

			if t.Type == "video_download" {
				if _, err := exec.LookPath("yt-dlp"); err != nil {
					commands = append(commands, "yt-dlp")
				}
			}

			if t.Type == "office" {
				if _, err := exec.LookPath("libreoffice"); err != nil {
					if _, err := exec.LookPath("soffice"); err != nil {
						commands = append(commands, "libreoffice/soffice")
					}
				}
			}

			if len(commands) > 0 {
				var installCmd string
				for _, cmd := range commands {
					switch cmd {
					case "yt-dlp":
						installCmd += "pip install yt-dlp\n"
					case "libreoffice/soffice":
						installCmd += "# macOS: brew install libreoffice\n# Ubuntu: sudo apt install libreoffice\n"
					}
				}
				return failItem(
					fmt.Sprintf(i18n.Translate(lang, "capability:Required commands not found: %s"), strings.Join(commands, ", ")),
					strings.TrimSpace(installCmd),
					i18n.Translate(lang, "capability:Install the required command-line tools"),
				)
			}

			return passItem(i18n.Translate(lang, "capability:All required commands are available"))
		},
	))

	result.addItem(runCheck("functions_valid",
		i18n.Translate(lang, "capability:Tool functions"),
		i18n.Translate(lang, "capability:Check if the tool exposes functions"),
		func() *CapabilityCheckItem {
			tp, err := tool.New(getToolConfig(t), lang)
			if err != nil {
				return failItem(
					fmt.Sprintf(i18n.Translate(lang, "capability:Failed to create tool provider: %v"), err),
					"",
					i18n.Translate(lang, "capability:Check tool configuration"),
				)
			}

			tools := tp.BuiltinTools()
			if len(tools) == 0 {
				return warnItem(
					i18n.Translate(lang, "capability:Tool exposes no functions"),
					"",
					i18n.Translate(lang, "capability:The tool may not be usable"),
				)
			}

			toolNames := make([]string, len(tools))
			for i, bt := range tools {
				toolNames[i] = bt.GetName()
			}
			result.ToolNames = toolNames

			return passItem(fmt.Sprintf(i18n.Translate(lang, "capability:Tool exposes %d functions: %s"), len(tools), strings.Join(toolNames, ", ")))
		},
	))

	result.addItem(runCheck("dry_run",
		i18n.Translate(lang, "capability:Dry-run call"),
		i18n.Translate(lang, "capability:Attempt a tool call with test content"),
		func() *CapabilityCheckItem {
			if t.TestContent == "" {
				return skipItem(i18n.Translate(lang, "capability:Skipped (no test content configured)"))
			}

			testResult, err := TestTool(t, lang)
			if err != nil {
				return failItem(
					fmt.Sprintf(i18n.Translate(lang, "capability:Dry-run failed: %v"), err),
					"",
					i18n.Translate(lang, "capability:Check testContent JSON format and tool arguments"),
				)
			}

			return passItem(fmt.Sprintf(i18n.Translate(lang, "capability:Dry-run succeeded: %s"), truncate(testResult, 100)))
		},
	))

	result.finalize()
	return result
}

func extractEnvVars(s string) []string {
	re := regexp.MustCompile(`\$\{?([A-Z_][A-Z0-9_]*)\}?`)
	matches := re.FindAllStringSubmatch(s, -1)
	seen := map[string]bool{}
	var vars []string
	for _, m := range matches {
		if !seen[m[1]] {
			seen[m[1]] = true
			vars = append(vars, m[1])
		}
	}
	return vars
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// ---------------------------------------------------------------------------
// Persistence helpers – write the latest capability check result back to the
// corresponding entity row.
// ---------------------------------------------------------------------------

// SaveServerCapabilityStatus writes the latestCapabilityStatus and
// latestCheckedAt fields for the given server.
func SaveServerCapabilityStatus(owner, name string, status CapabilityStatus) error {
	if owner == "" || name == "" {
		return nil
	}
	now := util.GetCurrentTime()
	_, err := adapter.engine.Where("owner = ? AND name = ?", owner, name).
		Cols("latest_capability_status", "latest_checked_at").
		Update(&Server{
			LatestCapabilityStatus: string(status),
			LatestCheckedAt:        now,
		})
	return err
}

// SaveSkillCapabilityStatus writes the latestCapabilityStatus and
// latestCheckedAt fields for the given skill.
func SaveSkillCapabilityStatus(owner, name string, status CapabilityStatus) error {
	if owner == "" || name == "" {
		return nil
	}
	now := util.GetCurrentTime()
	_, err := adapter.engine.Where("owner = ? AND name = ?", owner, name).
		Cols("latest_capability_status", "latest_checked_at").
		Update(&Skill{
			LatestCapabilityStatus: string(status),
			LatestCheckedAt:        now,
		})
	return err
}

// SaveToolCapabilityStatus writes the latestCapabilityStatus and
// latestCheckedAt fields for the given tool.
func SaveToolCapabilityStatus(owner, name string, status CapabilityStatus) error {
	if owner == "" || name == "" {
		return nil
	}
	now := util.GetCurrentTime()
	_, err := adapter.engine.Where("owner = ? AND name = ?", owner, name).
		Cols("latest_capability_status", "latest_checked_at").
		Update(&Tool{
			LatestCapabilityStatus: string(status),
			LatestCheckedAt:        now,
		})
	return err
}

// AsyncTriggerServerCapabilityCheck runs a capability check in a background
// goroutine and persists the full result. Safe to fire-and-forget.
func AsyncTriggerServerCapabilityCheck(s *Server, lang string) {
	if s == nil || s.Owner == "" || s.Name == "" {
		return
	}
	configHash := ComputeServerConfigHash(s)
	owner := s.Owner
	name := s.Name
	srv := *s // copy

	go func() {
		defer func() {
			_ = recover()
		}()
		_ = SetCapabilityPending(owner, name, "server", configHash)
		_ = SaveServerCapabilityStatus(owner, name, CapabilityStatusPending)

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		result := NewServerCapabilityChecker(&srv).Check(ctx, lang)
		if result != nil {
			_ = PersistCapabilityCheckResult(owner, name, "server", configHash, result)
			_ = SaveServerCapabilityStatus(owner, name, result.OverallStatus)
		}
	}()
}

// AsyncTriggerSkillCapabilityCheck runs a capability check in a background
// goroutine and persists the full result. Safe to fire-and-forget.
func AsyncTriggerSkillCapabilityCheck(s *Skill, lang string) {
	if s == nil || s.Owner == "" || s.Name == "" {
		return
	}
	configHash := ComputeSkillConfigHash(s)
	owner := s.Owner
	name := s.Name
	sk := *s // copy

	go func() {
		defer func() {
			_ = recover()
		}()
		_ = SetCapabilityPending(owner, name, "skill", configHash)
		_ = SaveSkillCapabilityStatus(owner, name, CapabilityStatusPending)

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		result := NewSkillCapabilityChecker(&sk).Check(ctx, lang)
		if result != nil {
			_ = PersistCapabilityCheckResult(owner, name, "skill", configHash, result)
			_ = SaveSkillCapabilityStatus(owner, name, result.OverallStatus)
		}
	}()
}

// AsyncTriggerToolCapabilityCheck runs a capability check in a background
// goroutine and persists the full result. Safe to fire-and-forget.
func AsyncTriggerToolCapabilityCheck(t *Tool, lang string) {
	if t == nil || t.Owner == "" || t.Name == "" {
		return
	}
	configHash := ComputeToolConfigHash(t)
	owner := t.Owner
	name := t.Name
	tl := *t // copy

	go func() {
		defer func() {
			_ = recover()
		}()
		_ = SetCapabilityPending(owner, name, "tool", configHash)
		_ = SaveToolCapabilityStatus(owner, name, CapabilityStatusPending)

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		result := NewToolCapabilityChecker(&tl).Check(ctx, lang)
		if result != nil {
			_ = PersistCapabilityCheckResult(owner, name, "tool", configHash, result)
			_ = SaveToolCapabilityStatus(owner, name, result.OverallStatus)
		}
	}()
}

// RunServerCapabilityCheck runs a capability check synchronously, persists the
// full result, and returns the updated availability info.
func RunServerCapabilityCheck(s *Server, lang string) (*CapabilityAvailability, error) {
	if s == nil || s.Owner == "" || s.Name == "" {
		return nil, fmt.Errorf("invalid server")
	}
	configHash := ComputeServerConfigHash(s)
	owner := s.Owner
	name := s.Name

	_ = SetCapabilityPending(owner, name, "server", configHash)
	_ = SaveServerCapabilityStatus(owner, name, CapabilityStatusPending)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	result := NewServerCapabilityChecker(s).Check(ctx, lang)
	if result == nil {
		return nil, fmt.Errorf("check returned nil result")
	}

	if err := PersistCapabilityCheckResult(owner, name, "server", configHash, result); err != nil {
		return nil, err
	}
	if err := SaveServerCapabilityStatus(owner, name, result.OverallStatus); err != nil {
		return nil, err
	}

	// Refresh server entity with updated status
	updated, err := GetServerByOwnerAndName(owner, name)
	if err != nil || updated == nil {
		return nil, fmt.Errorf("failed to refresh server after check")
	}
	return GetServerCapabilityAvailability(updated)
}

// RunSkillCapabilityCheck runs a capability check synchronously, persists the
// full result, and returns the updated availability info.
func RunSkillCapabilityCheck(s *Skill, lang string) (*CapabilityAvailability, error) {
	if s == nil || s.Owner == "" || s.Name == "" {
		return nil, fmt.Errorf("invalid skill")
	}
	configHash := ComputeSkillConfigHash(s)
	owner := s.Owner
	name := s.Name

	_ = SetCapabilityPending(owner, name, "skill", configHash)
	_ = SaveSkillCapabilityStatus(owner, name, CapabilityStatusPending)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	result := NewSkillCapabilityChecker(s).Check(ctx, lang)
	if result == nil {
		return nil, fmt.Errorf("check returned nil result")
	}

	if err := PersistCapabilityCheckResult(owner, name, "skill", configHash, result); err != nil {
		return nil, err
	}
	if err := SaveSkillCapabilityStatus(owner, name, result.OverallStatus); err != nil {
		return nil, err
	}

	updated, err := GetSkillByOwnerAndName(owner, name)
	if err != nil || updated == nil {
		return nil, fmt.Errorf("failed to refresh skill after check")
	}
	return GetSkillCapabilityAvailability(updated)
}

// RunToolCapabilityCheck runs a capability check synchronously, persists the
// full result, and returns the updated availability info.
func RunToolCapabilityCheck(t *Tool, lang string) (*CapabilityAvailability, error) {
	if t == nil || t.Owner == "" || t.Name == "" {
		return nil, fmt.Errorf("invalid tool")
	}
	configHash := ComputeToolConfigHash(t)
	owner := t.Owner
	name := t.Name

	_ = SetCapabilityPending(owner, name, "tool", configHash)
	_ = SaveToolCapabilityStatus(owner, name, CapabilityStatusPending)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	result := NewToolCapabilityChecker(t).Check(ctx, lang)
	if result == nil {
		return nil, fmt.Errorf("check returned nil result")
	}

	if err := PersistCapabilityCheckResult(owner, name, "tool", configHash, result); err != nil {
		return nil, err
	}
	if err := SaveToolCapabilityStatus(owner, name, result.OverallStatus); err != nil {
		return nil, err
	}

	updated, err := GetToolByOwnerAndName(owner, name)
	if err != nil || updated == nil {
		return nil, fmt.Errorf("failed to refresh tool after check")
	}
	return GetToolCapabilityAvailability(updated)
}

// IsCapabilityFailed returns true if the stored status equals "fail".
// Empty / unknown status is treated as not-failed (lenient).
func IsCapabilityFailed(status string) bool {
	return status == string(CapabilityStatusFail)
}

// IsCapabilityWarning returns true if the stored status equals "warning".
func IsCapabilityWarning(status string) bool {
	return status == string(CapabilityStatusWarning)
}

// ---------------------------------------------------------------------------
// Store validation – check that the MCP server, skills and tools referenced
// by a Store all have acceptable capability status.
//
// Rules:
//   - "fail"    → blocked, caller must abort the operation
//   - "warning" → allowed, but the caller should warn the user
//   - "pass" / "" / unknown → allowed without restriction
// ---------------------------------------------------------------------------

type CapabilityViolation struct {
	Kind       string `json:"kind"`       // "server" | "skill" | "tool"
	Name       string `json:"name"`       // entity name
	Status     string `json:"status"`     // "pass" | "warning" | "fail" | "pending" | "never_checked" | "config_stale"
	Reason     string `json:"reason"`     // human-readable reason
	ConfigHash string `json:"configHash"` // last checked config hash
}

// ValidateStoreCapabilities loads every server/skill/tool referenced by the
// store and partitions them into blocked (failed/pending/never_checked/
// config_stale) and warning lists.
func ValidateStoreCapabilities(store *Store, lang string) (blocked []CapabilityViolation, warnings []CapabilityViolation, err error) {
	if store == nil {
		return nil, nil, nil
	}
	owner := strings.TrimSpace(store.Owner)
	if owner == "" {
		owner = "admin"
	}

	// --- MCP server ---------------------------------------------------------
	if strings.TrimSpace(store.McpServer) != "" {
		srv, e := GetServerByOwnerAndName(owner, store.McpServer)
		if e != nil {
			return nil, nil, e
		}
		if srv != nil {
			avail, e := GetServerCapabilityAvailability(srv)
			if e != nil {
				return nil, nil, e
			}
			if avail.IsBlocked() {
				status := string(avail.Status)
				if !avail.HasRecord {
					status = "never_checked"
				} else if avail.ConfigStale {
					status = "config_stale"
				}
				blocked = append(blocked, CapabilityViolation{
					Kind:       "server",
					Name:       srv.Name,
					Status:     status,
					Reason:     avail.BlockReason(lang),
					ConfigHash: avail.ConfigHash,
				})
			} else if avail.Status == CapabilityStatusWarning {
				warnings = append(warnings, CapabilityViolation{
					Kind:       "server",
					Name:       srv.Name,
					Status:     string(avail.Status),
					Reason:     i18n.Translate(lang, "capability:Some warnings detected"),
					ConfigHash: avail.ConfigHash,
				})
			}
		}
	}

	// --- Skills -------------------------------------------------------------
	skillNames := store.Skills
	if len(skillNames) == 1 && skillNames[0] == "All" {
		allSkills, e := GetSkills(owner)
		if e != nil {
			return nil, nil, e
		}
		for _, s := range allSkills {
			if s == nil || s.State != "Active" {
				continue
			}
			avail, e := GetSkillCapabilityAvailability(s)
			if e != nil {
				return nil, nil, e
			}
			if avail.IsBlocked() {
				status := string(avail.Status)
				if !avail.HasRecord {
					status = "never_checked"
				} else if avail.ConfigStale {
					status = "config_stale"
				}
				blocked = append(blocked, CapabilityViolation{
					Kind:       "skill",
					Name:       s.Name,
					Status:     status,
					Reason:     avail.BlockReason(lang),
					ConfigHash: avail.ConfigHash,
				})
			} else if avail.Status == CapabilityStatusWarning {
				warnings = append(warnings, CapabilityViolation{
					Kind:       "skill",
					Name:       s.Name,
					Status:     string(avail.Status),
					Reason:     i18n.Translate(lang, "capability:Some warnings detected"),
					ConfigHash: avail.ConfigHash,
				})
			}
		}
	} else {
		for _, name := range skillNames {
			name = strings.TrimSpace(name)
			if name == "" || name == "All" {
				continue
			}
			s, e := GetSkillByOwnerAndName(owner, name)
			if e != nil {
				return nil, nil, e
			}
			if s == nil {
				continue
			}
			avail, e := GetSkillCapabilityAvailability(s)
			if e != nil {
				return nil, nil, e
			}
			if avail.IsBlocked() {
				status := string(avail.Status)
				if !avail.HasRecord {
					status = "never_checked"
				} else if avail.ConfigStale {
					status = "config_stale"
				}
				blocked = append(blocked, CapabilityViolation{
					Kind:       "skill",
					Name:       s.Name,
					Status:     status,
					Reason:     avail.BlockReason(lang),
					ConfigHash: avail.ConfigHash,
				})
			} else if avail.Status == CapabilityStatusWarning {
				warnings = append(warnings, CapabilityViolation{
					Kind:       "skill",
					Name:       s.Name,
					Status:     string(avail.Status),
					Reason:     i18n.Translate(lang, "capability:Some warnings detected"),
					ConfigHash: avail.ConfigHash,
				})
			}
		}
	}

	// --- Tools --------------------------------------------------------------
	toolNames := store.Tools
	if len(toolNames) == 1 && toolNames[0] == "All" {
		allTools, e := GetTools(owner)
		if e != nil {
			return nil, nil, e
		}
		for _, t := range allTools {
			if t == nil || t.State != "Active" {
				continue
			}
			avail, e := GetToolCapabilityAvailability(t)
			if e != nil {
				return nil, nil, e
			}
			if avail.IsBlocked() {
				status := string(avail.Status)
				if !avail.HasRecord {
					status = "never_checked"
				} else if avail.ConfigStale {
					status = "config_stale"
				}
				blocked = append(blocked, CapabilityViolation{
					Kind:       "tool",
					Name:       t.Name,
					Status:     status,
					Reason:     avail.BlockReason(lang),
					ConfigHash: avail.ConfigHash,
				})
			} else if avail.Status == CapabilityStatusWarning {
				warnings = append(warnings, CapabilityViolation{
					Kind:       "tool",
					Name:       t.Name,
					Status:     string(avail.Status),
					Reason:     i18n.Translate(lang, "capability:Some warnings detected"),
					ConfigHash: avail.ConfigHash,
				})
			}
		}
	} else {
		for _, name := range toolNames {
			name = strings.TrimSpace(name)
			if name == "" || name == "All" {
				continue
			}
			t, e := GetToolByOwnerAndName(owner, name)
			if e != nil {
				return nil, nil, e
			}
			if t == nil {
				continue
			}
			avail, e := GetToolCapabilityAvailability(t)
			if e != nil {
				return nil, nil, e
			}
			if avail.IsBlocked() {
				status := string(avail.Status)
				if !avail.HasRecord {
					status = "never_checked"
				} else if avail.ConfigStale {
					status = "config_stale"
				}
				blocked = append(blocked, CapabilityViolation{
					Kind:       "tool",
					Name:       t.Name,
					Status:     status,
					Reason:     avail.BlockReason(lang),
					ConfigHash: avail.ConfigHash,
				})
			} else if avail.Status == CapabilityStatusWarning {
				warnings = append(warnings, CapabilityViolation{
					Kind:       "tool",
					Name:       t.Name,
					Status:     string(avail.Status),
					Reason:     i18n.Translate(lang, "capability:Some warnings detected"),
					ConfigHash: avail.ConfigHash,
				})
			}
		}
	}

	return blocked, warnings, nil
}

// ---------------------------------------------------------------------------
// CapabilityCheckRecord – persisted full check result
// ---------------------------------------------------------------------------

type CapabilityCheckRecord struct {
	Owner       string                `xorm:"varchar(100) notnull pk" json:"owner"`
	Name        string                `xorm:"varchar(100) notnull pk" json:"name"`
	Kind        string                `xorm:"varchar(50) notnull pk" json:"kind"` // "server" | "skill" | "tool"
	ConfigHash  string                `xorm:"varchar(64)" json:"configHash"`
	Status      string                `xorm:"varchar(50)" json:"status"` // "pending" | "pass" | "warning" | "fail"
	Items       []*CapabilityCheckItem `xorm:"mediumtext" json:"items"`
	DryRunOutput string               `xorm:"mediumtext" json:"dryRunOutput,omitempty"`
	ToolNames   []string              `xorm:"mediumtext" json:"toolNames,omitempty"`
	ErrorMessage string               `xorm:"varchar(1000)" json:"errorMessage,omitempty"`
	CheckedAt   string                `xorm:"varchar(100)" json:"checkedAt"`
	CreatedAt   string                `xorm:"varchar(100)" json:"createdAt"`
}

func (r *CapabilityCheckRecord) TableName() string {
	return "capability_check_record"
}

// ---------------------------------------------------------------------------
// Config hash helpers – produce a stable hash of the entity's effective config
// ---------------------------------------------------------------------------

// ComputeServerConfigHash returns a short hash of the server fields that
// materially affect capability checks.
func ComputeServerConfigHash(s *Server) string {
	if s == nil {
		return ""
	}
	fields := map[string]interface{}{
		"url":         s.Url,
		"token":       s.Token,
		"testContent": s.TestContent,
		"isDefault":   s.IsDefault,
	}
	return hashFields(fields)
}

// ComputeSkillConfigHash returns a short hash of the skill fields that
// materially affect capability checks.
func ComputeSkillConfigHash(s *Skill) string {
	if s == nil {
		return ""
	}
	refStr := ""
	if s.References != nil {
		if b, err := json.Marshal(s.References); err == nil {
			refStr = string(b)
		}
	}
	fields := map[string]interface{}{
		"content":    s.Content,
		"references": refStr,
		"state":      s.State,
	}
	return hashFields(fields)
}

// ComputeToolConfigHash returns a short hash of the tool fields that
// materially affect capability checks.
func ComputeToolConfigHash(t *Tool) string {
	if t == nil {
		return ""
	}
	fields := map[string]interface{}{
		"type":         t.Type,
		"subType":      t.SubType,
		"state":        t.State,
		"clientId":     t.ClientId,
		"clientSecret": t.ClientSecret,
		"providerUrl":  t.ProviderUrl,
		"mode":         t.Mode,
		"testContent":  t.TestContent,
		"enableProxy":  t.EnableProxy,
	}
	return hashFields(fields)
}

func hashFields(fields map[string]interface{}) string {
	keys := make([]string, 0, len(fields))
	for k := range fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var sb strings.Builder
	for _, k := range keys {
		sb.WriteString(k)
		sb.WriteString("=")
		switch v := fields[k].(type) {
		case string:
			sb.WriteString(v)
		case bool:
			sb.WriteString(fmt.Sprintf("%t", v))
		default:
			sb.WriteString(fmt.Sprintf("%v", v))
		}
		sb.WriteString(";")
	}
	sum := md5.Sum([]byte(sb.String()))
	return hex.EncodeToString(sum[:])
}

// ---------------------------------------------------------------------------
// CapabilityCheckRecord CRUD
// ---------------------------------------------------------------------------

func GetCapabilityCheckRecord(owner, name, kind string) (*CapabilityCheckRecord, error) {
	record := CapabilityCheckRecord{Owner: owner, Name: name, Kind: kind}
	existed, err := adapter.engine.Get(&record)
	if err != nil {
		return nil, err
	}
	if !existed {
		return nil, nil
	}
	return &record, nil
}

func AddCapabilityCheckRecord(record *CapabilityCheckRecord) error {
	record.CreatedAt = util.GetCurrentTime()
	if record.CheckedAt == "" {
		record.CheckedAt = record.CreatedAt
	}
	_, err := adapter.engine.Insert(record)
	return err
}

func UpdateCapabilityCheckRecord(record *CapabilityCheckRecord) error {
	record.CheckedAt = util.GetCurrentTime()
	_, err := adapter.engine.Where("owner = ? AND name = ? AND kind = ?",
		record.Owner, record.Name, record.Kind).AllCols().Update(record)
	return err
}

func UpsertCapabilityCheckRecord(record *CapabilityCheckRecord) error {
	existing, err := GetCapabilityCheckRecord(record.Owner, record.Name, record.Kind)
	if err != nil {
		return err
	}
	if existing == nil {
		return AddCapabilityCheckRecord(record)
	}
	record.CreatedAt = existing.CreatedAt
	return UpdateCapabilityCheckRecord(record)
}

// ---------------------------------------------------------------------------
// Pending-state helpers
// ---------------------------------------------------------------------------

// SetCapabilityPending writes (or upserts) a pending record for the entity.
// This is called *before* the asynchronous check starts so callers know a
// check is in flight.
func SetCapabilityPending(owner, name, kind, configHash string) error {
	now := util.GetCurrentTime()
	record := &CapabilityCheckRecord{
		Owner:      owner,
		Name:       name,
		Kind:       kind,
		ConfigHash: configHash,
		Status:     string(CapabilityStatusPending),
		Items:      []*CapabilityCheckItem{},
		CheckedAt:  now,
	}
	return UpsertCapabilityCheckRecord(record)
}

// PersistCapabilityCheckResult writes the full check result back to the
// database, replacing the pending record.
func PersistCapabilityCheckResult(owner, name, kind, configHash string, result *CapabilityCheckResult) error {
	if result == nil {
		return nil
	}
	now := util.GetCurrentTime()
	dryRunOutput := ""
	for _, item := range result.Items {
		if strings.Contains(item.Id, "dry_run") && item.Message != "" {
			dryRunOutput = item.Message
			break
		}
	}
	record := &CapabilityCheckRecord{
		Owner:        owner,
		Name:         name,
		Kind:         kind,
		ConfigHash:   configHash,
		Status:       string(result.OverallStatus),
		Items:        result.Items,
		DryRunOutput: dryRunOutput,
		ToolNames:    result.ToolNames,
		CheckedAt:    now,
	}
	return UpsertCapabilityCheckRecord(record)
}

// ---------------------------------------------------------------------------
// Entity capability status query helpers
// ---------------------------------------------------------------------------

// CapabilityAvailability summarises the availability of a single entity,
// suitable for blocking / allowing decisions.
type CapabilityAvailability struct {
	Status       CapabilityStatus `json:"status"`       // pending | pass | warning | fail | "" (never checked)
	ConfigHash   string           `json:"configHash"`   // last checked config hash
	CheckedAt    string           `json:"checkedAt"`    // last check time
	HasRecord    bool             `json:"hasRecord"`    // whether a record exists at all
	ConfigStale  bool             `json:"configStale"`  // record's hash != current config
	Record       *CapabilityCheckRecord `json:"record,omitempty"` // full record (optional)
}

// IsBlocked returns true when the entity must not be used:
//   - never checked
//   - check in progress (pending)
//   - failed
//   - config has changed since last check
func (a *CapabilityAvailability) IsBlocked() bool {
	if !a.HasRecord {
		return true
	}
	if a.Status == CapabilityStatusFail || a.Status == CapabilityStatusPending || a.Status == "" {
		return true
	}
	if a.ConfigStale {
		return true
	}
	return false
}

// BlockReason returns a human-readable reason why the entity is blocked, or
// empty string if it is not blocked.
func (a *CapabilityAvailability) BlockReason(lang string) string {
	if !a.HasRecord {
		return i18n.Translate(lang, "capability:Never checked – run capability check first")
	}
	switch a.Status {
	case CapabilityStatusPending:
		return i18n.Translate(lang, "capability:Capability check in progress – please wait")
	case CapabilityStatusFail:
		return i18n.Translate(lang, "capability:Capability check failed – fix issues first")
	}
	if a.ConfigStale {
		return i18n.Translate(lang, "capability:Config has changed – re-check availability")
	}
	return ""
}

// GetServerCapabilityAvailability loads the persisted check record (if any)
// and compares its config hash to the current server.
func GetServerCapabilityAvailability(s *Server) (*CapabilityAvailability, error) {
	if s == nil {
		return &CapabilityAvailability{}, nil
	}
	record, err := GetCapabilityCheckRecord(s.Owner, s.Name, "server")
	if err != nil {
		return nil, err
	}
	if record == nil {
		return &CapabilityAvailability{HasRecord: false}, nil
	}
	currentHash := ComputeServerConfigHash(s)
	avail := &CapabilityAvailability{
		Status:      CapabilityStatus(record.Status),
		ConfigHash:  record.ConfigHash,
		CheckedAt:   record.CheckedAt,
		HasRecord:   true,
		ConfigStale: record.ConfigHash != currentHash,
		Record:      record,
	}
	return avail, nil
}

// GetSkillCapabilityAvailability loads the persisted check record (if any)
// and compares its config hash to the current skill.
func GetSkillCapabilityAvailability(s *Skill) (*CapabilityAvailability, error) {
	if s == nil {
		return &CapabilityAvailability{}, nil
	}
	record, err := GetCapabilityCheckRecord(s.Owner, s.Name, "skill")
	if err != nil {
		return nil, err
	}
	if record == nil {
		return &CapabilityAvailability{HasRecord: false}, nil
	}
	currentHash := ComputeSkillConfigHash(s)
	avail := &CapabilityAvailability{
		Status:      CapabilityStatus(record.Status),
		ConfigHash:  record.ConfigHash,
		CheckedAt:   record.CheckedAt,
		HasRecord:   true,
		ConfigStale: record.ConfigHash != currentHash,
		Record:      record,
	}
	return avail, nil
}

// GetToolCapabilityAvailability loads the persisted check record (if any)
// and compares its config hash to the current tool.
func GetToolCapabilityAvailability(t *Tool) (*CapabilityAvailability, error) {
	if t == nil {
		return &CapabilityAvailability{}, nil
	}
	record, err := GetCapabilityCheckRecord(t.Owner, t.Name, "tool")
	if err != nil {
		return nil, err
	}
	if record == nil {
		return &CapabilityAvailability{HasRecord: false}, nil
	}
	currentHash := ComputeToolConfigHash(t)
	avail := &CapabilityAvailability{
		Status:      CapabilityStatus(record.Status),
		ConfigHash:  record.ConfigHash,
		CheckedAt:   record.CheckedAt,
		HasRecord:   true,
		ConfigStale: record.ConfigHash != currentHash,
		Record:      record,
	}
	return avail, nil
}

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
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/ThinkInAIXYZ/go-mcp/protocol"
	"github.com/the-open-agent/openagent/i18n"
	mcppkg "github.com/the-open-agent/openagent/mcp"
	"github.com/the-open-agent/openagent/tool"
)

type CapabilityStatus string

const (
	CapabilityStatusPass    CapabilityStatus = "pass"
	CapabilityStatusWarning CapabilityStatus = "warning"
	CapabilityStatusFail    CapabilityStatus = "fail"
	CapabilityStatusSkipped CapabilityStatus = "skipped"
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

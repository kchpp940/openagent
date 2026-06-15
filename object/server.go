// Copyright 2025 The OpenAgent Authors. All Rights Reserved.
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
	"strings"
	"time"

	"github.com/ThinkInAIXYZ/go-mcp/client"
	"github.com/ThinkInAIXYZ/go-mcp/protocol"
	"github.com/the-open-agent/openagent/i18n"
	"github.com/the-open-agent/openagent/mcp"
	mcppkg "github.com/the-open-agent/openagent/mcp"
	"github.com/the-open-agent/openagent/util"
	"xorm.io/core"
)

type McpTool struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	IsAllowed   bool   `json:"isAllowed"`
	InputSchema string `json:"inputSchema,omitempty"`
}

type Server struct {
	Owner       string `xorm:"varchar(100) notnull pk" json:"owner"`
	Name        string `xorm:"varchar(100) notnull pk" json:"name"`
	CreatedTime string `xorm:"varchar(100)" json:"createdTime"`
	UpdatedTime string `xorm:"varchar(100)" json:"updatedTime"`
	DisplayName string `xorm:"varchar(100)" json:"displayName"`

	Url         string     `xorm:"varchar(500)" json:"url"`
	Token       string     `xorm:"varchar(500)" json:"-"`
	Tools       []*McpTool `xorm:"mediumtext" json:"tools"`
	TestContent string     `xorm:"varchar(500)" json:"testContent"`
	IsDefault   bool       `json:"isDefault"`
}

func (s *Server) GetId() string {
	return fmt.Sprintf("%s/%s", s.Owner, s.Name)
}

func GetServers(owner string) ([]*Server, error) {
	servers := []*Server{}
	err := adapter.engine.Desc("created_time").Find(&servers, &Server{Owner: owner})
	if err != nil {
		return servers, err
	}
	return servers, nil
}

func getServer(owner, name string) (*Server, error) {
	server := Server{Owner: owner, Name: name}
	existed, err := adapter.engine.Get(&server)
	if err != nil {
		return &server, err
	}
	if existed {
		return &server, nil
	}
	return nil, nil
}

func GetServer(id string) (*Server, error) {
	owner, name, err := util.GetOwnerAndNameFromIdWithError(id)
	if err != nil {
		return nil, err
	}
	return getServer(owner, name)
}

func GetServerByOwnerAndName(owner, nameOrId string) (*Server, error) {
	if nameOrId == "" {
		return nil, nil
	}
	var id string
	if _, _, err := util.GetOwnerAndNameFromIdWithError(nameOrId); err == nil {
		id = nameOrId
	} else {
		id = util.GetIdFromOwnerAndName(owner, nameOrId)
	}
	s, err := GetServer(id)
	if err != nil {
		return nil, err
	}
	if s != nil {
		return s, nil
	}
	if owner != "admin" && !strings.Contains(nameOrId, "/") {
		return GetServer(util.GetIdFromOwnerAndName("admin", nameOrId))
	}
	return nil, nil
}

func AddServer(server *Server) (bool, error) {
	affected, err := adapter.engine.Insert(server)
	if err != nil {
		return false, err
	}
	return affected != 0, nil
}

func UpdateServer(id string, server *Server) (bool, error) {
	owner, name, err := util.GetOwnerAndNameFromIdWithError(id)
	if err != nil {
		return false, err
	}

	oldServer, err := getServer(owner, name)
	if err != nil {
		return false, err
	}
	if oldServer != nil && server.Token == "" {
		server.Token = oldServer.Token
	}

	_, err = adapter.engine.ID(core.PK{owner, name}).AllCols().Update(server)
	if err != nil {
		return false, err
	}
	return true, nil
}

func SyncMcpTool(id string, server *Server, isCleared bool) (bool, error) {
	owner, name, err := util.GetOwnerAndNameFromIdWithError(id)
	if err != nil {
		return false, err
	}

	if isCleared {
		server.Tools = nil
		_, err = adapter.engine.ID(core.PK{owner, name}).Cols("tools").Update(server)
		if err != nil {
			return false, err
		}
		return true, nil
	}

	oldServer, err := getServer(owner, name)
	if err != nil {
		return false, err
	}
	if oldServer == nil {
		return false, nil
	}
	if server.Token == "" {
		server.Token = oldServer.Token
	}

	if err = syncServerTools(server); err != nil {
		return false, err
	}

	_, err = adapter.engine.ID(core.PK{owner, name}).AllCols().Update(server)
	if err != nil {
		return false, err
	}
	return true, nil
}

func syncServerTools(server *Server) error {
	if server.Url == "" {
		return fmt.Errorf("server URL is empty")
	}

	oldTools := server.Tools
	if oldTools == nil {
		oldTools = []*McpTool{}
	}

	tools, err := mcppkg.GetToolsFromURL(server.Url, server.Token)
	if err != nil {
		return err
	}

	newTools := make([]*McpTool, 0, len(tools))
	for _, t := range tools {
		isAllowed := true
		for _, old := range oldTools {
			if old.Name == t.Name {
				isAllowed = old.IsAllowed
				break
			}
		}
		schemaJSON, _ := json.Marshal(t.InputSchema)
		newTools = append(newTools, &McpTool{
			Name:        t.Name,
			Description: t.Description,
			IsAllowed:   isAllowed,
			InputSchema: string(schemaJSON),
		})
	}

	server.Tools = newTools
	return nil
}

func DeleteServer(server *Server) (bool, error) {
	affected, err := adapter.engine.ID(core.PK{server.Owner, server.Name}).Delete(&Server{})
	if err != nil {
		return false, err
	}
	return affected != 0, nil
}

// BuildMcpToolSet opens a connection to the server's URL and returns an
// McpToolSet with the allowed tools and the open connection.
// The caller must close all connections in McpToolSet.Connections when done.
func (s *Server) BuildMcpToolSet() (*mcp.ToolSet, error) {
	if s.Url == "" {
		return nil, nil
	}

	cli, err := mcp.NewClient(s.Url, s.Token)
	if err != nil {
		return nil, err
	}

	// Determine which tools are allowed. If Tools is empty (not yet synced),
	// allow everything; otherwise only include tools with IsAllowed = true.
	allowedSet := make(map[string]bool)
	hasFilter := len(s.Tools) > 0
	for _, t := range s.Tools {
		allowedSet[t.Name] = t.IsAllowed
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	list, err := cli.ListTools(ctx)
	if err != nil {
		cli.Close()
		return nil, err
	}

	var filteredTools []*protocol.Tool
	for _, t := range list.Tools {
		if hasFilter {
			if allowed, ok := allowedSet[t.Name]; !ok || !allowed {
				continue
			}
		}
		tCopy := *t
		toolId, err := mcp.GetIdFromServerNameAndToolName(s.Name, t.Name)
		if err != nil {
			cli.Close()
			return nil, fmt.Errorf("failed to construct tool id for server %s tool %s: %w", s.Name, t.Name, err)
		}
		tCopy.Name = toolId
		filteredTools = append(filteredTools, &tCopy)
	}

	return &mcp.ToolSet{
		Connections: map[string]*client.Client{s.Name: cli},
		Tools:       filteredTools,
	}, nil
}

// GetServerMcpToolSet loads the named MCP server and returns its tool set.
func GetServerMcpToolSet(owner, serverName, lang string) (*mcp.ToolSet, error) {
	if serverName == "" {
		return nil, nil
	}
	server, err := GetServerByOwnerAndName(owner, serverName)
	if err != nil {
		return nil, err
	}
	if server == nil {
		return nil, fmt.Errorf(i18n.Translate(lang, "object:The MCP server: %s is not found"), serverName)
	}
	return server.BuildMcpToolSet()
}

// TestMcpServer connects to the server URL and calls the tool specified in
// TestContent (JSON: {"tool": "toolName", "arguments": {...}}).
func TestMcpServer(s *Server, lang string) (string, error) {
	if s.Url == "" {
		return "", fmt.Errorf(i18n.Translate(lang, "object:Server URL is empty"))
	}
	var payload struct {
		Tool      string                 `json:"tool"`
		Arguments map[string]interface{} `json:"arguments"`
	}
	if err := json.Unmarshal([]byte(s.TestContent), &payload); err != nil {
		return "", fmt.Errorf(i18n.Translate(lang, "object:invalid MCP test JSON: %v"), err)
	}
	if strings.TrimSpace(payload.Tool) == "" {
		return "", fmt.Errorf(i18n.Translate(lang, "object:MCP test JSON must include non-empty \"tool\""))
	}
	if payload.Arguments == nil {
		payload.Arguments = map[string]interface{}{}
	}
	return mcp.CallTool(s.Url, s.Token, payload.Tool, payload.Arguments)
}

func GetServerCount(owner, field, value string) (int64, error) {
	session := GetDbSession(owner, -1, -1, field, value, "", "")
	count, err := session.Count(&Server{})
	if err != nil {
		return 0, err
	}
	return count, nil
}

func GetPaginationServers(owner string, offset, limit int, field, value, sortField, sortOrder string) ([]*Server, error) {
	servers := []*Server{}
	session := GetDbSession(owner, offset, limit, field, value, sortField, sortOrder)
	err := session.Find(&servers)
	if err != nil {
		return servers, err
	}
	return servers, nil
}

func CheckServerCapability(s *Server) *CapabilityCheckResult {
	result := NewCapabilityCheckResult()

	result.AddCheck(checkServerUrl(s))
	result.AddCheck(checkMcpConnection(s))
	result.AddCheck(checkMcpToolList(s))
	result.AddCheck(checkMcpToolSchema(s))
	result.AddCheck(checkMcpDryRun(s))

	return result
}

func checkServerUrl(s *Server) *CapabilityCheckItem {
	name := "server_url"
	desc := "Check if server URL is configured"

	if s.Url == "" {
		return FailedCheck(name, desc,
			"Server URL is empty",
			"Please enter the MCP server URL in the configuration",
		)
	}

	if !strings.HasPrefix(s.Url, "http://") && !strings.HasPrefix(s.Url, "https://") {
		return FailedCheck(name, desc,
			"Server URL must start with http:// or https://",
			"Add the correct protocol prefix to the URL",
		)
	}

	return PassedCheck(name, desc, fmt.Sprintf("Server URL is configured: %s", s.Url))
}

func checkMcpConnection(s *Server) *CapabilityCheckItem {
	name := "mcp_connection"
	desc := "Check MCP server connection"

	if s.Url == "" {
		return SkippedCheck(name, desc, "Skipped: server URL is empty")
	}

	cli, err := mcp.NewClient(s.Url, s.Token)
	if err != nil {
		return FailedCheck(name, desc,
			fmt.Sprintf("Failed to connect to MCP server: %v", err),
			"Verify the server URL is correct and the server is running",
		)
	}
	defer cli.Close()

	return PassedCheck(name, desc, "Successfully connected to MCP server")
}

func checkMcpToolList(s *Server) *CapabilityCheckItem {
	name := "tool_list"
	desc := "Check if tool list can be fetched"

	if s.Url == "" {
		return SkippedCheck(name, desc, "Skipped: server URL is empty")
	}

	tools, err := mcp.GetToolsFromURL(s.Url, s.Token)
	if err != nil {
		return FailedCheck(name, desc,
			fmt.Sprintf("Failed to fetch tool list: %v", err),
			"Check if the MCP server is running and supports tools/list",
		)
	}

	if len(tools) == 0 {
		return WarningCheck(name, desc,
			"No tools found on the server",
			"The server may not expose any tools, or there may be a configuration issue",
		)
	}

	return PassedCheck(name, desc, fmt.Sprintf("Successfully fetched %d tools", len(tools)))
}

func checkMcpToolSchema(s *Server) *CapabilityCheckItem {
	name := "tool_schema"
	desc := "Check if tool schemas are valid"

	if s.Url == "" {
		return SkippedCheck(name, desc, "Skipped: server URL is empty")
	}

	tools, err := mcp.GetToolsFromURL(s.Url, s.Token)
	if err != nil {
		return SkippedCheck(name, desc, "Skipped: cannot fetch tool list")
	}

	if len(tools) == 0 {
		return SkippedCheck(name, desc, "Skipped: no tools available")
	}

	validSchemas := 0
	for _, t := range tools {
		if t.InputSchema.Type != "" || len(t.InputSchema.Properties) > 0 {
			validSchemas++
		}
	}

	if validSchemas == 0 {
		return WarningCheck(name, desc,
			"No tools have input schemas defined",
			"Tools may work without schemas, but schema validation is recommended",
		)
	}

	return PassedCheck(name, desc, fmt.Sprintf("%d/%d tools have valid schemas", validSchemas, len(tools)))
}

func checkMcpDryRun(s *Server) *CapabilityCheckItem {
	name := "dry_run"
	desc := "Check if a dry-run tool call succeeds"

	if s.Url == "" {
		return SkippedCheck(name, desc, "Skipped: server URL is empty")
	}

	tools, err := mcp.GetToolsFromURL(s.Url, s.Token)
	if err != nil {
		return SkippedCheck(name, desc, "Skipped: cannot fetch tool list")
	}

	if len(tools) == 0 {
		return SkippedCheck(name, desc, "Skipped: no tools available")
	}

	testTool := findNoArgTool(tools)
	if testTool == nil {
		return WarningCheck(name, desc,
			"No zero-argument tool found for dry-run test",
			"Add testContent configuration to test a specific tool",
		)
	}

	_, err = mcp.CallTool(s.Url, s.Token, testTool.Name, map[string]interface{}{})
	if err != nil {
		return FailedCheck(name, desc,
			fmt.Sprintf("Dry-run call to '%s' failed: %v", testTool.Name, err),
			"Check tool permissions and server configuration",
		)
	}

	return PassedCheck(name, desc, fmt.Sprintf("Dry-run call to '%s' succeeded", testTool.Name))
}

func findNoArgTool(tools []*protocol.Tool) *protocol.Tool {
	for _, t := range tools {
		schema := t.InputSchema
		if len(schema.Required) == 0 && len(schema.Properties) == 0 {
			return t
		}
		if len(schema.Required) == 0 {
			return t
		}
	}
	return nil
}

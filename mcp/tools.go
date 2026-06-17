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

package mcp

import (
	"context"
	"fmt"

	"github.com/ThinkInAIXYZ/go-mcp/client"
	"github.com/ThinkInAIXYZ/go-mcp/protocol"
	"github.com/ThinkInAIXYZ/go-mcp/transport"
)

type ListToolsResult struct {
	Tools  []*protocol.Tool
	Error  error
	Client *client.Client
}

func ListToolsWithContext(tec *ToolExecutionContext, url, token string) *ListToolsResult {
	result := &ListToolsResult{}

	var tr transport.ClientTransport
	var err error

	if token != "" {
		tr, err = transport.NewStreamableHTTPClientTransport(url,
			transport.WithStreamableHTTPClientOptionHeader(map[string]string{
				"Authorization": "Bearer " + token,
			}),
		)
	} else {
		tr, err = transport.NewStreamableHTTPClientTransport(url)
	}
	if err != nil {
		result.Error = fmt.Errorf("mcp: create transport for %s: %w", url, err)
		return result
	}

	cli, err := client.NewClient(tr)
	if err != nil {
		result.Error = fmt.Errorf("mcp: create client for %s: %w", url, err)
		return result
	}
	result.Client = cli

	ctx, cancel := tec.GetTimeoutContext()
	defer cancel()

	// ---- SDK 原始调用边界 ----
	list, err := cli.ListTools(ctx)
	// --------------------------
	if err != nil {
		result.Error = fmt.Errorf("mcp: list tools from %s: %w", url, err)
		return result
	}
	result.Tools = list.Tools
	return result
}

// ---- 旧 API 兼容层 ----
// 新代码请使用 ListToolsWithContext 返回 ListToolsResult。
// 本函数仅保留向后兼容，内部直接委托给 ListToolsWithContext。
func GetToolsFromURL(url, token string) ([]*protocol.Tool, error) {
	tec := NewToolExecutionContext(context.Background()).WithTimeout(DefaultListToolsTimeout)
	result := ListToolsWithContext(tec, url, token)
	if result.Client != nil {
		defer result.Client.Close()
	}
	return result.Tools, result.Error
}

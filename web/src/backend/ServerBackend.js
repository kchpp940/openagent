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

import {ApiClient} from "./ApiClient";

export function getServers(owner, page = "", pageSize = "", field = "", value = "", sortField = "", sortOrder = "") {
  return ApiClient.get("/api/get-servers", {
    queryParams: {owner, p: page, pageSize, field, value, sortField, sortOrder},
  });
}

export function getServer(owner, name) {
  return ApiClient.get("/api/get-server", {
    queryParams: {id: `${owner}/${encodeURIComponent(name)}`},
  });
}

export function updateServer(owner, name, server) {
  return ApiClient.post("/api/update-server", {
    queryParams: {id: `${owner}/${encodeURIComponent(name)}`},
    body: server,
  });
}

export function addServer(server) {
  return ApiClient.post("/api/add-server", {
    body: server,
  });
}

export function deleteServer(server) {
  return ApiClient.post("/api/delete-server", {
    body: server,
  });
}

export function testMcpServer(server) {
  return ApiClient.post("/api/test-mcp-server", {
    body: server,
  });
}

export function syncMcpTool(owner, name, server, isCleared = false) {
  return ApiClient.post("/api/sync-mcp-tool", {
    queryParams: {id: `${owner}/${encodeURIComponent(name)}`, isCleared: isCleared ? "1" : "0"},
    body: server,
  });
}

export function getOnlineServers() {
  return ApiClient.get("/api/get-online-servers");
}

export function syncIntranetServers(cidr, ports = [], paths = []) {
  return ApiClient.post("/api/sync-intranet-servers", {
    body: {cidr, ports, paths},
  });
}

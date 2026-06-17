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

import {ApiClient} from "./ApiClient";

export function getGlobalTools() {
  return ApiClient.get("/api/get-global-tools");
}

export function getTools(owner, page = "", pageSize = "", field = "", value = "", sortField = "", sortOrder = "") {
  return ApiClient.get("/api/get-tools", {
    queryParams: {owner, p: page, pageSize, field, value, sortField, sortOrder},
  });
}

export function getTool(owner, name) {
  return ApiClient.get("/api/get-tool", {
    queryParams: {id: `${owner}/${encodeURIComponent(name)}`},
  });
}

export function updateTool(owner, name, tool) {
  return ApiClient.post("/api/update-tool", {
    queryParams: {id: `${owner}/${encodeURIComponent(name)}`},
    body: tool,
  });
}

export function addTool(tool) {
  return ApiClient.post("/api/add-tool", {
    body: tool,
  });
}

export function deleteTool(tool) {
  return ApiClient.post("/api/delete-tool", {
    body: tool,
  });
}

export function testTool(tool) {
  return ApiClient.post("/api/test-tool", {
    body: tool,
  });
}

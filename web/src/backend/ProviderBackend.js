// Copyright 2023 The OpenAgent Authors. All Rights Reserved.
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

export function getGlobalProviders() {
  return ApiClient.get("/api/get-global-providers");
}

export function getProviders(owner, storeName = "", page = "", pageSize = "", field = "", value = "", sortField = "", sortOrder = "") {
  return ApiClient.get("/api/get-providers", {queryParams: {owner, store: storeName, p: page, pageSize, field, value, sortField, sortOrder}});
}

export function getProvider(owner, name) {
  return ApiClient.get("/api/get-provider", {
    queryParams: {id: `${owner}/${encodeURIComponent(name)}`},
  });
}

export function updateProvider(owner, name, provider) {
  return ApiClient.post("/api/update-provider", {
    queryParams: {id: `${owner}/${encodeURIComponent(name)}`},
    body: provider,
  });
}

export function addProvider(provider) {
  return ApiClient.post("/api/add-provider", {body: provider});
}

export function deleteProvider(provider) {
  return ApiClient.post("/api/delete-provider", {body: provider});
}

export function getProviderModels(provider) {
  return ApiClient.post("/api/fetch-provider-models", {body: provider});
}

export function testTool(provider) {
  return ApiClient.post("/api/test-tool", {body: provider});
}

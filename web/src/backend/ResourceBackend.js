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

export function getGlobalResources(owner = "", page = "", pageSize = "", field = "", value = "", sortField = "", sortOrder = "") {
  return ApiClient.get("/api/get-global-resources", {queryParams: {owner, p: page, pageSize, field, value, sortField, sortOrder}});
}

export function getResource(owner, name) {
  return ApiClient.get("/api/get-resource", {
    queryParams: {id: `${owner}/${encodeURIComponent(name)}`},
  });
}

export function updateResource(owner, name, resource) {
  return ApiClient.post("/api/update-resource", {
    queryParams: {id: `${owner}/${encodeURIComponent(name)}`},
    body: resource,
  });
}

export function addResource(resource) {
  return ApiClient.post("/api/add-resource", {body: resource});
}

export function deleteResource(resource) {
  return ApiClient.post("/api/delete-resource", {body: resource});
}

// uploadResource sends a file as multipart/form-data and creates a Resource record.
// category: "avatar" | "chat" | "document"
export function uploadResource(user, category, objectType, objectId, file) {
  const formData = new FormData();
  formData.append("file", file);
  formData.append("category", category);
  formData.append("objectType", objectType || "");
  formData.append("objectId", objectId || "");
  return ApiClient.post("/api/upload-resource", {body: formData});
}

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

export function getGlobalFiles(store = "", page = "", pageSize = "", field = "", value = "", sortField = "", sortOrder = "") {
  return ApiClient.get("/api/get-global-files", {queryParams: {store, p: page, pageSize, field, value, sortField, sortOrder}});
}

export function getFiles(owner, store = "") {
  return ApiClient.get("/api/get-files", {queryParams: {owner, store}});
}

export function getFile(owner, name) {
  return ApiClient.get("/api/get-file", {
    queryParams: {id: `${owner}/${encodeURIComponent(name)}`},
  });
}

export function updateFile(owner, name, file) {
  return ApiClient.post("/api/update-file", {
    queryParams: {id: `${owner}/${encodeURIComponent(name)}`},
    body: file,
  });
}

export function uploadFile(filename, file) {
  const formData = new FormData();
  formData.append("file", file);
  return ApiClient.post("/api/upload-file", {queryParams: {filename: encodeURIComponent(filename)}, body: formData});
}

export function addFile(file) {
  return ApiClient.post("/api/add-file", {body: file});
}

export function deleteFile(file) {
  return ApiClient.post("/api/delete-file", {body: file});
}

export function refreshFileVectors(file) {
  return ApiClient.post("/api/refresh-file-vectors", {body: file});
}

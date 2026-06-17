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

export function getGlobalVectors() {
  return ApiClient.get("/api/get-global-vectors");
}

export function getVectors(owner, storeName, page = "", pageSize = "", field = "", value = "", sortField = "", sortOrder = "") {
  return ApiClient.get("/api/get-vectors", {queryParams: {owner, store: storeName, p: page, pageSize, field, value, sortField, sortOrder}});
}

export function getVector(owner, name) {
  return ApiClient.get("/api/get-vector", {
    queryParams: {id: `${owner}/${encodeURIComponent(name)}`},
  });
}

export function updateVector(owner, name, vector) {
  return ApiClient.post("/api/update-vector", {
    queryParams: {id: `${owner}/${encodeURIComponent(name)}`},
    body: vector,
  });
}

export function addVector(vector) {
  return ApiClient.post("/api/add-vector", {body: vector});
}

export function deleteVector(vector) {
  return ApiClient.post("/api/delete-vector", {body: vector});
}

export function deleteAllVectors() {
  return ApiClient.post("/api/delete-all-vectors");
}

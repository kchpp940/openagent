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

export function getHubStores() {
  return ApiClient.get("/api/get-hub-stores");
}

export function getGlobalStores(name = "", page = "", pageSize = "", field = "", value = "", sortField = "", sortOrder = "") {
  return ApiClient.get("/api/get-global-stores", {
    queryParams: {name, p: page, pageSize, field, value, sortField, sortOrder},
  });
}

export function getStores(owner) {
  return ApiClient.get("/api/get-stores", {
    queryParams: {owner},
  });
}

export function getStore(owner, name) {
  return ApiClient.get("/api/get-store", {
    queryParams: {id: `${owner}/${encodeURIComponent(name)}`},
  });
}

export function getStoreNames(owner) {
  return ApiClient.get("/api/get-store-names", {
    queryParams: {owner},
  });
}

export function updateStore(owner, name, store) {
  return ApiClient.post("/api/update-store", {
    queryParams: {id: `${owner}/${encodeURIComponent(name)}`},
    body: store,
  });
}

export function addStore(store) {
  return ApiClient.post("/api/add-store", {
    body: store,
  });
}

export function deleteStore(store) {
  return ApiClient.post("/api/delete-store", {
    body: store,
  });
}

export function refreshStoreVectors(store) {
  return ApiClient.post("/api/refresh-store-vectors", {
    body: store,
  });
}

export function claimStore(owner, name) {
  return ApiClient.post("/api/claim-store", {
    queryParams: {id: `${owner}/${encodeURIComponent(name)}`},
  });
}

export function addSharedStore(owner, name, targetUser) {
  return ApiClient.post("/api/add-shared-store", {
    body: {owner, name, targetUser},
  });
}

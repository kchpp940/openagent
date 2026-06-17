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

export function getGlobalPermissions() {
  return ApiClient.get("/api/get-global-permissions");
}

export function getPermissions(owner) {
  return ApiClient.get("/api/get-permissions", {queryParams: {owner}});
}

export function getPermission(owner, name) {
  return ApiClient.get("/api/get-permission", {
    queryParams: {id: `${owner}/${encodeURIComponent(name)}`},
  });
}

export function updatePermission(owner, name, permission) {
  return ApiClient.post("/api/update-permission", {
    queryParams: {id: `${owner}/${encodeURIComponent(name)}`},
    body: permission,
  });
}

export function addPermission(permission) {
  return ApiClient.post("/api/add-permission", {body: permission});
}

export function deletePermission(permission) {
  return ApiClient.post("/api/delete-permission", {body: permission});
}

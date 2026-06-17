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

export function getGlobalForms() {
  return ApiClient.get("/api/get-global-forms");
}

export function getForms(owner, page = "", pageSize = "", field = "", value = "", sortField = "", sortOrder = "") {
  return ApiClient.get("/api/get-forms", {queryParams: {owner, p: page, pageSize, field, value, sortField, sortOrder}});
}

export function getForm(owner, name) {
  return ApiClient.get("/api/get-form", {
    queryParams: {id: `${owner}/${encodeURIComponent(name)}`},
  });
}

export function updateForm(owner, name, form) {
  return ApiClient.post("/api/update-form", {
    queryParams: {id: `${owner}/${encodeURIComponent(name)}`},
    body: form,
  });
}

export function addForm(form) {
  return ApiClient.post("/api/add-form", {body: form});
}

export function deleteForm(form) {
  return ApiClient.post("/api/delete-form", {body: form});
}

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

export function getGlobalSkills() {
  return ApiClient.get("/api/get-global-skills");
}

export function getSkills(owner, page = "", pageSize = "", field = "", value = "", sortField = "", sortOrder = "") {
  return ApiClient.get("/api/get-skills", {
    queryParams: {owner, p: page, pageSize, field, value, sortField, sortOrder},
  });
}

export function getSkill(owner, name) {
  return ApiClient.get("/api/get-skill", {
    queryParams: {id: `${owner}/${encodeURIComponent(name)}`},
  });
}

export function updateSkill(owner, name, skill) {
  return ApiClient.post("/api/update-skill", {
    queryParams: {id: `${owner}/${encodeURIComponent(name)}`},
    body: skill,
  });
}

export function addSkill(skill) {
  return ApiClient.post("/api/add-skill", {
    body: skill,
  });
}

export function deleteSkill(skill) {
  return ApiClient.post("/api/delete-skill", {
    body: skill,
  });
}

export function loadSkill(path) {
  return ApiClient.get("/api/load-skill", {
    queryParams: {path: encodeURIComponent(path)},
  });
}

export function getMarketplaceSources() {
  return ApiClient.get("/api/get-marketplace-sources");
}

export function getMarketplaceSkills(source = "", keyword = "") {
  return ApiClient.get("/api/get-marketplace-skills", {
    queryParams: {source: encodeURIComponent(source), keyword: encodeURIComponent(keyword)},
  });
}

export function installMarketplaceSkill(item) {
  return ApiClient.post("/api/install-marketplace-skill", {
    body: item,
  });
}

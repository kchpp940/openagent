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

export function getGlobalSites() {
  return ApiClient.get("/api/get-global-sites");
}

export function getSites() {
  return ApiClient.get("/api/get-sites");
}

export function getSite(owner, name) {
  return ApiClient.get("/api/get-site", {
    queryParams: {id: `${owner}/${encodeURIComponent(name)}`},
  });
}

export function getBuiltInSite() {
  return ApiClient.get("/api/get-built-in-site");
}

export function updateSite(owner, name, site) {
  return ApiClient.post("/api/update-site", {
    queryParams: {id: `${owner}/${encodeURIComponent(name)}`},
    body: site,
  });
}

export function addSite(site) {
  return ApiClient.post("/api/add-site", {body: site});
}

export function deleteSite(site) {
  return ApiClient.post("/api/delete-site", {body: site});
}

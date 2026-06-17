// Copyright 2024 The OpenAgent Authors. All Rights Reserved.
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

import * as Setting from "../Setting";
import {ApiClient} from "./ApiClient";
import {ResponseAdapter} from "./ResponseAdapter";

// [EXEMPT] Custom serverUrl — uses ResponseAdapter directly instead of ApiClient
// because the URL target is a different host (monitoring server), not Setting.ServerUrl.
// ApiClient.buildUrl() always prefixes with Setting.ServerUrl, so it cannot be used here.
// ResponseAdapter.handleResponse() is used to ensure consistent error handling (throws ApiError).
function fetchWithCustomServerUrl(url) {
  return fetch(url, {
    method: "GET",
    credentials: "include",
    headers: {
      "Accept-Language": Setting.getAcceptLanguage(),
    },
  }).then(res => ResponseAdapter.handleResponse(res));
}

export function getUsages(serverUrl, storeName, selectedUser, days) {
  if (serverUrl === "") {
    return ApiClient.get("/api/get-usages", {queryParams: {days, store: storeName, selectedUser}});
  }
  return fetchWithCustomServerUrl(`${serverUrl}/api/get-usages?days=${days}&store=${storeName}&selectedUser=${selectedUser}`);
}

export function getRangeUsages(serverUrl, rangeType, count, storeName, selectedUser) {
  if (serverUrl === "") {
    return ApiClient.get("/api/get-range-usages", {queryParams: {rangeType, count, store: storeName, user: selectedUser}});
  }
  return fetchWithCustomServerUrl(`${serverUrl}/api/get-range-usages?rangeType=${rangeType}&count=${count}&store=${storeName}&user=${selectedUser}`);
}

export function getUsers(serverUrl, user, storeName = "") {
  if (serverUrl === "") {
    return ApiClient.get("/api/get-users", {queryParams: {user, store: storeName}});
  }
  return fetchWithCustomServerUrl(`${serverUrl}/api/get-users?user=${user}&store=${storeName}`);
}

export function getUserTableInfos(serverUrl, storeName, user) {
  if (serverUrl === "") {
    return ApiClient.get("/api/get-user-table-infos", {queryParams: {user, store: storeName}});
  }
  return fetchWithCustomServerUrl(`${serverUrl}/api/get-user-table-infos?user=${user}&store=${storeName}`);
}

export function getUsageProviders(owner) {
  return ApiClient.get("/api/get-usage-providers", {queryParams: {owner}});
}

export function getUsageHeatmap(owner) {
  return ApiClient.get("/api/get-usage-heatmap", {queryParams: {owner}});
}

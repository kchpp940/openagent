// Copyright 2023 The OpenAgent Authors.. All Rights Reserved.
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

export function getRecords(owner, page = "", pageSize = "", field = "", value = "", sortField = "", sortOrder = "") {
  return ApiClient.get("/api/get-records", {queryParams: {owner, p: page, pageSize, field, value, sortField, sortOrder}});
}

export function getRecord(owner, name) {
  return ApiClient.get("/api/get-record", {
    queryParams: {id: `${owner}/${encodeURIComponent(name)}`},
  });
}

export function updateRecord(owner, name, record) {
  return ApiClient.post("/api/update-record", {
    queryParams: {id: `${owner}/${encodeURIComponent(name)}`},
    body: record,
  });
}

export function addRecord(record) {
  return ApiClient.post("/api/add-record", {body: record});
}

export function deleteRecord(record) {
  return ApiClient.post("/api/delete-record", {body: record});
}

export function commitRecord(record) {
  return ApiClient.post("/api/commit-record", {body: record});
}

export function commitRecordSecond(record) {
  return ApiClient.post("/api/commit-record-second", {body: record});
}

export function queryRecord(owner, name) {
  return ApiClient.get("/api/query-record", {
    queryParams: {id: `${owner}/${encodeURIComponent(name)}`},
  });
}

export function queryRecordSecond(owner, name) {
  return ApiClient.get("/api/query-record-second", {
    queryParams: {id: `${owner}/${encodeURIComponent(name)}`},
  });
}

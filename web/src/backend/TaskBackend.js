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

export function getGlobalTasks() {
  return ApiClient.get("/api/get-global-tasks");
}

export function getTasks(owner, page = "", pageSize = "", field = "", value = "", sortField = "", sortOrder = "") {
  return ApiClient.get("/api/get-tasks", {
    queryParams: {owner, p: page, pageSize, field, value, sortField, sortOrder},
  });
}

export function getTask(owner, name) {
  return ApiClient.get("/api/get-task", {
    queryParams: {id: `${owner}/${encodeURIComponent(name)}`},
  });
}

export function updateTask(owner, name, task) {
  return ApiClient.post("/api/update-task", {
    queryParams: {id: `${owner}/${encodeURIComponent(name)}`},
    body: task,
  });
}

export function addTask(task) {
  return ApiClient.post("/api/add-task", {
    body: task,
  });
}

export function deleteTask(task) {
  return ApiClient.post("/api/delete-task", {
    body: task,
  });
}

export function uploadTaskDocument(taskId, base64, filename, filetype) {
  const formData = new FormData();
  formData.append("file", base64);
  formData.append("name", filename);
  formData.append("type", filetype);
  return ApiClient.post("/api/upload-task-document", {
    queryParams: {id: taskId},
    body: formData,
  });
}

export function analyzeTask(owner, name) {
  return ApiClient.post("/api/analyze-task", {
    queryParams: {id: `${owner}/${encodeURIComponent(name)}`},
  });
}

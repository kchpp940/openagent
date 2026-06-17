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

export function getGlobalPipes() {
  return ApiClient.get("/api/get-global-pipes");
}

export function getPipes(owner) {
  return ApiClient.get("/api/get-pipes", {queryParams: {owner}});
}

export function getPipe(owner, name) {
  return ApiClient.get("/api/get-pipe", {
    queryParams: {id: `${owner}/${encodeURIComponent(name)}`},
  });
}

export function updatePipe(owner, name, pipe) {
  return ApiClient.post("/api/update-pipe", {
    queryParams: {id: `${owner}/${encodeURIComponent(name)}`},
    body: pipe,
  });
}

export function addPipe(pipe) {
  return ApiClient.post("/api/add-pipe", {body: pipe});
}

export function deletePipe(pipe) {
  return ApiClient.post("/api/delete-pipe", {body: pipe});
}

export function setPipeWebhook(id) {
  return ApiClient.post("/api/set-pipe-webhook", {
    queryParams: {id: encodeURIComponent(id)},
  });
}

export function chatTest(id, chatId, message) {
  return ApiClient.post("/api/chat-test", {queryParams: {id: encodeURIComponent(id), chatId: encodeURIComponent(chatId), message: encodeURIComponent(message)}});
}

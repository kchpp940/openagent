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

export function getGlobalChats(page = "", pageSize = "", field = "", value = "", sortField = "", sortOrder = "", store = "") {
  return ApiClient.get("/api/get-global-chats", {queryParams: {p: page, pageSize, field, value, sortField, sortOrder, store: encodeURIComponent(store)}});
}

export function getChats(user, storeName = "", page = "", pageSize = "", field = "", value = "", sortField = "", sortOrder = "", selectedUser = "", startTime = "", endTime = "") {
  return ApiClient.get("/api/get-chats", {queryParams: {user, selectedUser, store: storeName, p: page, pageSize, field, value, sortField, sortOrder, startTime, endTime}});
}

export function getChat(owner, name) {
  return ApiClient.get("/api/get-chat", {
    queryParams: {id: `${owner}/${encodeURIComponent(name)}`},
  });
}

export function getChatStatus(owner, name) {
  return ApiClient.get("/api/get-chat-status", {
    queryParams: {id: `${owner}/${encodeURIComponent(name)}`},
  });
}

export function updateChat(owner, name, chat) {
  return ApiClient.post("/api/update-chat", {
    queryParams: {id: `${owner}/${encodeURIComponent(name)}`},
    body: chat,
  });
}

export function addChat(chat) {
  return ApiClient.post("/api/add-chat", {body: chat});
}

export function deleteChat(chat) {
  return ApiClient.post("/api/delete-chat", {body: chat});
}

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

import * as Setting from "../Setting";
import {ApiClient} from "./ApiClient";

export function getGlobalMessages(page = "", pageSize = "", field = "", value = "", sortField = "", sortOrder = "", store = "") {
  return ApiClient.get("/api/get-global-messages", {queryParams: {p: page, pageSize, field, value, sortField, sortOrder, store: encodeURIComponent(store)}});
}

export function getMessages(user, selectedUser = "") {
  return ApiClient.get("/api/get-messages", {queryParams: {user, selectedUser}});
}

export function getChatMessages(owner, chat) {
  return ApiClient.get("/api/get-messages", {queryParams: {owner, chat}});
}

const eventSourceMap = new Map();

export function getMessageAnswer(owner, name, onMessage, onReason, onTool, onSearch, onVector, onError, onEnd, onInfo, onChat, onToolDelta) {
  if (eventSourceMap.has(`${owner}/${name}`)) {
    return;
  }
  const eventSource = new EventSource(`${Setting.ServerUrl}/api/get-message-answer?id=${owner}/${encodeURIComponent(name)}`, {
    withCredentials: true,
  });
  eventSourceMap.set(`${owner}/${name}`, eventSource);

  eventSource.addEventListener("message", (e) => {
    onMessage(e.data);
  });

  eventSource.addEventListener("reason", (e) => {
    onReason(e.data);
  });

  eventSource.addEventListener("tool-start", (e) => {
    onTool(e.data);
  });

  eventSource.addEventListener("tool", (e) => {
    onTool(e.data);
  });

  if (onToolDelta) {
    eventSource.addEventListener("tool-delta", (e) => {
      onToolDelta(e.data);
    });
  }

  eventSource.addEventListener("search", (e) => {
    onSearch(e.data);
  });

  if (onVector) {
    eventSource.addEventListener("vector", (e) => {
      onVector(e.data);
    });
  }

  if (onInfo) {
    eventSource.addEventListener("myinfo", (e) => {
      onInfo(e.data);
    });
  }

  if (onChat) {
    eventSource.addEventListener("chat", (e) => {
      try {
        onChat(JSON.parse(e.data));
      } catch {
        // ignore malformed chat events
      }
    });
  }

  eventSource.addEventListener("myerror", (e) => {
    onError(e.data);
    eventSource.close();
    eventSourceMap.delete(`${owner}/${name}`);
  });

  eventSource.addEventListener("error", (e) => {
    let error = e.data;
    if (!error) {
      error = "Unknown error";
    }
    onError(error);
    eventSource.close();
    eventSourceMap.delete(`${owner}/${name}`);
  });

  eventSource.addEventListener("end", (e) => {
    onEnd(e.data);
    eventSource.close();
    eventSourceMap.delete(`${owner}/${name}`);
  });
}

export function getAnswer(provider, question, framework, video, tool = "") {
  return ApiClient.get("/api/get-answer", {queryParams: {provider, question: encodeURIComponent(question), framework: encodeURIComponent(framework), video: encodeURIComponent(video), tool: encodeURIComponent(tool)}});
}

export function getMessage(owner, name) {
  return ApiClient.get("/api/get-message", {
    queryParams: {id: `${owner}/${encodeURIComponent(name)}`},
  });
}

export function updateMessage(owner, name, message, isHitOnly = false) {
  return ApiClient.post("/api/update-message", {
    queryParams: {id: `${owner}/${encodeURIComponent(name)}`, isHitOnly},
    body: message,
  });
}

export function closeMessageEventSource(owner, name, cancel = false) {
  const key = `${owner}/${name}`;
  const found = eventSourceMap.has(key);
  if (found) {
    eventSourceMap.get(key).close();
    eventSourceMap.delete(key);
  }
  if (cancel) {
    cancelMessageAnswer(owner, name).catch(() => {});
  }
  return found;
}

export function cancelMessageAnswer(owner, name) {
  return ApiClient.post("/api/cancel-message-answer", {
    queryParams: {id: `${encodeURIComponent(owner)}/${encodeURIComponent(name)}`},
  });
}

export function addMessage(message) {
  return ApiClient.post("/api/add-message", {body: message});
}

export function deleteMessage(message) {
  return ApiClient.post("/api/delete-message", {body: message});
}

export function deleteWelcomeMessage(message) {
  return ApiClient.post("/api/delete-welcome-message", {body: message});
}

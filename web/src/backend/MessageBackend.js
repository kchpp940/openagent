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

export function getGlobalMessages(page = "", pageSize = "", field = "", value = "", sortField = "", sortOrder = "", store = "") {
  return fetch(`${Setting.ServerUrl}/api/get-global-messages?p=${page}&pageSize=${pageSize}&field=${field}&value=${value}&sortField=${sortField}&sortOrder=${sortOrder}&store=${encodeURIComponent(store)}`, {
    method: "GET",
    credentials: "include",
    headers: {
      "Accept-Language": Setting.getAcceptLanguage(),
    },
  }).then(res => Setting.handleFetchResponse(res));
}

export function getMessages(user, selectedUser = "") {
  return fetch(`${Setting.ServerUrl}/api/get-messages?user=${user}&selectedUser=${selectedUser}`, {
    method: "GET",
    credentials: "include",
    headers: {
      "Accept-Language": Setting.getAcceptLanguage(),
    },
  }).then(res => Setting.handleFetchResponse(res));
}

export function getChatMessages(owner, chat) {
  return fetch(`${Setting.ServerUrl}/api/get-messages?owner=${owner}&chat=${chat}`, {
    method: "GET",
    credentials: "include",
    headers: {
      "Accept-Language": Setting.getAcceptLanguage(),
    },
  }).then(res => Setting.handleFetchResponse(res));
}

const eventSourceMap = new Map();

export function safeJsonParse(str, fallback = null) {
  if (str === null || str === undefined) {
    return fallback;
  }
  if (typeof str !== "string") {
    return str;
  }
  const trimmed = str.trim();
  if (trimmed === "") {
    return fallback;
  }
  try {
    return JSON.parse(trimmed);
  } catch (e) {
    console.warn("[MessageBackend] JSON parse failed, raw input:", str.slice(0, 200), "error:", e.message);
    return fallback;
  }
}

class SSEParser {
  constructor() {
    this.buffer = "";
  }

  feed(chunk) {
    this.buffer += chunk;
    return this.readFrames();
  }

  readFrames() {
    const frames = [];
    while (true) {
      const idx = this.buffer.indexOf("\n\n");
      if (idx === -1) {
        break;
      }
      const frameStr = this.buffer.slice(0, idx);
      this.buffer = this.buffer.slice(idx + 2);
      if (frameStr.length === 0) {
        continue;
      }
      const frame = this.parseFrame(frameStr);
      if (frame.event || frame.data) {
        frames.push(frame);
      }
    }
    return frames;
  }

  parseFrame(frameStr) {
    const lines = frameStr.split("\n");
    let event = "";
    const dataLines = [];
    for (const rawLine of lines) {
      const line = rawLine;
      if (line.length === 0) {
        continue;
      }
      if (line[0] === ":") {
        continue;
      }
      let sepIdx = line.indexOf(": ");
      if (sepIdx === -1) {
        sepIdx = line.indexOf(":");
        if (sepIdx === -1) {
          continue;
        }
      }
      const field = line.slice(0, sepIdx);
      let value;
      if (sepIdx + 1 < line.length && line[sepIdx + 1] === " ") {
        value = line.slice(sepIdx + 2);
      } else {
        value = line.slice(sepIdx + 1);
      }
      if (field === "event") {
        event = value;
      } else if (field === "data") {
        dataLines.push(value);
      }
    }
    const data = dataLines.join("\n");
    if (!event && dataLines.length > 0) {
      event = "message";
    }
    return {event, data};
  }

  remaining() {
    return this.buffer;
  }
}

async function streamWithFetch(url, handlers) {
  const {onMessage, onReason, onToolStart, onTool, onToolDelta, onSearch, onVector, onError, onEnd, onInfo, onChat} = handlers;
  let closed = false;
  const parser = new SSEParser();
  let response;
  try {
    response = await fetch(url, {
      method: "GET",
      credentials: "include",
      headers: {
        "Accept": "text/event-stream",
        "Cache-Control": "no-cache",
        "Accept-Language": Setting.getAcceptLanguage(),
      },
    });
  } catch (fetchErr) {
    if (!closed) {
      onError && onError(`Connection failed: ${fetchErr.message}`);
    }
    return;
  }

  if (!response.ok) {
    let errText = `HTTP ${response.status}`;
    try {
      errText = await response.text();
    } catch (_) {
      // ignore
    }
    onError && onError(errText);
    return;
  }

  const reader = response.body?.getReader();
  if (!reader) {
    onError && onError("Streaming not supported by this browser");
    return;
  }

  const decoder = new TextDecoder("utf-8");
  const dispatchFrame = (frame) => {
    if (closed) return;
    const {event, data} = frame;
    switch (event) {
      case "message":
        onMessage && onMessage(data);
        break;
      case "reason":
        onReason && onReason(data);
        break;
      case "tool-start":
        onToolStart && onToolStart(data);
        onTool && onTool(data);
        break;
      case "tool-delta":
        onToolDelta && onToolDelta(data);
        break;
      case "tool":
        onTool && onTool(data);
        break;
      case "search":
        onSearch && onSearch(data);
        break;
      case "vector":
        onVector && onVector(data);
        break;
      case "myinfo":
        onInfo && onInfo(data);
        break;
      case "chat":
        onChat && onChat(data);
        break;
      case "myerror":
        closed = true;
        onError && onError(data);
        break;
      case "error":
        closed = true;
        onError && onError(data || "Unknown error");
        break;
      case "end":
        closed = true;
        onEnd && onEnd(data);
        break;
      default:
        break;
    }
  };

  try {
    while (true) {
      const {done, value} = await reader.read();
      if (done) {
        const rest = parser.remaining();
        if (rest && rest.trim().length > 0) {
          let fallbackFrame;
          if (rest.startsWith("event:")) {
            fallbackFrame = parser.parseFrame(rest);
          } else {
            fallbackFrame = {event: "message", data: rest};
          }
          if (fallbackFrame.data || fallbackFrame.event) {
            dispatchFrame(fallbackFrame);
          }
        }
        if (!closed) {
          closed = true;
          onEnd && onEnd("stream-closed");
        }
        break;
      }
      const text = decoder.decode(value, {stream: true});
      const frames = parser.feed(text);
      for (const frame of frames) {
        dispatchFrame(frame);
      }
    }
  } catch (streamErr) {
    if (!closed) {
      closed = true;
      onError && onError(`Stream error: ${streamErr.message}`);
    }
  } finally {
    try {
      reader.releaseLock();
    } catch (_) {
      // ignore
    }
  }
}

export function getMessageAnswer(owner, name, onMessage, onReason, onTool, onSearch, onVector, onError, onEnd, onInfo, onChat, onToolDelta) {
  const key = `${owner}/${name}`;
  if (eventSourceMap.has(key)) {
    return;
  }

  const url = `${Setting.ServerUrl}/api/get-message-answer?id=${owner}/${encodeURIComponent(name)}`;

  const useFetch = typeof ReadableStream !== "undefined" && typeof TextDecoder !== "undefined";

  const cleanup = () => {
    eventSourceMap.delete(key);
  };

  const wrappedOnError = (error) => {
    cleanup();
    onError && onError(error);
  };

  const wrappedOnEnd = (data) => {
    cleanup();
    onEnd && onEnd(data);
  };

  if (useFetch) {
    const ctrl = {closed: false, abort: null};
    const abortController = typeof AbortController !== "undefined" ? new AbortController() : null;
    ctrl.abort = () => {
      ctrl.closed = true;
      if (abortController) {
        try { abortController.abort(); } catch (_) { /* ignore */ }
      }
    };
    eventSourceMap.set(key, {type: "fetch", abort: ctrl.abort});

    const safeOnInfo = onInfo;
    const safeOnChat = onChat;

    streamWithFetch(url, {
      onMessage,
      onReason,
      onToolStart: onTool,
      onTool,
      onToolDelta,
      onSearch,
      onVector,
      onError: wrappedOnError,
      onEnd: wrappedOnEnd,
      onInfo: safeOnInfo,
      onChat: safeOnChat,
    }).catch((err) => {
      if (!ctrl.closed) {
        wrappedOnError(err && err.message ? err.message : String(err));
      }
    });
    return;
  }

  const eventSource = new EventSource(url, {
    withCredentials: true,
  });
  eventSourceMap.set(key, {type: "eventsource", close: () => eventSource.close()});

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
      const parsed = safeJsonParse(e.data, null);
      if (parsed !== null) {
        onChat(parsed);
      }
    });
  }

  eventSource.addEventListener("myerror", (e) => {
    wrappedOnError(e.data);
    eventSource.close();
  });

  eventSource.addEventListener("error", (e) => {
    let error = e.data;
    if (!error) {
      error = "Unknown error";
    }
    wrappedOnError(error);
    eventSource.close();
  });

  eventSource.addEventListener("end", (e) => {
    wrappedOnEnd(e.data);
    eventSource.close();
  });
}

export function getAnswer(provider, question, framework, video, tool = "") {
  return fetch(`${Setting.ServerUrl}/api/get-answer?provider=${provider}&question=${encodeURIComponent(question)}&framework=${encodeURIComponent(framework)}&video=${encodeURIComponent(video)}&tool=${encodeURIComponent(tool)}`, {
    method: "GET",
    credentials: "include",
    headers: {
      "Accept-Language": Setting.getAcceptLanguage(),
    },
  }).then(res => Setting.handleFetchResponse(res));
}

export function getMessage(owner, name) {
  return fetch(`${Setting.ServerUrl}/api/get-message?id=${owner}/${encodeURIComponent(name)}`, {
    method: "GET",
    credentials: "include",
    headers: {
      "Accept-Language": Setting.getAcceptLanguage(),
    },
  }).then(res => Setting.handleFetchResponse(res));
}

export function updateMessage(owner, name, message, isHitOnly = false) {
  const newMessage = Setting.deepCopy(message);
  return fetch(`${Setting.ServerUrl}/api/update-message?id=${owner}/${encodeURIComponent(name)}&isHitOnly=${isHitOnly}`, {
    method: "POST",
    credentials: "include",
    headers: {
      "Accept-Language": Setting.getAcceptLanguage(),
    },
    body: JSON.stringify(newMessage),
  }).then(res => Setting.handleFetchResponse(res));
}

export function closeMessageEventSource(owner, name, cancel = false) {
  const key = `${owner}/${name}`;
  const found = eventSourceMap.has(key);
  if (found) {
    const entry = eventSourceMap.get(key);
    eventSourceMap.delete(key);
    if (entry) {
      if (entry.type === "eventsource" && typeof entry.close === "function") {
        entry.close();
      } else if (entry.type === "fetch" && typeof entry.abort === "function") {
        entry.abort();
      }
    }
  }
  if (cancel) {
    cancelMessageAnswer(owner, name).catch(() => {});
  }
  return found;
}

export function cancelMessageAnswer(owner, name) {
  return fetch(`${Setting.ServerUrl}/api/cancel-message-answer?id=${encodeURIComponent(owner)}/${encodeURIComponent(name)}`, {
    method: "POST",
    credentials: "include",
    headers: {
      "Accept-Language": Setting.getAcceptLanguage(),
    },
  }).then(res => Setting.handleFetchResponse(res));
}

export function addMessage(message) {
  const newMessage = Setting.deepCopy(message);
  return fetch(`${Setting.ServerUrl}/api/add-message`, {
    method: "POST",
    credentials: "include",
    headers: {
      "Accept-Language": Setting.getAcceptLanguage(),
    },
    body: JSON.stringify(newMessage),
  }).then(res => Setting.handleFetchResponse(res));
}

export function deleteMessage(message) {
  const newMessage = Setting.deepCopy(message);
  return fetch(`${Setting.ServerUrl}/api/delete-message`, {
    method: "POST",
    credentials: "include",
    headers: {
      "Accept-Language": Setting.getAcceptLanguage(),
    },
    body: JSON.stringify(newMessage),
  }).then(res => Setting.handleFetchResponse(res));
}

export function deleteWelcomeMessage(message) {
  const newMessage = Setting.deepCopy(message);
  return fetch(`${Setting.ServerUrl}/api/delete-welcome-message`, {
    method: "POST",
    credentials: "include",
    headers: {
      "Accept-Language": Setting.getAcceptLanguage(),
    },
    body: JSON.stringify(newMessage),
  }).then(res => Setting.handleFetchResponse(res));
}

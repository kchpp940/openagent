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

import * as Setting from "../Setting";

// [EXEMPT] Blob response — cannot use ApiClient.
// ApiClient always parses the response as JSON via ResponseAdapter.handleResponse(),
// but this endpoint returns binary audio data (audio/mpeg) on success.
// The response Content-Type must be inspected before deciding how to parse:
//   - Non-JSON (audio) → return response.blob()
//   - JSON (error) → parse as JSON and throw
export function generateTextToSpeechAudio(storeId, providerId, messageId, text) {
  const payload = {
    storeId: storeId,
    providerId: providerId,
    messageId: messageId,
    text: text,
  };

  return fetch(`${Setting.ServerUrl}/api/generate-text-to-speech-audio`, {
    method: "POST",
    credentials: "include",
    headers: {
      "Accept-Language": Setting.getAcceptLanguage(),
    },
    body: JSON.stringify(payload),
  }).then(async response => {
    const contentType = response.headers.get("Content-Type") || "";
    const treatAsError = !response.ok || contentType.includes("application/json");
    if (!treatAsError) {
      return response.blob();
    }
    const text = await response.text();
    let data = null;
    try {
      data = JSON.parse(text);
    } catch (e) {
      // ignore parse error
    }
    const msg = (data && (data.msg || data.message)) || `HTTP ${response.status}`;
    throw new Error(msg || "TTS request failed");
  });
}

// [EXEMPT] EventSource stream — cannot use ApiClient.
// This returns an EventSource object for server-sent events (streaming audio).
// ApiClient is designed for request/response patterns and would incorrectly
// try to parse the entire stream as a single JSON response.
export function generateTextToSpeechAudioStream(storeId, messageId) {
  const url = `${Setting.ServerUrl}/api/generate-text-to-speech-audio-stream?storeId=${encodeURIComponent(storeId)}&messageId=${encodeURIComponent(messageId)}`;

  return new EventSource(url, {
    withCredentials: true,
  });
}

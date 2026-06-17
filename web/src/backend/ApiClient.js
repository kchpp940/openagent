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

/**
 * ApiClient — Unified Frontend API Calling Layer
 *
 * ══════════════════════════════════════════════════════════════
 *  WHEN TO USE EACH PATH (read before adding new Backend calls)
 * ══════════════════════════════════════════════════════════════
 *
 *  1. Normal JSON request/response  →  ApiClient.get / post / put / delete
 *     This is the DEFAULT path. Use it for 95%+ of Backend methods.
 *
 *       import {ApiClient} from "./ApiClient";
 *       return ApiClient.get("/api/get-foo", {queryParams: {id, owner}});
 *       return ApiClient.post("/api/update-foo", {body: fooObj});
 *
 *     - Auto-appends Setting.ServerUrl + path
 *     - Auto-encodes queryParams (skips null/undefined/"")
 *     - JSON body: auto deepCopy + stringify, sets Content-Type: application/json
 *     - FormData body: auto-detected, Content-Type omitted (browser sets multipart boundary)
 *     - Response: ResponseAdapter.handleResponse() parses JSON, throws ApiError on failure
 *     - On success: resolves with the full parsed JSON object ({status, data, data2, msg, ...})
 *     - On failure: rejects with ApiError (includes .code, .message, .raw)
 *
 *  2. FormData / file upload  →  ApiClient.post(path, {body: formData})
 *     Just pass a FormData instance as body. ApiClient auto-detects it.
 *     Do NOT set Content-Type manually.
 *
 *  3. Blob response (binary download)  →  Raw fetch + response.blob()
 *     ApiClient always parses response as JSON, so it CANNOT handle blob responses.
 *     Use raw fetch() and add a [EXEMPT] comment with reason.
 *
 *       // [EXEMPT] Blob response — ApiClient parses JSON only.
 *       return fetch(url, {...}).then(async res => {
 *         const ct = res.headers.get("Content-Type") || "";
 *         if (!res.ok || ct.includes("application/json")) { throw ... }
 *         return res.blob();
 *       });
 *
 *  4. EventSource / SSE stream  →  new EventSource(url, {withCredentials: true})
 *     ApiClient is request/response; it cannot handle streaming.
 *     Use EventSource directly and add a [EXEMPT] comment with reason.
 *
 *       // [EXEMPT] EventSource stream — ApiClient is not streaming-capable.
 *       return new EventSource(url, {withCredentials: true});
 *
 *  5. Custom serverUrl (different host)  →  Raw fetch + ResponseAdapter.handleResponse()
 *     ApiClient.buildUrl() always prefixes Setting.ServerUrl, so it CANNOT target
 *     a different host. Use raw fetch() but pipe through ResponseAdapter.handleResponse()
 *     so that error handling (throw ApiError) stays consistent with ApiClient.
 *
 *       import {ResponseAdapter} from "./ResponseAdapter";
 *       // [EXEMPT] Custom serverUrl — ApiClient.buildUrl() uses Setting.ServerUrl only.
 *       return fetch(`${serverUrl}/api/...`, {...}).then(res => ResponseAdapter.handleResponse(res));
 *
 *  6. Plain text response (source file viewer)  →  Raw fetch + response.text()
 *     For viewing file contents (not JSON). Add a [EXEMPT] comment with reason.
 *
 *       // [EXEMPT] Plain text response — not JSON.
 *       fetch(url, {...}).then(res => res.text())
 *
 * ══════════════════════════════════════════════════════════════
 *  PAGE COMPONENT PATTERN (use in .then() / .catch())
 * ══════════════════════════════════════════════════════════════
 *
 *  Modify operation (add/delete/save):
 *    Backend.addXxx(obj)
 *      .then(() => Setting.ResponseAdapter.showSuccessMessage(i18next.t("general:Successfully added")))
 *      .catch(error => Setting.ResponseAdapter.showErrorMessage(error, i18next.t("general:Failed to add")));
 *
 *  Fetch operation (get/list):
 *    Backend.getXxx(...)
 *      .then(res => this.setState({data: res.data, loading: false}))
 *      .catch(error => {
 *        if (error instanceof Setting.ApiError && error.isPermissionDenied()) {
 *          this.setState({isAuthorized: false, loading: false});
 *        } else {
 *          this.setState({loading: false});
 *          Setting.ResponseAdapter.showErrorMessage(error, i18next.t("general:Failed to get"));
 *        }
 *      });
 *
 * ══════════════════════════════════════════════════════════════
 *  [EXEMPT] ANNOTATION POLICY
 * ══════════════════════════════════════════════════════════════
 *
 *  Any use of raw fetch() / EventSource / Setting.handleFetchResponse() in Backend
 *  files MUST have a [EXEMPT] comment explaining:
 *    1. Which category above it falls into (Blob/EventSource/CustomUrl/PlainText)
 *    2. Why ApiClient cannot be used
 *  Without this annotation, the code should be refactored to use ApiClient.
 */

import * as Setting from "../Setting";
import {ApiError, ErrorCode, ResponseAdapter} from "./ResponseAdapter";

function buildUrl(path, queryParams = {}) {
  const baseUrl = Setting.ServerUrl;
  let url = baseUrl + path;

  const params = new URLSearchParams();
  let hasParams = false;
  for (const [key, value] of Object.entries(queryParams)) {
    if (value !== undefined && value !== null && value !== "") {
      params.append(key, value);
      hasParams = true;
    }
  }
  if (hasParams) {
    const separator = url.includes("?") ? "&" : "?";
    url += separator + params.toString();
  }

  return url;
}

function buildHeaders(extraHeaders = {}) {
  const headers = {
    "Accept-Language": Setting.getAcceptLanguage(),
    ...extraHeaders,
  };
  return headers;
}

function buildOptions(method, options = {}) {
  const {body, headers, ...restOptions} = options;

  const fetchOptions = {
    method,
    credentials: "include",
    headers: buildHeaders(headers),
    ...restOptions,
  };

  if (body !== undefined && body !== null) {
    if (body instanceof FormData) {
      fetchOptions.body = body;
    } else {
      fetchOptions.headers["Content-Type"] = "application/json";
      fetchOptions.body = JSON.stringify(Setting.deepCopy(body));
    }
  }

  return fetchOptions;
}

async function request(path, method, options = {}) {
  const {queryParams, body, headers, ...fetchOptions} = options;
  const url = buildUrl(path, queryParams);
  const config = buildOptions(method, {body, headers, ...fetchOptions});

  try {
    const response = await fetch(url, config);
    return await ResponseAdapter.handleResponse(response);
  } catch (error) {
    if (error instanceof ApiError) {
      throw error;
    }
    throw new ApiError(
      ErrorCode.NETWORK_ERROR,
      error?.message || "Network request failed",
      null
    );
  }
}

export const ApiClient = {
  get(path, options = {}) {
    return request(path, "GET", options);
  },

  post(path, options = {}) {
    return request(path, "POST", options);
  },

  put(path, options = {}) {
    return request(path, "PUT", options);
  },

  delete(path, options = {}) {
    return request(path, "DELETE", options);
  },

  request,
};

export {ApiError, ErrorCode} from "./ResponseAdapter";

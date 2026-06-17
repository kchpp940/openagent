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

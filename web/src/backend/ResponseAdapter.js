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

import i18next from "i18next";
import * as Setting from "../Setting";

export const ErrorCode = {
  SUCCESS: "ok",
  ERROR: "error",
  UNAUTHORIZED: "unauthorized",
  FORBIDDEN: "forbidden",
  NOT_FOUND: "not_found",
  NETWORK_ERROR: "network_error",
  INVALID_RESPONSE: "invalid_response",
  SESSION_EXPIRED: "session_expired",
  OPERATION_DENIED: "operation_denied",
};

const DENIAL_MESSAGES = [
  "Unauthorized operation",
  "this operation requires admin privilege",
];

const SESSION_EXPIRED_MESSAGES = [
  "session expired",
  "login required",
  "not logged in",
  "unauthenticated",
];

export class ApiError extends Error {
  constructor(code, message, raw = null) {
    super(message);
    this.name = "ApiError";
    this.code = code;
    this.raw = raw;
  }

  static fromResponse(data, httpStatus) {
    const code = data?.status || ErrorCode.ERROR;
    const message = data?.msg || data?.message || `HTTP ${httpStatus}`;
    return new ApiError(code, message, data);
  }

  isSessionExpired() {
    if (this.code === ErrorCode.SESSION_EXPIRED) {
      return true;
    }
    const msg = (this.message || "").toLowerCase();
    return SESSION_EXPIRED_MESSAGES.some(m => msg.includes(m.toLowerCase()));
  }

  isPermissionDenied() {
    if (this.code === ErrorCode.FORBIDDEN || this.code === ErrorCode.OPERATION_DENIED) {
      return true;
    }
    return DENIAL_MESSAGES.some(m => this.message === m);
  }

  isUnauthorized() {
    return this.code === ErrorCode.UNAUTHORIZED || this.isSessionExpired();
  }

  getDisplayMessage(prefix = "") {
    const baseMessage = this.getUserFriendlyMessage();
    if (prefix) {
      return `${prefix}: ${baseMessage}`;
    }
    return baseMessage;
  }

  getUserFriendlyMessage() {
    switch (this.code) {
    case ErrorCode.NETWORK_ERROR:
      return i18next.t("general:Failed to connect to server");
    case ErrorCode.INVALID_RESPONSE:
      return i18next.t("general:Invalid response from server");
    case ErrorCode.SESSION_EXPIRED:
      return i18next.t("general:Session expired, please login again");
    case ErrorCode.OPERATION_DENIED:
    case ErrorCode.FORBIDDEN:
      return i18next.t("general:Permission denied");
    default:
      return this.message || i18next.t("general:Unknown error");
    }
  }
}

export const ResponseAdapter = {
  async handleResponse(response) {
    const text = await response.text();
    let data = null;

    if (text) {
      try {
        data = JSON.parse(text);
      } catch (e) {
        if (!response.ok) {
          const preview = text.replace(/\s+/g, " ").trim().slice(0, 160);
          const msg = `HTTP ${response.status} ${response.statusText || ""}`.trim() +
            (preview ? `: ${preview}` : "");
          throw new ApiError(
            response.status === 401 ? ErrorCode.UNAUTHORIZED :
              response.status === 403 ? ErrorCode.FORBIDDEN :
                response.status === 404 ? ErrorCode.NOT_FOUND :
                  ErrorCode.INVALID_RESPONSE,
            msg,
            null
          );
        }
        throw new ApiError(
          ErrorCode.INVALID_RESPONSE,
          "Invalid JSON response",
          null
        );
      }
    }

    if (!response.ok) {
      const error = ApiError.fromResponse(data, response.status);

      if (response.status === 401 || error.isSessionExpired()) {
        error.code = ErrorCode.SESSION_EXPIRED;
        ResponseAdapter.handleSessionExpired();
      } else if (response.status === 403 || error.isPermissionDenied()) {
        error.code = ErrorCode.OPERATION_DENIED;
      }

      throw error;
    }

    if (data && data.status !== undefined && data.status !== ErrorCode.SUCCESS) {
      const error = ApiError.fromResponse(data, response.status);

      if (error.isSessionExpired()) {
        error.code = ErrorCode.SESSION_EXPIRED;
        ResponseAdapter.handleSessionExpired();
      } else if (error.isPermissionDenied()) {
        error.code = ErrorCode.OPERATION_DENIED;
      }

      throw error;
    }

    return data;
  },

  handleSessionExpired() {
    Setting.redirectToLogin();
  },

  isResponseDenied(data) {
    if (!data) {return false;}
    return DENIAL_MESSAGES.some(m => data.msg === m);
  },

  getErrorMessage(error, prefix = "") {
    if (error instanceof ApiError) {
      return error.getDisplayMessage(prefix);
    }
    const message = error?.message || String(error);
    if (prefix) {
      return `${prefix}: ${message}`;
    }
    return message;
  },

  showErrorMessage(error, prefix = "") {
    const message = ResponseAdapter.getErrorMessage(error, prefix);
    Setting.showMessage("error", message);
  },

  showSuccessMessage(text) {
    Setting.showMessage("success", text);
  },

  withNotification(promise, successMsg, errorPrefix) {
    return promise
      .then(res => {
        if (successMsg) {
          ResponseAdapter.showSuccessMessage(successMsg);
        }
        return res;
      })
      .catch(error => {
        if (errorPrefix !== undefined && errorPrefix !== null) {
          ResponseAdapter.showErrorMessage(error, errorPrefix);
        }
        throw error;
      });
  },

  withSilentError(promise) {
    return promise.catch(error => {
      if (!(error instanceof ApiError) || !error.isPermissionDenied()) {
        ResponseAdapter.showErrorMessage(error);
      }
      throw error;
    });
  },
};

export default ResponseAdapter;

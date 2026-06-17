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

export function getAccount() {
  const fromPath = encodeURIComponent(window.location.pathname);
  return ApiClient.get("/api/get-account", {queryParams: {fromPath}});
}

export function getSigninOptions() {
  return ApiClient.get("/api/get-signin-options");
}

export function updateAccount(account) {
  return ApiClient.post("/api/update-account", {body: account});
}

export function signin(code, state) {
  return ApiClient.post("/api/signin", {queryParams: {code, state}});
}

export function signinWithPassword(username, password) {
  return ApiClient.post("/api/signin", {body: {username, password}});
}

export function signout() {
  return ApiClient.post("/api/signout");
}

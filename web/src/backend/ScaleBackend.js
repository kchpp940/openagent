// Copyright 2025 The OpenAgent Authors. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

import {ApiClient} from "./ApiClient";

export function getGlobalScales() {
  return ApiClient.get("/api/get-global-scales");
}

export function getScales(owner, page = "", pageSize = "", field = "", value = "", sortField = "", sortOrder = "") {
  return ApiClient.get("/api/get-scales", {queryParams: {owner, p: page, pageSize, field, value, sortField, sortOrder}});
}

export function getScale(owner, name) {
  return ApiClient.get("/api/get-scale", {
    queryParams: {id: `${owner}/${encodeURIComponent(name)}`},
  });
}

export function getPublicScales() {
  return ApiClient.get("/api/get-public-scales");
}

export function updateScale(owner, name, scale) {
  return ApiClient.post("/api/update-scale", {
    queryParams: {id: `${owner}/${encodeURIComponent(name)}`},
    body: scale,
  });
}

export function addScale(scale) {
  return ApiClient.post("/api/add-scale", {body: scale});
}

export function deleteScale(scale) {
  return ApiClient.post("/api/delete-scale", {body: scale});
}

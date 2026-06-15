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

export function createTaskAnalysisItem() {
  return {
    name: "",
    score: 0,
    advantage: "",
    disadvantage: "",
    suggestion: "",
  };
}

export function createTaskAnalysisCategory() {
  return {
    name: "",
    score: 0,
    items: [],
  };
}

export function createTaskAnalysisReport() {
  return {
    title: "",
    designer: "",
    stage: "",
    participants: "",
    grade: "",
    instructor: "",
    subject: "",
    school: "",
    otherSubjects: "",
    textbook: "",
    score: 0,
    summary: "",
    categories: [],
  };
}

function parseNumericScore(value) {
  if (typeof value === "number") {
    return value;
  }
  const num = Number(value);
  return Number.isFinite(num) ? num : 0;
}

function normalizeItem(raw) {
  return {
    name: String(raw.name ?? ""),
    score: parseNumericScore(raw.score),
    advantage: String(raw.advantage ?? ""),
    disadvantage: String(raw.disadvantage ?? ""),
    suggestion: String(raw.suggestion ?? ""),
  };
}

function normalizeCategory(raw) {
  return {
    name: String(raw.name ?? ""),
    score: parseNumericScore(raw.score),
    items: Array.isArray(raw.items) ? raw.items.map(normalizeItem) : [],
  };
}

export function normalizeTaskAnalysisReport(raw) {
  if (!raw) {
    return createTaskAnalysisReport();
  }
  if (typeof raw === "string") {
    try {
      raw = JSON.parse(raw);
    } catch {
      return createTaskAnalysisReport();
    }
  }
  return {
    title: String(raw.title ?? ""),
    designer: String(raw.designer ?? ""),
    stage: String(raw.stage ?? ""),
    participants: String(raw.participants ?? ""),
    grade: String(raw.grade ?? ""),
    instructor: String(raw.instructor ?? ""),
    subject: String(raw.subject ?? ""),
    school: String(raw.school ?? ""),
    otherSubjects: String(raw.otherSubjects ?? ""),
    textbook: String(raw.textbook ?? ""),
    score: parseNumericScore(raw.score),
    summary: String(raw.summary ?? ""),
    categories: Array.isArray(raw.categories) ? raw.categories.map(normalizeCategory) : [],
  };
}

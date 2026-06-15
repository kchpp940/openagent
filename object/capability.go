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

package object

import (
	"fmt"
	"time"
)

type CapabilityStatus string

const (
	CapabilityStatusPassed  CapabilityStatus = "passed"
	CapabilityStatusFailed  CapabilityStatus = "failed"
	CapabilityStatusWarning CapabilityStatus = "warning"
	CapabilityStatusSkipped CapabilityStatus = "skipped"
	CapabilityStatusPending CapabilityStatus = "pending"
)

type CapabilityCheckItem struct {
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Status      CapabilityStatus `json:"status"`
	Message     string           `json:"message"`
	FixSuggestion  string        `json:"fixSuggestion"`
	FixCommand   string          `json:"fixCommand"`
	Details     interface{}      `json:"details,omitempty"`
}

type CapabilityCheckResult struct {
	OverallStatus CapabilityStatus      `json:"overallStatus"`
	TotalChecks   int                   `json:"totalChecks"`
	PassedChecks  int                   `json:"passedChecks"`
	FailedChecks  int                   `json:"failedChecks"`
	WarningChecks int                   `json:"warningChecks"`
	SkippedChecks int                   `json:"skippedChecks"`
	Checks        []*CapabilityCheckItem `json:"checks"`
}

func NewCapabilityCheckResult() *CapabilityCheckResult {
	return &CapabilityCheckResult{
		OverallStatus: CapabilityStatusPassed,
		Checks:        []*CapabilityCheckItem{},
	}
}

func (r *CapabilityCheckResult) AddCheck(item *CapabilityCheckItem) {
	r.Checks = append(r.Checks, item)
	r.TotalChecks++

	switch item.Status {
	case CapabilityStatusPassed:
		r.PassedChecks++
	case CapabilityStatusFailed:
		r.FailedChecks++
		r.OverallStatus = CapabilityStatusFailed
	case CapabilityStatusWarning:
		r.WarningChecks++
		if r.OverallStatus == CapabilityStatusPassed {
			r.OverallStatus = CapabilityStatusWarning
		}
	case CapabilityStatusSkipped:
		r.SkippedChecks++
	}
}

func NewCheckItem(name, description string, status CapabilityStatus, message string, fixSuggestion ...string) *CapabilityCheckItem {
	item := &CapabilityCheckItem{
		Name:        name,
		Description: description,
		Status:      status,
		Message:     message,
	}
	if len(fixSuggestion) > 0 {
		item.FixSuggestion = fixSuggestion[0]
	}
	return item
}

func PassedCheck(name, description, message string) *CapabilityCheckItem {
	return NewCheckItem(name, description, CapabilityStatusPassed, message)
}

func FailedCheck(name, description, message, fixSuggestion string, fixCommand ...string) *CapabilityCheckItem {
	item := NewCheckItem(name, description, CapabilityStatusFailed, message, fixSuggestion)
	if len(fixCommand) > 0 {
		item.FixCommand = fixCommand[0]
	}
	return item
}

func WarningCheck(name, description, message, fixSuggestion string, fixCommand ...string) *CapabilityCheckItem {
	item := NewCheckItem(name, description, CapabilityStatusWarning, message, fixSuggestion)
	if len(fixCommand) > 0 {
		item.FixCommand = fixCommand[0]
	}
	return item
}

func SkippedCheck(name, description, message string) *CapabilityCheckItem {
	return NewCheckItem(name, description, CapabilityStatusSkipped, message)
}

func StatusToColor(status CapabilityStatus) string {
	switch status {
	case CapabilityStatusPassed:
		return "green"
	case CapabilityStatusFailed:
		return "red"
	case CapabilityStatusWarning:
		return "orange"
	case CapabilityStatusSkipped:
		return "default"
	default:
		return "default"
	}
}

func (s CapabilityStatus) String() string {
	return string(s)
}

func FormatCheckSummary(result *CapabilityCheckResult) string {
	return fmt.Sprintf("%d/%d passed, %d failed, %d warnings, %d skipped",
		result.PassedChecks, result.TotalChecks,
		result.FailedChecks, result.WarningChecks, result.SkippedChecks)
}

func UpdateServerCapabilityStatus(server *Server, result *CapabilityCheckResult) error {
	if server == nil {
		return nil
	}
	now := time.Now().Format(time.RFC3339)
	server.LatestCapabilityStatus = string(result.OverallStatus)
	server.LatestCheckedAt = now
	_, err := adapter.engine.ID(server.GetId()).Cols("latest_capability_status", "latest_checked_at").Update(server)
	return err
}

func UpdateSkillCapabilityStatus(skill *Skill, result *CapabilityCheckResult) error {
	if skill == nil {
		return nil
	}
	now := time.Now().Format(time.RFC3339)
	skill.LatestCapabilityStatus = string(result.OverallStatus)
	skill.LatestCheckedAt = now
	_, err := adapter.engine.ID(skill.GetId()).Cols("latest_capability_status", "latest_checked_at").Update(skill)
	return err
}

func UpdateToolCapabilityStatus(tool *Tool, result *CapabilityCheckResult) error {
	if tool == nil {
		return nil
	}
	now := time.Now().Format(time.RFC3339)
	tool.LatestCapabilityStatus = string(result.OverallStatus)
	tool.LatestCheckedAt = now
	_, err := adapter.engine.ID(tool.GetId()).Cols("latest_capability_status", "latest_checked_at").Update(tool)
	return err
}

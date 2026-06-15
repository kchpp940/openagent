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
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
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

type CapabilityCheckRecord struct {
	Id           string                   `xorm:"varchar(100) notnull pk" json:"id"`
	Owner        string                   `xorm:"varchar(100) notnull index" json:"owner"`
	EntityType   string                   `xorm:"varchar(50) notnull index" json:"entityType"`
	EntityId     string                   `xorm:"varchar(200) notnull index" json:"entityId"`
	ConfigHash   string                   `xorm:"varchar(100) notnull index" json:"configHash"`
	Status       string                   `xorm:"varchar(50) notnull" json:"status"`
	CheckResult  *CapabilityCheckResult   `xorm:"mediumtext" json:"checkResult"`
	CheckedAt    string                   `xorm:"varchar(100) notnull" json:"checkedAt"`
	Error        string                   `xorm:"text" json:"error,omitempty"`
}

func (r *CapabilityCheckRecord) GetId() string {
	return r.Id
}

func CalculateConfigHash(entity interface{}) string {
	data, err := json.Marshal(entity)
	if err != nil {
		return ""
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return ""
	}

	delete(raw, "latestCapabilityStatus")
	delete(raw, "latestCheckedAt")
	delete(raw, "createdTime")
	delete(raw, "updatedTime")

	keys := make([]string, 0, len(raw))
	for k := range raw {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var buf strings.Builder
	for _, k := range keys {
		buf.WriteString(k)
		buf.WriteString(":")
		v, _ := json.Marshal(raw[k])
		buf.Write(v)
		buf.WriteString(";")
	}

	hash := sha256.Sum256([]byte(buf.String()))
	return hex.EncodeToString(hash[:])
}

func SaveCapabilityCheckRecord(owner, entityType, entityId string, result *CapabilityCheckResult, configHash string, checkErr ...error) (*CapabilityCheckRecord, error) {
	now := time.Now().Format(time.RFC3339)
	id := fmt.Sprintf("%s/%s/%s/%d", owner, entityType, entityId, time.Now().UnixNano())

	record := &CapabilityCheckRecord{
		Id:          id,
		Owner:       owner,
		EntityType:  entityType,
		EntityId:    entityId,
		ConfigHash:  configHash,
		Status:      string(result.OverallStatus),
		CheckResult: result,
		CheckedAt:   now,
	}

	if len(checkErr) > 0 && checkErr[0] != nil {
		record.Error = checkErr[0].Error()
	}

	_, err := adapter.engine.Insert(record)
	if err != nil {
		return nil, err
	}

	return record, nil
}

func GetLatestCapabilityCheckRecord(owner, entityType, entityId string) (*CapabilityCheckRecord, error) {
	var records []*CapabilityCheckRecord
	err := adapter.engine.Where("owner = ? AND entity_type = ? AND entity_id = ?", owner, entityType, entityId).
		Desc("checked_at").
		Limit(1).
		Find(&records)
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, nil
	}
	return records[0], nil
}

func GetLatestCapabilityCheckRecordByHash(owner, entityType, entityId, configHash string) (*CapabilityCheckRecord, error) {
	var records []*CapabilityCheckRecord
	err := adapter.engine.Where("owner = ? AND entity_type = ? AND entity_id = ? AND config_hash = ?", owner, entityType, entityId, configHash).
		Desc("checked_at").
		Limit(1).
		Find(&records)
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, nil
	}
	return records[0], nil
}

func GetCapabilityCheckRecords(owner, entityType, entityId string, limit int) ([]*CapabilityCheckRecord, error) {
	if limit <= 0 {
		limit = 20
	}
	var records []*CapabilityCheckRecord
	err := adapter.engine.Where("owner = ? AND entity_type = ? AND entity_id = ?", owner, entityType, entityId).
		Desc("checked_at").
		Limit(limit).
		Find(&records)
	return records, err
}

func UpdateEntityStatusFromRecord(entity interface{}, record *CapabilityCheckRecord) {
	if record == nil {
		return
	}
	switch e := entity.(type) {
	case *Server:
		e.LatestCapabilityStatus = record.Status
		e.LatestCheckedAt = record.CheckedAt
	case *Skill:
		e.LatestCapabilityStatus = record.Status
		e.LatestCheckedAt = record.CheckedAt
	case *Tool:
		e.LatestCapabilityStatus = record.Status
		e.LatestCheckedAt = record.CheckedAt
	}
}

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

func UpdateServerCapabilityStatus(server *Server, result *CapabilityCheckResult, configHash ...string) error {
	if server == nil {
		return nil
	}
	now := time.Now().Format(time.RFC3339)
	server.LatestCapabilityStatus = string(result.OverallStatus)
	server.LatestCheckedAt = now
	if len(configHash) > 0 {
		server.LatestCapabilityConfigHash = configHash[0]
	}
	_, err := adapter.engine.ID(server.GetId()).Cols("latest_capability_status", "latest_checked_at", "latest_capability_config_hash").Update(server)
	return err
}

func UpdateSkillCapabilityStatus(skill *Skill, result *CapabilityCheckResult, configHash ...string) error {
	if skill == nil {
		return nil
	}
	now := time.Now().Format(time.RFC3339)
	skill.LatestCapabilityStatus = string(result.OverallStatus)
	skill.LatestCheckedAt = now
	if len(configHash) > 0 {
		skill.LatestCapabilityConfigHash = configHash[0]
	}
	_, err := adapter.engine.ID(skill.GetId()).Cols("latest_capability_status", "latest_checked_at", "latest_capability_config_hash").Update(skill)
	return err
}

func UpdateToolCapabilityStatus(tool *Tool, result *CapabilityCheckResult, configHash ...string) error {
	if tool == nil {
		return nil
	}
	now := time.Now().Format(time.RFC3339)
	tool.LatestCapabilityStatus = string(result.OverallStatus)
	tool.LatestCheckedAt = now
	if len(configHash) > 0 {
		tool.LatestCapabilityConfigHash = configHash[0]
	}
	_, err := adapter.engine.ID(tool.GetId()).Cols("latest_capability_status", "latest_checked_at", "latest_capability_config_hash").Update(tool)
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
		e.LatestCapabilityConfigHash = record.ConfigHash
	case *Skill:
		e.LatestCapabilityStatus = record.Status
		e.LatestCheckedAt = record.CheckedAt
		e.LatestCapabilityConfigHash = record.ConfigHash
	case *Tool:
		e.LatestCapabilityStatus = record.Status
		e.LatestCheckedAt = record.CheckedAt
		e.LatestCapabilityConfigHash = record.ConfigHash
	}
}

type EntityCapabilityValidity string

const (
	EntityCapabilityValid    EntityCapabilityValidity = "valid"
	EntityCapabilityInvalid  EntityCapabilityValidity = "invalid"
	EntityCapabilityStale    EntityCapabilityValidity = "stale"
	EntityCapabilityPending  EntityCapabilityValidity = "pending"
	EntityCapabilityUnknown  EntityCapabilityValidity = "unknown"
)

type EntityCapabilityValidation struct {
	EntityType         string                     `json:"entityType"`
	EntityId           string                     `json:"entityId"`
	EntityName         string                     `json:"entityName"`
	Validity           EntityCapabilityValidity   `json:"validity"`
	Status             string                     `json:"status"`
	CurrentConfigHash  string                     `json:"currentConfigHash"`
	LatestRecordHash   string                     `json:"latestRecordHash,omitempty"`
	LatestRecord       *CapabilityCheckRecord     `json:"latestRecord,omitempty"`
	MatchingRecord     *CapabilityCheckRecord     `json:"matchingRecord,omitempty"`
	FailedChecks       []*CapabilityCheckItem     `json:"failedChecks,omitempty"`
	WarningChecks      []*CapabilityCheckItem     `json:"warningChecks,omitempty"`
	Message            string                     `json:"message"`
}

func (v *EntityCapabilityValidation) IsBlocked() bool {
	return v.Validity == EntityCapabilityInvalid ||
		v.Validity == EntityCapabilityStale ||
		v.Validity == EntityCapabilityPending ||
		v.Validity == EntityCapabilityUnknown
}

func ValidateEntityCapability(owner, entityType, entityId string, entity interface{}) (*EntityCapabilityValidation, error) {
	result := &EntityCapabilityValidation{
		EntityType: entityType,
		EntityId:   entityId,
		Validity:   EntityCapabilityUnknown,
		Message:    "Capability has not been checked yet",
	}

	if entity == nil {
		result.Message = "Entity is nil"
		return result, nil
	}

	currentHash := CalculateConfigHash(entity)
	result.CurrentConfigHash = currentHash

	switch e := entity.(type) {
	case *Server:
		result.EntityName = e.Name
	case *Skill:
		result.EntityName = e.Name
	case *Tool:
		result.EntityName = e.Name
	}

	latestRecord, err := GetLatestCapabilityCheckRecord(owner, entityType, entityId)
	if err != nil {
		return result, fmt.Errorf("failed to get latest check record: %v", err)
	}
	result.LatestRecord = latestRecord

	if latestRecord == nil {
		result.Validity = EntityCapabilityUnknown
		result.Message = "No capability check records found. Please run a capability check first."
		return result, nil
	}

	result.LatestRecordHash = latestRecord.ConfigHash
	result.Status = latestRecord.Status

	if latestRecord.Status == string(CapabilityStatusPending) {
		result.Validity = EntityCapabilityPending
		result.Message = "Capability check is in progress. Please wait for it to complete."
		return result, nil
	}

	if latestRecord.ConfigHash != currentHash {
		result.Validity = EntityCapabilityStale
		result.Message = fmt.Sprintf("Configuration has changed since last check (last checked at %s). Please re-run capability check.", latestRecord.CheckedAt)
		if latestRecord.CheckResult != nil {
			for _, c := range latestRecord.CheckResult.Checks {
				if c.Status == CapabilityStatusFailed {
					result.FailedChecks = append(result.FailedChecks, c)
				} else if c.Status == CapabilityStatusWarning {
					result.WarningChecks = append(result.WarningChecks, c)
				}
			}
		}
		return result, nil
	}

	matchingRecord, err := GetLatestCapabilityCheckRecordByHash(owner, entityType, entityId, currentHash)
	if err != nil {
		return result, fmt.Errorf("failed to get matching check record: %v", err)
	}
	result.MatchingRecord = matchingRecord

	if matchingRecord == nil {
		result.Validity = EntityCapabilityStale
		result.Message = "No matching check record for current configuration. Please re-run capability check."
		return result, nil
	}

	switch matchingRecord.Status {
	case string(CapabilityStatusPassed):
		result.Validity = EntityCapabilityValid
		result.Message = "All capability checks passed for current configuration"
	case string(CapabilityStatusWarning):
		result.Validity = EntityCapabilityValid
		result.Message = "Capability checks passed with warnings for current configuration"
		if matchingRecord.CheckResult != nil {
			for _, c := range matchingRecord.CheckResult.Checks {
				if c.Status == CapabilityStatusWarning {
					result.WarningChecks = append(result.WarningChecks, c)
				}
			}
		}
	case string(CapabilityStatusFailed):
		result.Validity = EntityCapabilityInvalid
		result.Message = "Capability checks failed for current configuration"
		if matchingRecord.CheckResult != nil {
			for _, c := range matchingRecord.CheckResult.Checks {
				if c.Status == CapabilityStatusFailed {
					result.FailedChecks = append(result.FailedChecks, c)
				} else if c.Status == CapabilityStatusWarning {
					result.WarningChecks = append(result.WarningChecks, c)
				}
			}
		}
	default:
		result.Validity = EntityCapabilityUnknown
		result.Message = fmt.Sprintf("Unknown capability status: %s", matchingRecord.Status)
	}

	return result, nil
}

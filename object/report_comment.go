// Copyright 2024 The OpenAgent Authors. All Rights Reserved.
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
	"crypto/rand"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"unicode"

	"github.com/the-open-agent/openagent/util"
	"xorm.io/core"
)

const (
	ReportCommentStatusOpen     = "open"
	ReportCommentStatusResolved = "resolved"
	ReportCommentStatusDisputed = "disputed"
)

const (
	AnchorTypeCategory = "category"
	AnchorTypeItem     = "item"
	AnchorTypeField    = "field"
)

const (
	MatchTypeExact     = "exact"
	MatchTypeFuzzy     = "fuzzy"
	MatchTypeAmbiguous = "ambiguous"
	MatchTypeOrphan    = "orphan"
	MatchTypeManual    = "manual"
)

type ReportComment struct {
	Id                int64  `xorm:"pk autoincr" json:"id"`
	Owner             string `xorm:"varchar(100) notnull index" json:"owner"`
	TaskOwner         string `xorm:"varchar(100) notnull index" json:"taskOwner"`
	TaskName          string `xorm:"varchar(100) notnull index" json:"taskName"`

	AnchorId       string `xorm:"varchar(64) index" json:"anchorId"`
	ItemAnchorId   string `xorm:"varchar(64) index" json:"itemAnchorId"`
	AnchorCategory string `xorm:"varchar(100) index" json:"anchorCategory"`
	AnchorItem     string `xorm:"varchar(200) index" json:"anchorItem"`
	AnchorField    string `xorm:"varchar(300) index" json:"anchorField"`
	ContentHash    string `xorm:"varchar(24) index" json:"contentHash"`

	StableCategoryKey string `xorm:"varchar(200) index" json:"-"`
	StableItemKey     string `xorm:"varchar(200) index" json:"-"`

	CategoryName      string `xorm:"varchar(200)" json:"categoryName"`
	ItemName          string `xorm:"varchar(200)" json:"itemName"`
	ReferencedField   string `xorm:"varchar(50)" json:"referencedField"`
	ReferencedContent string `xorm:"mediumtext" json:"referencedContent"`

	Content      string `xorm:"mediumtext" json:"content"`
	Status       string `xorm:"varchar(20) notnull default 'open'" json:"status"`
	Author       string `xorm:"varchar(100) notnull" json:"author"`
	Resolver     string `xorm:"varchar(100)" json:"resolver"`
	CreatedTime  string `xorm:"varchar(100)" json:"createdTime"`
	UpdatedTime  string `xorm:"varchar(100)" json:"updatedTime"`
	ResolvedTime string `xorm:"varchar(100)" json:"resolvedTime"`
}

type TaskAnchor struct {
	AnchorId          string `json:"anchorId"`
	ItemAnchorId      string `json:"itemAnchorId"`
	AnchorType        string `json:"anchorType"`
	AnchorField       string `json:"anchorField"`
	AnchorItem        string `json:"anchorItem"`
	AnchorCategory    string `json:"anchorCategory"`
	ContentHash       string `json:"contentHash"`
	StableCategoryKey string `json:"stableCategoryKey"`
	StableItemKey     string `json:"stableItemKey"`
	CategoryName      string `json:"categoryName"`
	ItemName          string `json:"itemName"`
	ReferencedField   string `json:"referencedField"`
	ReferencedContent string `json:"referencedContent"`
	CategoryIndex     int    `json:"categoryIndex"`
	ItemIndex         int    `json:"itemIndex"`
}

type TaskAnchorsResult struct {
	Anchors []*TaskAnchor `json:"anchors"`
}

type TaskAnchorSnapshot struct {
	Id            int64  `xorm:"pk autoincr" json:"id"`
	SnapshotId    string `xorm:"varchar(32) notnull unique" json:"snapshotId"`
	TaskOwner     string `xorm:"varchar(100) notnull index" json:"taskOwner"`
	TaskName      string `xorm:"varchar(100) notnull index" json:"taskName"`
	ReportVersion int    `xorm:"bigint notnull" json:"reportVersion"`
	IsLatest      bool   `xorm:"bool notnull default false index" json:"isLatest"`
	ResultHash    string `xorm:"varchar(64) notnull index" json:"resultHash"`
	CreatedTime   string `xorm:"varchar(100)" json:"createdTime"`
}

type TaskAnchorRecord struct {
	Id                int64  `xorm:"pk autoincr" json:"id"`
	SnapshotId        string `xorm:"varchar(32) notnull index unique(snapshot_anchor)" json:"snapshotId"`
	AnchorId          string `xorm:"varchar(64) notnull unique(snapshot_anchor)" json:"anchorId"`
	ItemAnchorId      string `xorm:"varchar(64) index" json:"itemAnchorId"`
	AnchorType        string `xorm:"varchar(20)" json:"anchorType"`
	AnchorField       string `xorm:"varchar(300)" json:"anchorField"`
	AnchorItem        string `xorm:"varchar(200)" json:"anchorItem"`
	AnchorCategory    string `xorm:"varchar(100)" json:"anchorCategory"`
	ContentHash       string `xorm:"varchar(24)" json:"contentHash"`
	StableCategoryKey string `xorm:"varchar(200)" json:"stableCategoryKey"`
	StableItemKey     string `xorm:"varchar(200)" json:"stableItemKey"`
	CategoryName      string `xorm:"varchar(200)" json:"categoryName"`
	ItemName          string `xorm:"varchar(200)" json:"itemName"`
	ReferencedField   string `xorm:"varchar(50)" json:"referencedField"`
	ReferencedContent string `xorm:"mediumtext" json:"referencedContent"`
	CategoryIndex     int    `json:"categoryIndex"`
	ItemIndex         int    `json:"itemIndex"`
}

type TaskAnchorMigration struct {
	Id              int64  `xorm:"pk autoincr" json:"id"`
	TaskOwner       string `xorm:"varchar(100) notnull index" json:"taskOwner"`
	TaskName        string `xorm:"varchar(100) notnull index" json:"taskName"`
	FromSnapshotId  string `xorm:"varchar(32) notnull index" json:"fromSnapshotId"`
	ToSnapshotId    string `xorm:"varchar(32) notnull index" json:"toSnapshotId"`
	FromAnchorId    string `xorm:"varchar(64) notnull index" json:"fromAnchorId"`
	ToAnchorId      string `xorm:"varchar(64) index" json:"toAnchorId"`
	MatchType       string `xorm:"varchar(20) notnull" json:"matchType"`
	MatchScore      int    `json:"matchScore"`
	IsAmbiguous     bool   `xorm:"bool notnull default false" json:"isAmbiguous"`
	CandidatesJson  string `xorm:"mediumtext" json:"candidatesJson"`
	Applied         bool   `xorm:"bool notnull default false index" json:"applied"`
	ManualOverride  bool   `xorm:"bool notnull default false" json:"manualOverride"`
	ManuallySetBy   string `xorm:"varchar(100)" json:"manuallySetBy"`
	CreatedTime     string `xorm:"varchar(100)" json:"createdTime"`
	UpdatedTime     string `xorm:"varchar(100)" json:"updatedTime"`
}

type BuildTaskResultAnchorsV2Result struct {
	SnapshotId     string              `json:"snapshotId"`
	ReportVersion  int                 `json:"reportVersion"`
	IsNewSnapshot  bool                `json:"isNewSnapshot"`
	Anchors        []*TaskAnchor       `json:"anchors"`
	Records        []*TaskAnchorRecord `json:"-"`
	ResultHash     string              `json:"resultHash"`
	OldSnapshotId  string              `json:"oldSnapshotId,omitempty"`
}

type AnchorMigrationCandidate struct {
	AnchorId  string `json:"anchorId"`
	Score     int    `json:"score"`
}

type AmbiguousAnchorInfo struct {
	CommentIds   []int64                      `json:"commentIds"`
	FromAnchorId string                       `json:"fromAnchorId"`
	Candidates   []*AnchorMigrationCandidate  `json:"candidates"`
}

type ApplyCommentMigrationResult struct {
	UpdatedCount     int      `json:"updatedCount"`
	SkippedOrphan    int      `json:"skippedOrphan"`
	SkippedAmbiguous int      `json:"skippedAmbiguous"`
	SkippedNoChange  int      `json:"skippedNoChange"`
	AppliedIds       []string `json:"-"`
}

type AnchorMigrationResult struct {
	Migration      map[string]*TaskAnchorMigration          `json:"migration"`
	OrphanFromIds  []string                                 `json:"orphanFromIds"`
	AmbiguousMap   map[string][]*AnchorMigrationCandidate   `json:"ambiguousMap"`
}

type MigrationCandidate struct {
	AnchorId  string `json:"anchorId"`
	Score     int    `json:"score"`
}

type AmbiguousInfo struct {
	CommentId     int64               `json:"commentId"`
	FromAnchorId  string              `json:"fromAnchorId"`
	Candidates    []MigrationCandidate `json:"candidates"`
	BestScore     int                 `json:"bestScore"`
	SecondScore   int                 `json:"secondScore"`
}

type GroupedCommentsResult struct {
	Grouped       map[string][]*ReportComment  `json:"grouped"`
	OrphanCount   int                          `json:"orphanCount"`
	Orphans       []*ReportComment             `json:"orphans"`
	AmbiguousInfo []*AmbiguousAnchorInfo       `json:"ambiguousInfo"`
	LatestSnapId  string                       `json:"snapshotId"`
	ReportVersion int                          `json:"reportVersion"`
	Migration     map[string]string            `json:"migration"`
}

func randomHexId(n int) string {
	b := make([]byte, n)
	_, err := rand.Read(b)
	if err != nil {
		h := sha1.Sum([]byte(util.GetCurrentTime()))
		return hex.EncodeToString(h[:])[:n*2]
	}
	return hex.EncodeToString(b)
}

func normalizeAnchorToken(s string) string {
	var b strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(unicode.ToLower(r))
		} else if r == ' ' || r == '-' || r == '_' || r == '/' || r == '.' {
			if b.Len() > 0 && b.String()[b.Len()-1] != '_' {
				b.WriteRune('_')
			}
		}
	}
	res := strings.TrimRight(b.String(), "_")
	if len(res) > 80 {
		h := sha1.Sum([]byte(res))
		res = res[:60] + hex.EncodeToString(h[:])[:12]
	}
	return res
}

func shortContentHash(content string) string {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return ""
	}
	runes := []rune(trimmed)
	if len(runes) > 500 {
		trimmed = string(runes[:500])
	}
	h := sha1.Sum([]byte(trimmed))
	return hex.EncodeToString(h[:])[:16]
}

func GenerateCategoryAnchor(categoryName string) string {
	return "C:" + normalizeAnchorToken(categoryName)
}

func GenerateItemAnchor(categoryName string, itemName string) string {
	c := normalizeAnchorToken(categoryName)
	i := normalizeAnchorToken(itemName)
	return "I:" + c + "||" + i
}

func GenerateFieldAnchor(categoryName string, itemName string, field string, content string) string {
	itemAnchor := GenerateItemAnchor(categoryName, itemName)
	h := shortContentHash(content)
	if h != "" {
		return "F:" + itemAnchor + "::" + field + "#" + h
	}
	return "F:" + itemAnchor + "::" + field
}

func shortIdFromLong(s string) string {
	if s == "" {
		return ""
	}
	h := sha1.Sum([]byte(s))
	return hex.EncodeToString(h[:])[:12]
}

func GenerateAnchorId(anchorType string, anchorCategory string, anchorItem string, anchorField string) string {
	switch anchorType {
	case AnchorTypeCategory:
		return "A:C:" + shortIdFromLong(anchorCategory)
	case AnchorTypeItem:
		return "A:I:" + shortIdFromLong(anchorItem)
	case AnchorTypeField:
		return "A:F:" + shortIdFromLong(anchorField)
	default:
		return "A:X:" + shortIdFromLong(anchorCategory+"||"+anchorItem+"::"+anchorField)
	}
}

func GenerateRandomAnchorId(anchorType string) string {
	randomPart := randomHexId(12)
	switch anchorType {
	case AnchorTypeCategory:
		return "A:C:" + randomPart
	case AnchorTypeItem:
		return "A:I:" + randomPart
	case AnchorTypeField:
		return "A:F:" + randomPart
	default:
		return "A:X:" + randomPart
	}
}

func GenerateItemAnchorIdFromNames(categoryName string, itemName string) string {
	return "A:I:" + shortIdFromLong(GenerateItemAnchor(categoryName, itemName))
}

func (r *ReportComment) FillAnchorsFromTask() {
	if r.CategoryName != "" && r.AnchorCategory == "" {
		r.AnchorCategory = GenerateCategoryAnchor(r.CategoryName)
	}
	if r.CategoryName != "" && r.ItemName != "" && r.AnchorItem == "" {
		r.AnchorItem = GenerateItemAnchor(r.CategoryName, r.ItemName)
	}
	if r.CategoryName != "" && r.ItemName != "" && r.ReferencedField != "" && r.AnchorField == "" {
		r.AnchorField = GenerateFieldAnchor(r.CategoryName, r.ItemName, r.ReferencedField, r.ReferencedContent)
	}
	if r.ReferencedContent != "" && r.ContentHash == "" {
		r.ContentHash = shortContentHash(r.ReferencedContent)
	}
	if r.StableCategoryKey == "" && r.CategoryName != "" {
		r.StableCategoryKey = GenerateStableCategoryKey(r.CategoryName, 0)
	}
	if r.StableItemKey == "" && r.CategoryName != "" && r.ItemName != "" {
		r.StableItemKey = GenerateStableItemKey(r.CategoryName, r.ItemName, 0, 0)
	}
	if r.ItemAnchorId == "" && r.AnchorItem != "" {
		r.ItemAnchorId = GenerateAnchorId(AnchorTypeItem, r.AnchorCategory, r.AnchorItem, "")
	} else if r.ItemAnchorId == "" && r.CategoryName != "" && r.ItemName != "" {
		r.ItemAnchorId = GenerateItemAnchorIdFromNames(r.CategoryName, r.ItemName)
	}
	if r.AnchorId == "" && r.AnchorField != "" {
		r.AnchorId = GenerateAnchorId(AnchorTypeField, r.AnchorCategory, r.AnchorItem, r.AnchorField)
	} else if r.AnchorId == "" && r.AnchorItem != "" {
		r.AnchorId = GenerateAnchorId(AnchorTypeItem, r.AnchorCategory, r.AnchorItem, "")
	} else if r.AnchorId == "" && r.AnchorCategory != "" {
		r.AnchorId = GenerateAnchorId(AnchorTypeCategory, r.AnchorCategory, "", "")
	}
}

func BuildTaskResultAnchors(result *TaskResult) *TaskAnchorsResult {
	res := &TaskAnchorsResult{Anchors: []*TaskAnchor{}}
	if result == nil {
		return res
	}
	for ci, cat := range result.Categories {
		if cat == nil {
			continue
		}
		catAnchor := GenerateCategoryAnchor(cat.Name)
		stableCatKey := GenerateStableCategoryKey(cat.Name, ci)
		catAnchorId := GenerateAnchorId(AnchorTypeCategory, catAnchor, "", "")
		res.Anchors = append(res.Anchors, &TaskAnchor{
			AnchorId:          catAnchorId,
			ItemAnchorId:      "",
			AnchorType:        AnchorTypeCategory,
			AnchorCategory:    catAnchor,
			StableCategoryKey: stableCatKey,
			CategoryName:      cat.Name,
			CategoryIndex:     ci,
		})
		for ii, item := range cat.Items {
			if item == nil {
				continue
			}
			itemAnchor := GenerateItemAnchor(cat.Name, item.Name)
			stableItemKey := GenerateStableItemKey(cat.Name, item.Name, ci, ii)
			itemAnchorId := GenerateAnchorId(AnchorTypeItem, catAnchor, itemAnchor, "")

			addField := func(field string, content interface{}) {
				contentStr := ""
				switch v := content.(type) {
				case string:
					contentStr = v
				case int:
					contentStr = fmt.Sprintf("%d", v)
				case float64:
					contentStr = fmt.Sprintf("%.4f", v)
				case float32:
					contentStr = fmt.Sprintf("%.4f", v)
				}
				fieldAnchor := GenerateFieldAnchor(cat.Name, item.Name, field, contentStr)
				ch := shortContentHash(contentStr)
				fieldAnchorId := GenerateAnchorId(AnchorTypeField, catAnchor, itemAnchor, fieldAnchor)
				res.Anchors = append(res.Anchors, &TaskAnchor{
					AnchorId:          fieldAnchorId,
					ItemAnchorId:      itemAnchorId,
					AnchorType:        AnchorTypeField,
					AnchorCategory:    catAnchor,
					AnchorItem:        itemAnchor,
					AnchorField:       fieldAnchor,
					ContentHash:       ch,
					StableCategoryKey: stableCatKey,
					StableItemKey:     stableItemKey,
					CategoryName:      cat.Name,
					ItemName:          item.Name,
					ReferencedField:   field,
					ReferencedContent: contentStr,
					CategoryIndex:     ci,
					ItemIndex:         ii,
				})
			}

			res.Anchors = append(res.Anchors, &TaskAnchor{
				AnchorId:          itemAnchorId,
				ItemAnchorId:      itemAnchorId,
				AnchorType:        AnchorTypeItem,
				AnchorCategory:    catAnchor,
				AnchorItem:        itemAnchor,
				StableCategoryKey: stableCatKey,
				StableItemKey:     stableItemKey,
				CategoryName:      cat.Name,
				ItemName:          item.Name,
				CategoryIndex:     ci,
				ItemIndex:         ii,
			})

			addField("score", item.Score)
			addField("advantage", item.Advantage)
			addField("disadvantage", item.Disadvantage)
			addField("suggestion", item.Suggestion)
		}
	}
	return res
}

func computeResultHash(taskOwner, taskName string, result *TaskResult) string {
	h := sha1.New()
	h.Write([]byte(taskOwner))
	h.Write([]byte("||"))
	h.Write([]byte(taskName))
	h.Write([]byte("||"))
	if result != nil {
		jsonBytes, _ := json.Marshal(result)
		h.Write(jsonBytes)
	}
	return hex.EncodeToString(h.Sum(nil))
}

func GetLatestAnchorSnapshot(taskOwner, taskName string) (*TaskAnchorSnapshot, error) {
	snapshot := &TaskAnchorSnapshot{}
	existed, err := adapter.engine.Where("task_owner = ? AND task_name = ? AND is_latest = ?", taskOwner, taskName, true).Desc("report_version").Get(snapshot)
	if err != nil {
		return nil, err
	}
	if !existed {
		return nil, nil
	}
	return snapshot, nil
}

func GetAnchorSnapshot(snapshotId string) (*TaskAnchorSnapshot, error) {
	snapshot := &TaskAnchorSnapshot{}
	existed, err := adapter.engine.Where("snapshot_id = ?", snapshotId).Get(snapshot)
	if err != nil {
		return nil, err
	}
	if !existed {
		return nil, nil
	}
	return snapshot, nil
}

func GetAnchorRecordsBySnapshot(snapshotId string) ([]*TaskAnchorRecord, error) {
	records := []*TaskAnchorRecord{}
	err := adapter.engine.Where("snapshot_id = ?", snapshotId).Find(&records)
	if err != nil {
		return records, err
	}
	return records, nil
}

func GetAnchorRecordsAsTaskAnchors(records []*TaskAnchorRecord) []*TaskAnchor {
	anchors := make([]*TaskAnchor, 0, len(records))
	for _, r := range records {
		anchors = append(anchors, &TaskAnchor{
			AnchorId:          r.AnchorId,
			ItemAnchorId:      r.ItemAnchorId,
			AnchorType:        r.AnchorType,
			AnchorField:       r.AnchorField,
			AnchorItem:        r.AnchorItem,
			AnchorCategory:    r.AnchorCategory,
			ContentHash:       r.ContentHash,
			StableCategoryKey: r.StableCategoryKey,
			StableItemKey:     r.StableItemKey,
			CategoryName:      r.CategoryName,
			ItemName:          r.ItemName,
			ReferencedField:   r.ReferencedField,
			ReferencedContent: r.ReferencedContent,
			CategoryIndex:     r.CategoryIndex,
			ItemIndex:         r.ItemIndex,
		})
	}
	return anchors
}

func GetPreviousSnapshot(taskOwner string, taskName string, reportVersion int) (*TaskAnchorSnapshot, error) {
	snapshot := &TaskAnchorSnapshot{}
	existed, err := adapter.engine.Where("task_owner = ? AND task_name = ? AND report_version = ?", taskOwner, taskName, reportVersion).Get(snapshot)
	if err != nil {
		return nil, err
	}
	if !existed {
		return nil, nil
	}
	return snapshot, nil
}

func taskAnchorToRecord(snapshotId string, a *TaskAnchor) *TaskAnchorRecord {
	return &TaskAnchorRecord{
		SnapshotId:        snapshotId,
		AnchorId:          a.AnchorId,
		ItemAnchorId:      a.ItemAnchorId,
		AnchorType:        a.AnchorType,
		AnchorField:       a.AnchorField,
		AnchorItem:        a.AnchorItem,
		AnchorCategory:    a.AnchorCategory,
		ContentHash:       a.ContentHash,
		StableCategoryKey: a.StableCategoryKey,
		StableItemKey:     a.StableItemKey,
		CategoryName:      a.CategoryName,
		ItemName:          a.ItemName,
		ReferencedField:   a.ReferencedField,
		ReferencedContent: a.ReferencedContent,
		CategoryIndex:     a.CategoryIndex,
		ItemIndex:         a.ItemIndex,
	}
}

func BuildTaskResultAnchorsV2(taskOwner, taskName string, result *TaskResult) (*BuildTaskResultAnchorsV2Result, error) {
	anchorsResult := BuildTaskResultAnchors(result)
	anchors := anchorsResult.Anchors
	resultHash := computeResultHash(taskOwner, taskName, result)

	existingSnap := &TaskAnchorSnapshot{}
	existed, err := adapter.engine.Where("task_owner = ? AND task_name = ? AND result_hash = ? AND is_latest = ?", taskOwner, taskName, resultHash, true).Get(existingSnap)
	if err != nil {
		return nil, err
	}
	if existed {
		existingRecords, errR := GetAnchorRecordsBySnapshot(existingSnap.SnapshotId)
		if errR != nil {
			return nil, errR
		}
		existingAnchors := GetAnchorRecordsAsTaskAnchors(existingRecords)
		return &BuildTaskResultAnchorsV2Result{
			SnapshotId:    existingSnap.SnapshotId,
			ReportVersion: existingSnap.ReportVersion,
			IsNewSnapshot: false,
			Anchors:       existingAnchors,
			Records:       existingRecords,
			ResultHash:    resultHash,
		}, nil
	}

	session := adapter.engine.NewSession()
	defer session.Close()

	if err = session.Begin(); err != nil {
		return nil, err
	}

	oldLatest := &TaskAnchorSnapshot{}
	oldExisted, err := session.Where("task_owner = ? AND task_name = ? AND is_latest = ?", taskOwner, taskName, true).Get(oldLatest)
	if err != nil {
		session.Rollback()
		return nil, err
	}

	var oldSnapshotId string
	newVersion := 1
	if oldExisted {
		oldSnapshotId = oldLatest.SnapshotId
		newVersion = oldLatest.ReportVersion + 1
		oldLatest.IsLatest = false
		_, err = session.ID(core.PK{oldLatest.Id}).Cols("is_latest").Update(oldLatest)
		if err != nil {
			session.Rollback()
			return nil, err
		}
	}

	newSnapshotId := randomHexId(12)
	now := util.GetCurrentTime()
	newSnap := &TaskAnchorSnapshot{
		SnapshotId:    newSnapshotId,
		TaskOwner:     taskOwner,
		TaskName:      taskName,
		ReportVersion: newVersion,
		IsLatest:      true,
		ResultHash:    resultHash,
		CreatedTime:   now,
	}
	_, err = session.Insert(newSnap)
	if err != nil {
		session.Rollback()
		return nil, err
	}

	newRecords := make([]*TaskAnchorRecord, 0, len(anchors))
	if len(anchors) > 0 {
		records := make([]interface{}, 0, len(anchors))
		for _, a := range anchors {
			rec := taskAnchorToRecord(newSnapshotId, a)
			newRecords = append(newRecords, rec)
			records = append(records, rec)
		}
		_, err = session.Insert(records...)
		if err != nil {
			session.Rollback()
			return nil, err
		}
	}

	if err = session.Commit(); err != nil {
		return nil, err
	}

	return &BuildTaskResultAnchorsV2Result{
		SnapshotId:    newSnapshotId,
		ReportVersion: newVersion,
		IsNewSnapshot: true,
		Anchors:       anchors,
		Records:       newRecords,
		ResultHash:    resultHash,
		OldSnapshotId: oldSnapshotId,
	}, nil
}

func FindAnchorById(anchors []*TaskAnchor, anchorId string) *TaskAnchor {
	anchorId = strings.TrimSpace(anchorId)
	if anchorId == "" {
		return nil
	}
	for _, a := range anchors {
		if a != nil && a.AnchorId == anchorId {
			return a
		}
	}
	return nil
}

func stringEqualCI(a, b string) bool {
	return strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b))
}

func computeAnchorMatchScore(oldA *TaskAnchor, newA *TaskAnchor) int {
	score := 0
	if oldA.AnchorType == newA.AnchorType {
		score += 2000
	}
	if oldA.AnchorCategory != "" && oldA.AnchorCategory == newA.AnchorCategory {
		score += 1000
	}
	if oldA.AnchorItem != "" && oldA.AnchorItem == newA.AnchorItem {
		score += 800
	}
	if oldA.ContentHash != "" && oldA.ContentHash == newA.ContentHash {
		score += 500
	}
	if oldA.CategoryName != "" && newA.CategoryName != "" && stringEqualCI(oldA.CategoryName, newA.CategoryName) {
		score += 400
	}
	if oldA.AnchorField != "" && oldA.AnchorField == newA.AnchorField {
		score += 300
	}
	if oldA.ReferencedField != "" && oldA.ReferencedField == newA.ReferencedField {
		score += 250
	}
	if oldA.ItemName != "" && newA.ItemName != "" && stringEqualCI(oldA.ItemName, newA.ItemName) {
		score += 250
	}
	return score
}

func ComputeAnchorMigration(taskOwner, taskName, fromId, toId string, fromRecs []*TaskAnchorRecord, toRecs []*TaskAnchorRecord) (*AnchorMigrationResult, error) {
	fromAnchors := GetAnchorRecordsAsTaskAnchors(fromRecs)
	toAnchors := GetAnchorRecordsAsTaskAnchors(toRecs)
	now := util.GetCurrentTime()

	result := &AnchorMigrationResult{
		Migration:     make(map[string]*TaskAnchorMigration),
		OrphanFromIds: []string{},
		AmbiguousMap:  make(map[string][]*AnchorMigrationCandidate),
	}

	for _, oldA := range fromAnchors {
		if oldA == nil || oldA.AnchorId == "" {
			continue
		}
		candidates := []*AnchorMigrationCandidate{}
		for _, newA := range toAnchors {
			if newA == nil || newA.AnchorId == "" {
				continue
			}
			s := computeAnchorMatchScore(oldA, newA)
			if s > 0 {
				candidates = append(candidates, &AnchorMigrationCandidate{
					AnchorId: newA.AnchorId,
					Score:    s,
				})
			}
		}

		sort.Slice(candidates, func(i, j int) bool {
			return candidates[i].Score > candidates[j].Score
		})

		matchType := MatchTypeOrphan
		toAnchorId := ""
		bestScore := 0
		secondScore := 0
		isAmbiguous := false

		if len(candidates) > 0 {
			bestScore = candidates[0].Score
			if bestScore < 500 {
				matchType = MatchTypeOrphan
			} else {
				if len(candidates) > 1 {
					secondScore = candidates[1].Score
					diffPct := 0.0
					if bestScore > 0 {
						diffPct = float64(bestScore-secondScore) / float64(bestScore)
					}
					if diffPct < 0.15 && bestScore < 400 {
						isAmbiguous = true
						matchType = MatchTypeAmbiguous
						result.AmbiguousMap[oldA.AnchorId] = candidates
					} else {
						if bestScore >= 4800 {
							matchType = MatchTypeExact
						} else {
							matchType = MatchTypeFuzzy
						}
						toAnchorId = candidates[0].AnchorId
					}
				} else {
					if bestScore >= 4800 {
						matchType = MatchTypeExact
					} else {
						matchType = MatchTypeFuzzy
					}
					toAnchorId = candidates[0].AnchorId
				}
			}
		}

		if matchType == MatchTypeOrphan {
			result.OrphanFromIds = append(result.OrphanFromIds, oldA.AnchorId)
		}

		legacyCands := make([]MigrationCandidate, 0, len(candidates))
		for _, c := range candidates {
			legacyCands = append(legacyCands, MigrationCandidate{AnchorId: c.AnchorId, Score: c.Score})
		}
		candidatesJson := ""
		if len(legacyCands) > 0 {
			jsonBytes, _ := json.Marshal(legacyCands)
			candidatesJson = string(jsonBytes)
		}

		mig := &TaskAnchorMigration{
			TaskOwner:      taskOwner,
			TaskName:       taskName,
			FromSnapshotId: fromId,
			ToSnapshotId:   toId,
			FromAnchorId:   oldA.AnchorId,
			ToAnchorId:     toAnchorId,
			MatchType:      matchType,
			MatchScore:     bestScore,
			IsAmbiguous:    isAmbiguous,
			CandidatesJson: candidatesJson,
			Applied:        false,
			ManualOverride: false,
			CreatedTime:    now,
			UpdatedTime:    now,
		}
		result.Migration[oldA.AnchorId] = mig
	}
	return result, nil
}

func PersistAnchorMigration(migs map[string]*TaskAnchorMigration) error {
	if len(migs) == 0 {
		return nil
	}
	now := util.GetCurrentTime()
	records := make([]interface{}, 0, len(migs))
	for _, m := range migs {
		if m.CreatedTime == "" {
			m.CreatedTime = now
		}
		if m.UpdatedTime == "" {
			m.UpdatedTime = now
		}
		records = append(records, m)
	}
	_, err := adapter.engine.Insert(records...)
	return err
}

func LoadPersistedMigration(fromId, toId string) (map[string]*TaskAnchorMigration, error) {
	migrations := []*TaskAnchorMigration{}
	err := adapter.engine.Where("from_snapshot_id = ? AND to_snapshot_id = ?", fromId, toId).Find(&migrations)
	if err != nil {
		return nil, err
	}
	result := make(map[string]*TaskAnchorMigration)
	for _, m := range migrations {
		result[m.FromAnchorId] = m
	}
	return result, nil
}

func LoadPersistedMigrationFull(taskOwner, taskName, fromSnapshotId, toSnapshotId string) ([]*TaskAnchorMigration, error) {
	migrations := []*TaskAnchorMigration{}
	err := adapter.engine.Where("task_owner = ? AND task_name = ? AND from_snapshot_id = ? AND to_snapshot_id = ?", taskOwner, taskName, fromSnapshotId, toSnapshotId).Find(&migrations)
	if err != nil {
		return nil, err
	}
	return migrations, nil
}

func ApplyCommentMigration(taskOwner, taskName, toSnapId string, migResult *AnchorMigrationResult, forceAmbiguous bool, operator string) (*ApplyCommentMigrationResult, error) {
	result := &ApplyCommentMigrationResult{
		UpdatedCount:     0,
		SkippedOrphan:    0,
		SkippedAmbiguous: 0,
		SkippedNoChange:  0,
		AppliedIds:       []string{},
	}

	if migResult == nil || len(migResult.Migration) == 0 {
		return result, nil
	}

	comments, err := GetReportCommentsByTask(taskOwner, taskName)
	if err != nil {
		return nil, err
	}

	if len(comments) == 0 {
		return result, nil
	}

	session := adapter.engine.NewSession()
	defer session.Close()

	if err = session.Begin(); err != nil {
		return nil, err
	}

	now := util.GetCurrentTime()
	appliedMigrationFromIds := make(map[string]bool)
	migrationMap := migResult.Migration

	for _, comment := range comments {
		updated := false

		processAnchorId := func(currentId string) (string, bool, bool) {
			if currentId == "" {
				return "", false, false
			}
			m, ok := migrationMap[currentId]
			if !ok {
				return currentId, false, false
			}
			if m.MatchType == MatchTypeOrphan {
				result.SkippedOrphan++
				return currentId, true, false
			}
			if m.MatchType == MatchTypeAmbiguous && !forceAmbiguous {
				result.SkippedAmbiguous++
				return currentId, true, false
			}
			if m.ToAnchorId == "" || m.ToAnchorId == currentId {
				if m.ToAnchorId == currentId {
					result.SkippedNoChange++
				}
				return currentId, true, false
			}
			if !m.Applied {
				appliedMigrationFromIds[m.FromAnchorId] = true
			}
			return m.ToAnchorId, true, true
		}

		if comment.AnchorId != "" {
			newId, processed, shouldUpdate := processAnchorId(comment.AnchorId)
			if processed && shouldUpdate {
				comment.AnchorId = newId
				updated = true
			}
		}

		if comment.ItemAnchorId != "" {
			newId, processed, shouldUpdate := processAnchorId(comment.ItemAnchorId)
			if processed && shouldUpdate {
				comment.ItemAnchorId = newId
				updated = true
			}
		}

		if updated {
			comment.UpdatedTime = now
			_, err = session.ID(core.PK{comment.Id}).Cols("anchor_id", "item_anchor_id", "updated_time").Update(comment)
			if err != nil {
				session.Rollback()
				return nil, err
			}
			result.UpdatedCount++
		}
	}

	if len(appliedMigrationFromIds) > 0 {
		fromIds := make([]string, 0, len(appliedMigrationFromIds))
		for id := range appliedMigrationFromIds {
			fromIds = append(fromIds, id)
			result.AppliedIds = append(result.AppliedIds, id)
		}
		_, err = session.In("from_anchor_id", fromIds).And("from_snapshot_id = ?", migResult.Migration[fromIds[0]].FromSnapshotId).And("to_snapshot_id = ?", toSnapId).Cols("applied", "updated_time", "manually_set_by").Update(&TaskAnchorMigration{Applied: true, UpdatedTime: now, ManuallySetBy: operator})
		if err != nil {
			session.Rollback()
			return nil, err
		}
	}

	if err = session.Commit(); err != nil {
		return nil, err
	}

	return result, nil
}

func SetReportCommentAnchorManually(commentId int64, newAnchorId, taskOwner, taskName, latestSnapshotId string, latestRecords []*TaskAnchorRecord, operator string) error {
	validAnchorIds := make(map[string]bool)
	anchorMap := make(map[string]*TaskAnchorRecord)
	for _, r := range latestRecords {
		if r != nil && r.AnchorId != "" {
			validAnchorIds[r.AnchorId] = true
			anchorMap[r.AnchorId] = r
		}
	}

	if newAnchorId != "" && !validAnchorIds[newAnchorId] {
		return fmt.Errorf("new anchor id %s not found in latest snapshot", newAnchorId)
	}

	comment, err := GetReportComment(commentId)
	if err != nil {
		return err
	}
	if comment == nil {
		return fmt.Errorf("report comment not found")
	}

	session := adapter.engine.NewSession()
	defer session.Close()

	if err = session.Begin(); err != nil {
		return err
	}

	now := util.GetCurrentTime()
	oldAnchorId := comment.AnchorId

	if newAnchorId != "" {
		comment.AnchorId = newAnchorId
		if anchor, ok := anchorMap[newAnchorId]; ok {
			if anchor.ItemAnchorId != "" {
				comment.ItemAnchorId = anchor.ItemAnchorId
			}
			if anchor.CategoryName != "" {
				comment.CategoryName = anchor.CategoryName
			}
			if anchor.ItemName != "" {
				comment.ItemName = anchor.ItemName
			}
			if anchor.ReferencedField != "" {
				comment.ReferencedField = anchor.ReferencedField
			}
			if anchor.ReferencedContent != "" {
				comment.ReferencedContent = anchor.ReferencedContent
			}
			if anchor.AnchorCategory != "" {
				comment.AnchorCategory = anchor.AnchorCategory
			}
			if anchor.AnchorItem != "" {
				comment.AnchorItem = anchor.AnchorItem
			}
			if anchor.AnchorField != "" {
				comment.AnchorField = anchor.AnchorField
			}
			if anchor.ContentHash != "" {
				comment.ContentHash = anchor.ContentHash
			}
		}
	}
	comment.UpdatedTime = now

	_, err = session.ID(core.PK{commentId}).Cols("anchor_id", "item_anchor_id", "category_name", "item_name", "referenced_field", "referenced_content", "anchor_category", "anchor_item", "anchor_field", "content_hash", "updated_time").Update(comment)
	if err != nil {
		session.Rollback()
		return err
	}

	if oldAnchorId != "" && newAnchorId != "" && oldAnchorId != newAnchorId {
		migration := &TaskAnchorMigration{
			TaskOwner:      taskOwner,
			TaskName:       taskName,
			FromSnapshotId: latestSnapshotId,
			ToSnapshotId:   latestSnapshotId,
			FromAnchorId:   oldAnchorId,
			ToAnchorId:     newAnchorId,
			MatchType:      MatchTypeManual,
			MatchScore:     0,
			IsAmbiguous:    false,
			Applied:        true,
			ManualOverride: true,
			ManuallySetBy:  operator,
			CreatedTime:    now,
			UpdatedTime:    now,
		}
		_, err = session.Insert(migration)
		if err != nil {
			session.Rollback()
			return err
		}
	}

	if err = session.Commit(); err != nil {
		return err
	}

	return nil
}

func GetLatestAnchorSnapshotFromComment(commentId int64) (*TaskAnchorSnapshot, error) {
	comment, err := GetReportComment(commentId)
	if err != nil {
		return nil, err
	}
	if comment == nil {
		return nil, fmt.Errorf("comment not found")
	}
	return GetLatestAnchorSnapshot(comment.TaskOwner, comment.TaskName)
}

func findBestAnchorByFallback(anchors []*TaskAnchor, comment *ReportComment) *TaskAnchor {
	if len(anchors) == 0 {
		return nil
	}
	type cand struct {
		a     *TaskAnchor
		score int
	}
	candidates := []cand{}
	for _, a := range anchors {
		s := 0
		if a.AnchorCategory != "" && a.AnchorCategory == comment.AnchorCategory {
			s += 1000
		}
		if a.AnchorItem != "" && a.AnchorItem == comment.AnchorItem {
			s += 100
		}
		if a.AnchorField != "" && comment.AnchorField != "" && a.AnchorField == comment.AnchorField {
			s += 10
		}
		if a.ReferencedField != "" && a.ReferencedField == comment.ReferencedField {
			s += 5
		}
		if a.ContentHash != "" && comment.ContentHash != "" && a.ContentHash == comment.ContentHash {
			s += 50
		}
		if s > 0 {
			candidates = append(candidates, cand{a, s})
		}
	}
	if len(candidates) == 0 {
		return nil
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].score > candidates[j].score })
	return candidates[0].a
}

func GroupCommentsWithAnchors(comments []*ReportComment, anchors []*TaskAnchor) map[string][]*ReportComment {
	result := map[string][]*ReportComment{}
	anchorByField := map[string]*TaskAnchor{}
	anchorByItem := map[string]*TaskAnchor{}
	anchorById := map[string]*TaskAnchor{}
	for _, a := range anchors {
		if a.AnchorId != "" {
			anchorById[a.AnchorId] = a
		}
		if a.AnchorType == AnchorTypeField && a.AnchorField != "" {
			anchorByField[a.AnchorField] = a
		}
		if a.AnchorType == AnchorTypeItem && a.AnchorItem != "" {
			if _, exists := anchorByItem[a.AnchorItem]; !exists {
				anchorByItem[a.AnchorItem] = a
			}
		}
	}
	for _, c := range comments {
		var matchedKey string
		if c.AnchorId != "" {
			if a, ok := anchorById[c.AnchorId]; ok && a.ItemAnchorId != "" {
				matchedKey = a.ItemAnchorId
			}
		}
		if matchedKey == "" && c.ItemAnchorId != "" {
			matchedKey = c.ItemAnchorId
		}
		if matchedKey == "" && c.AnchorField != "" {
			if a, ok := anchorByField[c.AnchorField]; ok && a.ItemAnchorId != "" {
				matchedKey = a.ItemAnchorId
			}
		}
		if matchedKey == "" && c.AnchorItem != "" {
			if a, ok := anchorByItem[c.AnchorItem]; ok && a.ItemAnchorId != "" {
				matchedKey = a.ItemAnchorId
			}
		}
		if matchedKey == "" {
			best := findBestAnchorByFallback(anchors, c)
			if best != nil && best.ItemAnchorId != "" {
				matchedKey = best.ItemAnchorId
			} else if best != nil && best.AnchorType == AnchorTypeItem {
				if best.ItemAnchorId != "" {
					matchedKey = best.ItemAnchorId
				} else if best.AnchorId != "" {
					matchedKey = best.AnchorId
				}
			}
		}
		if matchedKey == "" {
			if c.ItemAnchorId != "" {
				matchedKey = c.ItemAnchorId
			} else if c.StableItemKey != "" {
				matchedKey = c.StableItemKey
			} else {
				matchedKey = "__orphan__"
			}
		}
		result[matchedKey] = append(result[matchedKey], c)
	}
	return result
}

func GroupCommentsWithAnchorsV2(comments []*ReportComment, anchors []*TaskAnchor, latestSnapId string, reportVersion int, migration map[string]*TaskAnchorMigration) *GroupedCommentsResult {
	migrationMap := make(map[string]string)
	orphanMatchTypes := map[string]bool{}
	ambiguousMap := map[string][]*AnchorMigrationCandidate{}
	ambiguousCommentMap := map[string][]int64{}

	if migration != nil {
		for fromId, m := range migration {
			if m == nil {
				continue
			}
			if m.ToAnchorId != "" && (m.MatchType == MatchTypeExact || m.MatchType == MatchTypeFuzzy || m.MatchType == MatchTypeManual) {
				migrationMap[fromId] = m.ToAnchorId
			}
			if m.MatchType == MatchTypeOrphan {
				orphanMatchTypes[fromId] = true
			}
			if m.MatchType == MatchTypeAmbiguous {
				var cands []MigrationCandidate
				if m.CandidatesJson != "" {
					json.Unmarshal([]byte(m.CandidatesJson), &cands)
				}
				newCands := make([]*AnchorMigrationCandidate, 0, len(cands))
				for _, c := range cands {
					newCands = append(newCands, &AnchorMigrationCandidate{AnchorId: c.AnchorId, Score: c.Score})
				}
				ambiguousMap[fromId] = newCands
			}
		}
	}

	result := &GroupedCommentsResult{
		Grouped:       map[string][]*ReportComment{},
		OrphanCount:   0,
		Orphans:       []*ReportComment{},
		AmbiguousInfo: []*AmbiguousAnchorInfo{},
		LatestSnapId:  latestSnapId,
		ReportVersion: reportVersion,
		Migration:     migrationMap,
	}

	anchorByField := map[string]*TaskAnchor{}
	anchorByItem := map[string]*TaskAnchor{}
	anchorById := map[string]*TaskAnchor{}
	for _, a := range anchors {
		if a == nil {
			continue
		}
		if a.AnchorId != "" {
			anchorById[a.AnchorId] = a
		}
		if a.AnchorType == AnchorTypeField && a.AnchorField != "" {
			anchorByField[a.AnchorField] = a
		}
		if a.AnchorType == AnchorTypeItem && a.AnchorItem != "" {
			if _, exists := anchorByItem[a.AnchorItem]; !exists {
				anchorByItem[a.AnchorItem] = a
			}
		}
	}

	for _, c := range comments {
		if c == nil {
			continue
		}
		var matchedKey string
		currentAnchorId := c.AnchorId
		if mappedId, ok := migrationMap[currentAnchorId]; ok && mappedId != "" {
			currentAnchorId = mappedId
		}
		currentItemAnchorId := c.ItemAnchorId
		if mappedId, ok := migrationMap[currentItemAnchorId]; ok && mappedId != "" {
			currentItemAnchorId = mappedId
		}

		isOrphan := false
		if c.AnchorId != "" && orphanMatchTypes[c.AnchorId] {
			isOrphan = true
		}
		if !isOrphan && c.ItemAnchorId != "" && orphanMatchTypes[c.ItemAnchorId] {
			isOrphan = true
		}

		if c.AnchorId != "" {
			if _, ok := ambiguousMap[c.AnchorId]; ok {
				ambiguousCommentMap[c.AnchorId] = append(ambiguousCommentMap[c.AnchorId], c.Id)
			}
		}

		if isOrphan {
			result.Orphans = append(result.Orphans, c)
			result.OrphanCount++
			continue
		}

		if currentAnchorId != "" {
			if a, ok := anchorById[currentAnchorId]; ok && a.ItemAnchorId != "" {
				matchedKey = a.ItemAnchorId
			}
		}
		if matchedKey == "" && currentItemAnchorId != "" {
			matchedKey = currentItemAnchorId
		}
		if matchedKey == "" && c.AnchorField != "" {
			if a, ok := anchorByField[c.AnchorField]; ok && a.ItemAnchorId != "" {
				matchedKey = a.ItemAnchorId
			}
		}
		if matchedKey == "" && c.AnchorItem != "" {
			if a, ok := anchorByItem[c.AnchorItem]; ok && a.ItemAnchorId != "" {
				matchedKey = a.ItemAnchorId
			}
		}
		if matchedKey == "" {
			best := findBestAnchorByFallback(anchors, c)
			if best != nil && best.ItemAnchorId != "" {
				matchedKey = best.ItemAnchorId
			} else if best != nil && best.AnchorType == AnchorTypeItem {
				if best.ItemAnchorId != "" {
					matchedKey = best.ItemAnchorId
				} else if best.AnchorId != "" {
					matchedKey = best.AnchorId
				}
			}
		}
		if matchedKey == "" {
			if c.ItemAnchorId != "" {
				matchedKey = c.ItemAnchorId
			} else if c.StableItemKey != "" {
				matchedKey = c.StableItemKey
			} else {
				result.Orphans = append(result.Orphans, c)
				result.OrphanCount++
				continue
			}
		}
		result.Grouped[matchedKey] = append(result.Grouped[matchedKey], c)
	}

	for fromId, commentIds := range ambiguousCommentMap {
		if cands, ok := ambiguousMap[fromId]; ok {
			result.AmbiguousInfo = append(result.AmbiguousInfo, &AmbiguousAnchorInfo{
				CommentIds:   commentIds,
				FromAnchorId: fromId,
				Candidates:   cands,
			})
		}
	}

	return result
}

func sanitizeForStableKey(s string) string {
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(unicode.ToLower(r))
		} else if r == ' ' || r == '-' || r == '_' {
			b.WriteRune('_')
		}
	}
	res := b.String()
	if len(res) > 60 {
		res = res[:60]
	}
	return res
}

func GenerateStableCategoryKey(categoryName string, categoryIndex int) string {
	return fmt.Sprintf("cat_%d_%s", categoryIndex, sanitizeForStableKey(categoryName))
}

func GenerateStableItemKey(categoryName string, itemName string, categoryIndex int, itemIndex int) string {
	return fmt.Sprintf("item_%d_%d_%s_%s", categoryIndex, itemIndex, sanitizeForStableKey(categoryName), sanitizeForStableKey(itemName))
}

func AddReportComment(comment *ReportComment) (int64, error) {
	comment.FillAnchorsFromTask()
	if comment.CreatedTime == "" {
		comment.CreatedTime = util.GetCurrentTime()
	}
	if comment.UpdatedTime == "" {
		comment.UpdatedTime = comment.CreatedTime
	}
	if comment.Status == "" {
		comment.Status = ReportCommentStatusOpen
	}
	affected, err := adapter.engine.Insert(comment)
	if err != nil {
		return 0, err
	}
	return affected, nil
}

func GetReportComment(id int64) (*ReportComment, error) {
	comment := &ReportComment{Id: id}
	existed, err := adapter.engine.Get(comment)
	if err != nil {
		return nil, err
	}
	if !existed {
		return nil, nil
	}
	return comment, nil
}

func GetReportCommentsByTask(taskOwner string, taskName string) ([]*ReportComment, error) {
	comments := []*ReportComment{}
	session := adapter.engine.Where("task_owner = ? AND task_name = ?", taskOwner, taskName).Desc("created_time")
	err := session.Find(&comments)
	if err != nil {
		return comments, err
	}
	return comments, nil
}

func GetReportCommentsByTaskAndStatus(taskOwner string, taskName string, status string) ([]*ReportComment, error) {
	comments := []*ReportComment{}
	session := adapter.engine.Where("task_owner = ? AND task_name = ? AND status = ?", taskOwner, taskName, status).Desc("created_time")
	err := session.Find(&comments)
	if err != nil {
		return comments, err
	}
	return comments, nil
}

func GetReportCommentCountByTask(taskOwner string, taskName string) (map[string]int64, error) {
	result := map[string]int64{
		"total":      0,
		"open":       0,
		"resolved":   0,
		"disputed":   0,
		"unresolved": 0,
	}

	total, err := adapter.engine.Where("task_owner = ? AND task_name = ?", taskOwner, taskName).Count(&ReportComment{})
	if err != nil {
		return result, err
	}
	result["total"] = total

	for _, status := range []string{ReportCommentStatusOpen, ReportCommentStatusResolved, ReportCommentStatusDisputed} {
		count, err := adapter.engine.Where("task_owner = ? AND task_name = ? AND status = ?", taskOwner, taskName, status).Count(&ReportComment{})
		if err != nil {
			return result, err
		}
		result[status] = count
	}
	result["unresolved"] = result[ReportCommentStatusOpen] + result[ReportCommentStatusDisputed]

	return result, nil
}

func GetReportCommentsByStableItem(taskOwner string, taskName string, stableItemKey string) ([]*ReportComment, error) {
	comments := []*ReportComment{}
	session := adapter.engine.Where("task_owner = ? AND task_name = ? AND stable_item_key = ?", taskOwner, taskName, stableItemKey).Desc("created_time")
	err := session.Find(&comments)
	if err != nil {
		return comments, err
	}
	return comments, nil
}

func UpdateReportComment(id int64, comment *ReportComment) (bool, error) {
	existing, err := GetReportComment(id)
	if err != nil {
		return false, err
	}
	if existing == nil {
		return false, fmt.Errorf("report comment not found")
	}

	comment.FillAnchorsFromTask()
	comment.UpdatedTime = util.GetCurrentTime()
	_, err = adapter.engine.ID(core.PK{id}).AllCols().Update(comment)
	if err != nil {
		return false, err
	}
	return true, nil
}

func ResolveReportComment(id int64, resolver string) (bool, error) {
	existing, err := GetReportComment(id)
	if err != nil {
		return false, err
	}
	if existing == nil {
		return false, fmt.Errorf("report comment not found")
	}

	existing.Status = ReportCommentStatusResolved
	existing.Resolver = resolver
	existing.ResolvedTime = util.GetCurrentTime()
	existing.UpdatedTime = existing.ResolvedTime

	_, err = adapter.engine.ID(core.PK{id}).Cols("status", "resolver", "resolved_time", "updated_time").Update(existing)
	if err != nil {
		return false, err
	}
	return true, nil
}

func ReopenReportComment(id int64, resolver string) (bool, error) {
	existing, err := GetReportComment(id)
	if err != nil {
		return false, err
	}
	if existing == nil {
		return false, fmt.Errorf("report comment not found")
	}

	existing.Status = ReportCommentStatusOpen
	existing.Resolver = resolver
	existing.ResolvedTime = ""
	existing.UpdatedTime = util.GetCurrentTime()

	_, err = adapter.engine.ID(core.PK{id}).Cols("status", "resolver", "resolved_time", "updated_time").Update(existing)
	if err != nil {
		return false, err
	}
	return true, nil
}

func SetReportCommentDisputed(id int64, resolver string) (bool, error) {
	existing, err := GetReportComment(id)
	if err != nil {
		return false, err
	}
	if existing == nil {
		return false, fmt.Errorf("report comment not found")
	}

	existing.Status = ReportCommentStatusDisputed
	existing.Resolver = resolver
	existing.UpdatedTime = util.GetCurrentTime()

	_, err = adapter.engine.ID(core.PK{id}).Cols("status", "resolver", "updated_time").Update(existing)
	if err != nil {
		return false, err
	}
	return true, nil
}

func DeleteReportComment(id int64) (bool, error) {
	existing, err := GetReportComment(id)
	if err != nil {
		return false, err
	}
	if existing == nil {
		return false, fmt.Errorf("report comment not found")
	}

	affected, err := adapter.engine.ID(core.PK{id}).Delete(&ReportComment{})
	if err != nil {
		return false, err
	}
	return affected > 0, nil
}

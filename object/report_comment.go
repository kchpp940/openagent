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
	"crypto/sha1"
	"encoding/hex"
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

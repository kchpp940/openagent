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
	"fmt"
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

type ReportComment struct {
	Id                int64  `xorm:"pk autoincr" json:"id"`
	Owner             string `xorm:"varchar(100) notnull index" json:"owner"`
	TaskOwner         string `xorm:"varchar(100) notnull index" json:"taskOwner"`
	TaskName          string `xorm:"varchar(100) notnull index" json:"taskName"`
	StableCategoryKey string `xorm:"varchar(200) notnull index" json:"stableCategoryKey"`
	StableItemKey     string `xorm:"varchar(200) notnull index" json:"stableItemKey"`
	CategoryName      string `xorm:"varchar(200)" json:"categoryName"`
	ItemName          string `xorm:"varchar(200)" json:"itemName"`
	CategoryIndex     int    `xorm:"int" json:"categoryIndex"`
	ItemIndex         int    `xorm:"int" json:"itemIndex"`
	ReferencedField   string `xorm:"varchar(50)" json:"referencedField"`
	ReferencedContent string `xorm:"mediumtext" json:"referencedContent"`
	Content           string `xorm:"mediumtext" json:"content"`
	Status            string `xorm:"varchar(20) notnull default 'open'" json:"status"`
	Author            string `xorm:"varchar(100) notnull" json:"author"`
	Resolver          string `xorm:"varchar(100)" json:"resolver"`
	CreatedTime       string `xorm:"varchar(100)" json:"createdTime"`
	UpdatedTime       string `xorm:"varchar(100)" json:"updatedTime"`
	ResolvedTime      string `xorm:"varchar(100)" json:"resolvedTime"`
}

func GetTaskStableKeys(result *TaskResult) (map[string]int, map[string]int) {
	categoryKeys := map[string]int{}
	itemKeys := map[string]int{}
	if result == nil {
		return categoryKeys, itemKeys
	}
	for ci, cat := range result.Categories {
		if cat == nil {
			continue
		}
		catKey := GenerateStableCategoryKey(cat.Name, ci)
		categoryKeys[catKey] = ci
		for ii, item := range cat.Items {
			if item == nil {
				continue
			}
			itemKey := GenerateStableItemKey(cat.Name, item.Name, ci, ii)
			itemKeys[itemKey] = ii*1000 + ci
		}
	}
	return categoryKeys, itemKeys
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
		"total":    0,
		"open":     0,
		"resolved": 0,
		"disputed": 0,
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

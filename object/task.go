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

package object

import (
	"encoding/json"
	"fmt"

	"github.com/the-open-agent/openagent/util"
	"xorm.io/core"
)

const (
	DocumentParseStatusNone        = ""
	DocumentParseStatusPending     = "pending"
	DocumentParseStatusSuccess     = "success"
	DocumentParseStatusEmpty       = "empty"
	DocumentParseStatusFailed      = "failed"
	DocumentParseStatusUnsupported = "unsupported"
)

type TaskAnalysisItem struct {
	Name         string  `json:"name"`
	Score        float64 `json:"score"`
	Advantage    string  `json:"advantage"`
	Disadvantage string  `json:"disadvantage"`
	Suggestion   string  `json:"suggestion"`
}

type TaskAnalysisCategory struct {
	Name  string              `json:"name"`
	Score float64             `json:"score"`
	Items []*TaskAnalysisItem `json:"items"`
}

type TaskAnalysisReport struct {
	Title         string                  `json:"title"`
	Designer      string                  `json:"designer"`
	Stage         string                  `json:"stage"`
	Participants  string                  `json:"participants"`
	Grade         string                  `json:"grade"`
	Instructor    string                  `json:"instructor"`
	Subject       string                  `json:"subject"`
	School        string                  `json:"school"`
	OtherSubjects string                  `json:"otherSubjects"`
	Textbook      string                  `json:"textbook"`
	Score         float64                 `json:"score"`
	Summary       string                  `json:"summary"`
	Categories    []*TaskAnalysisCategory `json:"categories"`
}

type TaskResultItem = TaskAnalysisItem
type TaskResultCategory = TaskAnalysisCategory
type TaskResult = TaskAnalysisReport

type Task struct {
	Owner       string `xorm:"varchar(100) notnull pk" json:"owner"`
	Name        string `xorm:"varchar(100) notnull pk" json:"name"`
	CreatedTime string `xorm:"varchar(100)" json:"createdTime"`

	DisplayName string `xorm:"varchar(100)" json:"displayName"`
	Provider    string `xorm:"varchar(100)" json:"provider"`
	Type        string `xorm:"varchar(100)" json:"type"`

	Subject  string  `xorm:"varchar(100)" json:"subject"`
	Topic    string  `xorm:"varchar(100)" json:"topic"`
	Score    float64 `xorm:"float" json:"score"`
	Activity string  `xorm:"varchar(100)" json:"activity"`
	Grade    string  `xorm:"varchar(100)" json:"grade"`

	Path  string `xorm:"varchar(100)" json:"path"`
	Scale string `xorm:"varchar(100)" json:"scale"`

	Example string   `xorm:"varchar(200)" json:"example"`
	Labels  []string `xorm:"mediumtext" json:"labels"`
	Log     string   `xorm:"mediumtext" json:"log"`

	Result    string              `xorm:"mediumtext" json:"-"`
	ResultObj *TaskAnalysisReport `xorm:"-" json:"result"`

	DocumentUrl          string `xorm:"varchar(500)" json:"documentUrl"`
	DocumentText         string `xorm:"mediumtext" json:"documentText"`
	DocumentFileType     string `xorm:"varchar(100)" json:"documentFileType"`
	DocumentParseStatus  string `xorm:"varchar(50)" json:"documentParseStatus"`
	DocumentError        string `xorm:"varchar(500)" json:"documentError"`
	DocumentTypeSource   string `xorm:"varchar(50)" json:"documentTypeSource"`
	DocumentTypeConflict bool   `xorm:"bool" json:"documentTypeConflict"`
	DocumentConflictMsg  string `xorm:"varchar(500)" json:"documentConflictMsg"`
	AnalyzeError         string `xorm:"varchar(500)" json:"analyzeError"`
}

func (task *Task) IsDocumentReadyForAnalysis() bool {
	if task == nil {
		return false
	}
	return task.DocumentParseStatus == DocumentParseStatusSuccess && task.DocumentText != ""
}

func ParseTaskAnalysisReport(raw string) *TaskAnalysisReport {
	if raw == "" {
		return nil
	}
	var rawData map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &rawData); err != nil {
		return nil
	}
	return normalizeTaskResultFromMap(rawData)
}

func (task *Task) PopulateResultObj() {
	if task == nil {
		return
	}
	task.ResultObj = ParseTaskAnalysisReport(task.Result)
}

func (task *Task) SerializeResultObj() {
	if task == nil {
		return
	}
	if task.ResultObj == nil {
		task.Result = ""
		return
	}
	if b, err := json.Marshal(task.ResultObj); err == nil {
		task.Result = string(b)
	}
}

func PopulateTaskResultObj(task *Task) {
	if task != nil {
		task.PopulateResultObj()
	}
}

func PopulateTasksResultObj(tasks []*Task) {
	for _, t := range tasks {
		t.PopulateResultObj()
	}
}

func SerializeTaskResultObj(task *Task) {
	if task != nil {
		task.SerializeResultObj()
	}
}

func GetMaskedTask(task *Task, isMaskEnabled bool) *Task {
	if !isMaskEnabled {
		return task
	}

	if task == nil {
		return nil
	}

	return task
}

func GetMaskedTasks(tasks []*Task, isMaskEnabled bool) []*Task {
	if !isMaskEnabled {
		return tasks
	}

	for _, task := range tasks {
		task = GetMaskedTask(task, isMaskEnabled)
	}
	return tasks
}

func GetGlobalTasks(owner string) ([]*Task, error) {
	tasks := []*Task{}
	session := adapter.engine.Asc("owner").Desc("created_time")
	if owner != "" {
		session = session.Where("owner = ?", owner)
	}
	err := session.Find(&tasks)
	if err != nil {
		return tasks, err
	}

	PopulateTasksResultObj(tasks)
	return tasks, nil
}

func GetTasks(owner string) ([]*Task, error) {
	tasks := []*Task{}
	session := adapter.engine.Desc("created_time")
	if owner != "" {
		session = session.Where("owner = ?", owner)
	}
	err := session.Find(&tasks)
	if err != nil {
		return tasks, err
	}

	PopulateTasksResultObj(tasks)
	return tasks, nil
}

func getTask(owner string, name string) (*Task, error) {
	task := Task{Owner: owner, Name: name}
	existed, err := adapter.engine.Get(&task)
	if err != nil {
		return &task, err
	}

	if existed {
		task.PopulateResultObj()
		return &task, nil
	} else {
		return nil, nil
	}
}

func GetTask(id string) (*Task, error) {
	owner, name, err := util.GetOwnerAndNameFromIdWithError(id)
	if err != nil {
		return nil, err
	}
	return getTask(owner, name)
}

// GetTaskEffectiveScale returns rubric text: from referenced Scale.Text when Task.Scale is set.
func GetTaskEffectiveScale(task *Task) (string, error) {
	if task == nil {
		return "", fmt.Errorf("task is nil")
	}
	if task.Scale == "" {
		return "", nil
	}
	s, err := GetScale(task.Scale)
	if err != nil {
		return "", err
	}
	if s == nil {
		return "", nil
	}
	return s.Text, nil
}

func UpdateTask(id string, task *Task) (bool, error) {
	owner, name, err := util.GetOwnerAndNameFromIdWithError(id)
	if err != nil {
		return false, err
	}
	_, err = getTask(owner, name)
	if err != nil {
		return false, err
	}
	if task == nil {
		return false, nil
	}

	SerializeTaskResultObj(task)
	_, err = adapter.engine.ID(core.PK{owner, name}).AllCols().Update(task)
	if err != nil {
		return false, err
	}

	task.PopulateResultObj()
	return true, nil
}

func AddTask(task *Task) (bool, error) {
	SerializeTaskResultObj(task)
	affected, err := adapter.engine.Insert(task)
	if err != nil {
		return false, err
	}

	task.PopulateResultObj()
	return affected != 0, nil
}

func DeleteTask(task *Task) (bool, error) {
	affected, err := adapter.engine.ID(core.PK{task.Owner, task.Name}).Delete(&Task{})
	if err != nil {
		return false, err
	}

	return affected != 0, nil
}

func (task *Task) GetId() string {
	return fmt.Sprintf("%s/%s", task.Owner, task.Name)
}

func GetTaskCount(owner string, field, value string) (int64, error) {
	session := GetDbSession(owner, -1, -1, field, value, "", "")
	return session.Count(&Task{})
}

func GetPaginationTasks(owner string, offset, limit int, field, value, sortField, sortOrder string) ([]*Task, error) {
	tasks := []*Task{}
	session := GetDbSession(owner, offset, limit, field, value, sortField, sortOrder)
	err := session.Find(&tasks)
	if err != nil {
		return tasks, err
	}

	PopulateTasksResultObj(tasks)
	return tasks, nil
}

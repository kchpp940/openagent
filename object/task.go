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
	Owner       string `xorm:"varchar(100) notnull pk" json:"-"`
	Name        string `xorm:"varchar(100) notnull pk" json:"-"`
	CreatedTime string `xorm:"varchar(100)" json:"-"`

	DisplayName string `xorm:"varchar(100)" json:"-"`
	Provider    string `xorm:"varchar(100)" json:"-"`
	Type        string `xorm:"varchar(100)" json:"-"`

	Subject  string  `xorm:"varchar(100)" json:"-"`
	Topic    string  `xorm:"varchar(100)" json:"-"`
	Score    float64 `xorm:"float" json:"-"`
	Activity string  `xorm:"varchar(100)" json:"-"`
	Grade    string  `xorm:"varchar(100)" json:"-"`

	Path  string `xorm:"varchar(100)" json:"-"`
	Scale string `xorm:"varchar(100)" json:"-"`

	Example string   `xorm:"varchar(200)" json:"-"`
	Labels  []string `xorm:"mediumtext" json:"-"`
	Log     string   `xorm:"mediumtext" json:"-"`

	Result string `xorm:"mediumtext" json:"-"`

	DocumentUrl          string `xorm:"varchar(500)" json:"-"`
	DocumentText         string `xorm:"mediumtext" json:"-"`
	DocumentFileType     string `xorm:"varchar(100)" json:"-"`
	DocumentParseStatus  string `xorm:"varchar(50)" json:"-"`
	DocumentError        string `xorm:"varchar(500)" json:"-"`
	DocumentTypeSource   string `xorm:"varchar(50)" json:"-"`
	DocumentTypeConflict bool   `xorm:"bool" json:"-"`
	DocumentConflictMsg  string `xorm:"varchar(500)" json:"-"`
	AnalyzeError         string `xorm:"varchar(500)" json:"-"`
}

type TaskResponse struct {
	Owner       string              `json:"owner"`
	Name        string              `json:"name"`
	CreatedTime string              `json:"createdTime"`
	DisplayName string              `json:"displayName"`
	Provider    string              `json:"provider"`
	Type        string              `json:"type"`
	Subject     string              `json:"subject"`
	Topic       string              `json:"topic"`
	Score       float64             `json:"score"`
	Activity    string              `json:"activity"`
	Grade       string              `json:"grade"`
	Path        string              `json:"path"`
	Scale       string              `json:"scale"`
	Example     string              `json:"example"`
	Labels      []string            `json:"labels"`
	Log         string              `json:"log"`
	Result      *TaskAnalysisReport `json:"result"`

	DocumentUrl          string `json:"documentUrl"`
	DocumentText         string `json:"documentText"`
	DocumentFileType     string `json:"documentFileType"`
	DocumentParseStatus  string `json:"documentParseStatus"`
	DocumentError        string `json:"documentError"`
	DocumentTypeSource   string `json:"documentTypeSource"`
	DocumentTypeConflict bool   `json:"documentTypeConflict"`
	DocumentConflictMsg  string `json:"documentConflictMsg"`
	AnalyzeError         string `json:"analyzeError"`
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

func SerializeTaskAnalysisReport(report *TaskAnalysisReport) string {
	if report == nil {
		return ""
	}
	b, err := json.Marshal(report)
	if err != nil {
		return ""
	}
	return string(b)
}

func BuildTaskResponse(task *Task) *TaskResponse {
	if task == nil {
		return nil
	}
	return &TaskResponse{
		Owner:       task.Owner,
		Name:        task.Name,
		CreatedTime: task.CreatedTime,
		DisplayName: task.DisplayName,
		Provider:    task.Provider,
		Type:        task.Type,
		Subject:     task.Subject,
		Topic:       task.Topic,
		Score:       task.Score,
		Activity:    task.Activity,
		Grade:       task.Grade,
		Path:        task.Path,
		Scale:       task.Scale,
		Example:     task.Example,
		Labels:      task.Labels,
		Log:         task.Log,
		Result:      ParseTaskAnalysisReport(task.Result),

		DocumentUrl:          task.DocumentUrl,
		DocumentText:         task.DocumentText,
		DocumentFileType:     task.DocumentFileType,
		DocumentParseStatus:  task.DocumentParseStatus,
		DocumentError:        task.DocumentError,
		DocumentTypeSource:   task.DocumentTypeSource,
		DocumentTypeConflict: task.DocumentTypeConflict,
		DocumentConflictMsg:  task.DocumentConflictMsg,
		AnalyzeError:         task.AnalyzeError,
	}
}

func BuildTaskResponses(tasks []*Task) []*TaskResponse {
	if tasks == nil {
		return nil
	}
	resp := make([]*TaskResponse, 0, len(tasks))
	for _, t := range tasks {
		resp = append(resp, BuildTaskResponse(t))
	}
	return resp
}

func ParseTaskFromResponse(r *TaskResponse) *Task {
	if r == nil {
		return nil
	}
	return &Task{
		Owner:       r.Owner,
		Name:        r.Name,
		CreatedTime: r.CreatedTime,
		DisplayName: r.DisplayName,
		Provider:    r.Provider,
		Type:        r.Type,
		Subject:     r.Subject,
		Topic:       r.Topic,
		Score:       r.Score,
		Activity:    r.Activity,
		Grade:       r.Grade,
		Path:        r.Path,
		Scale:       r.Scale,
		Example:     r.Example,
		Labels:      r.Labels,
		Log:         r.Log,
		Result:      SerializeTaskAnalysisReport(r.Result),

		DocumentUrl:          r.DocumentUrl,
		DocumentText:         r.DocumentText,
		DocumentFileType:     r.DocumentFileType,
		DocumentParseStatus:  r.DocumentParseStatus,
		DocumentError:        r.DocumentError,
		DocumentTypeSource:   r.DocumentTypeSource,
		DocumentTypeConflict: r.DocumentTypeConflict,
		DocumentConflictMsg:  r.DocumentConflictMsg,
		AnalyzeError:         r.AnalyzeError,
	}
}

func GetMaskedTask(task *TaskResponse, isMaskEnabled bool) *TaskResponse {
	if !isMaskEnabled {
		return task
	}

	if task == nil {
		return nil
	}

	return task
}

func GetMaskedTasks(tasks []*TaskResponse, isMaskEnabled bool) []*TaskResponse {
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

	return tasks, nil
}

func getTask(owner string, name string) (*Task, error) {
	task := Task{Owner: owner, Name: name}
	existed, err := adapter.engine.Get(&task)
	if err != nil {
		return &task, err
	}

	if existed {
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

	_, err = adapter.engine.ID(core.PK{owner, name}).AllCols().Update(task)
	if err != nil {
		return false, err
	}

	return true, nil
}

func AddTask(task *Task) (bool, error) {
	affected, err := adapter.engine.Insert(task)
	if err != nil {
		return false, err
	}

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

	return tasks, nil
}

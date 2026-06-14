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

type TaskResultItem struct {
	Name         string  `json:"name"`
	Score        float64 `json:"score"`
	Advantage    string  `json:"advantage"`
	Disadvantage string  `json:"disadvantage"`
	Suggestion   string  `json:"suggestion"`
}

type TaskResultCategory struct {
	Name  string            `json:"name"`
	Score float64           `json:"score"`
	Items []*TaskResultItem `json:"items"`
}

type TaskResult struct {
	Title         string                `json:"title"`
	Designer      string                `json:"designer"`
	Stage         string                `json:"stage"`
	Participants  string                `json:"participants"`
	Grade         string                `json:"grade"`
	Instructor    string                `json:"instructor"`
	Subject       string                `json:"subject"`
	School        string                `json:"school"`
	OtherSubjects string                `json:"otherSubjects"`
	Textbook      string                `json:"textbook"`
	Score         float64               `json:"score"`
	Categories    []*TaskResultCategory `json:"categories"`
}

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

	Result string `xorm:"mediumtext" json:"result"`

	DocumentUrl            string `xorm:"varchar(500)" json:"documentUrl"`
	DocumentText           string `xorm:"mediumtext" json:"documentText"`
	DocumentFileType       string `xorm:"varchar(100)" json:"documentFileType"`
	DocumentMimeType       string `xorm:"varchar(200)" json:"documentMimeType"`
	DocumentFileSize       int64  `xorm:"bigint" json:"documentFileSize"`
	DocumentResourceId     string `xorm:"varchar(200)" json:"documentResourceId"`
	DocumentFileName       string `xorm:"varchar(500)" json:"documentFileName"`
	DocumentUploadSuccess  bool   `xorm:"bool" json:"documentUploadSuccess"`
	DocumentParseStatus    string `xorm:"varchar(50)" json:"documentParseStatus"`
	DocumentError          string `xorm:"varchar(500)" json:"documentError"`
	DocumentTypeSource     string `xorm:"varchar(50)" json:"documentTypeSource"`
	DocumentTypeConflict   bool   `xorm:"bool" json:"documentTypeConflict"`
	DocumentConflictMsg    string `xorm:"varchar(500)" json:"documentConflictMsg"`
	AnalyzeError           string `xorm:"varchar(500)" json:"analyzeError"`
}

func (task *Task) ResetDocumentFields() {
	task.DocumentUrl = ""
	task.DocumentText = ""
	task.DocumentFileType = ""
	task.DocumentMimeType = ""
	task.DocumentFileSize = 0
	task.DocumentResourceId = ""
	task.DocumentFileName = ""
	task.DocumentUploadSuccess = false
	task.DocumentParseStatus = DocumentParseStatusNone
	task.DocumentError = ""
	task.DocumentTypeSource = ""
	task.DocumentTypeConflict = false
	task.DocumentConflictMsg = ""
	task.AnalyzeError = ""
}

type DocumentUploadInfo struct {
	FileName      string
	MimeType      string
	FileType      string
	FileUrl       string
	ResourceId    string
	FileSize      int64
	TypeSource    string
	TypeConflict  bool
	ConflictMsg   string
}

type DocumentParseResult struct {
	Status       string
	Error        string
	Text         string
}

// SyncDocumentStatus updates both Task and Resource in a single unified call.
// upload=nil means upload failed (clears all document fields).
// parse=nil means only upload info should be recorded (parse pending).
// parse!=nil means final parse result should be applied.
func SyncDocumentStatus(taskId string, task *Task, upload *DocumentUploadInfo, parse *DocumentParseResult) error {
	if task == nil {
		return nil
	}

	if upload == nil {
		task.DocumentUploadSuccess = false
		task.DocumentParseStatus = DocumentParseStatusFailed
		task.DocumentError = ""
		task.DocumentUrl = ""
		task.DocumentText = ""
		task.DocumentResourceId = ""
		task.DocumentFileSize = 0
		task.DocumentFileName = ""
		task.DocumentMimeType = ""
		task.DocumentFileType = ""
		task.DocumentTypeSource = ""
		task.DocumentTypeConflict = false
		task.DocumentConflictMsg = ""
		task.AnalyzeError = ""

		if parse != nil {
			task.DocumentError = parse.Error
		}

		_, err := UpdateTask(taskId, task)
		return err
	}

	task.DocumentUploadSuccess = true
	task.DocumentFileName = upload.FileName
	task.DocumentMimeType = upload.MimeType
	task.DocumentFileType = upload.FileType
	task.DocumentUrl = upload.FileUrl
	task.DocumentResourceId = upload.ResourceId
	task.DocumentFileSize = upload.FileSize
	task.DocumentTypeSource = upload.TypeSource
	task.DocumentTypeConflict = upload.TypeConflict
	task.DocumentConflictMsg = upload.ConflictMsg
	task.DocumentError = ""
	task.DocumentText = ""
	task.AnalyzeError = ""

	if parse == nil {
		task.DocumentParseStatus = DocumentParseStatusPending
	} else {
		task.DocumentParseStatus = parse.Status
		task.DocumentError = parse.Error
		task.DocumentText = parse.Text
	}

	if upload.ResourceId != "" {
		resource, resErr := GetResource(upload.ResourceId)
		if resErr == nil && resource != nil {
			resource.ParseStatus = task.DocumentParseStatus
			resource.ParseError = task.DocumentError
			_, _ = UpdateResource(upload.ResourceId, resource)
		}
	}

	_, err := UpdateTask(taskId, task)
	return err
}

func (task *Task) IsDocumentReadyForAnalysis() bool {
	if task == nil {
		return false
	}
	return task.DocumentParseStatus == DocumentParseStatusSuccess
}

type DocumentStatusResponse struct {
	Url              string `json:"url"`
	Text             string `json:"text"`
	ParseStatus      string `json:"parseStatus"`
	Error            string `json:"error"`
	FileType         string `json:"fileType"`
	MimeType         string `json:"mimeType"`
	FileSize         int64  `json:"fileSize"`
	FileName         string `json:"fileName"`
	ResourceId       string `json:"resourceId"`
	TypeSource       string `json:"typeSource"`
	TypeConflict     bool   `json:"typeConflict"`
	ConflictMessage  string `json:"conflictMessage"`
	FileNameExt      string `json:"fileNameExt"`
	MimeTypeExt      string `json:"mimeTypeExt"`
	ParseSuccess     bool   `json:"parseSuccess"`
	UploadSuccess    bool   `json:"uploadSuccess"`
}

func (task *Task) BuildDocumentStatusResponse(extra *DocumentTypeDetectionExtra) *DocumentStatusResponse {
	if task == nil {
		return &DocumentStatusResponse{
			ParseStatus:   DocumentParseStatusNone,
			UploadSuccess: false,
			ParseSuccess:  false,
		}
	}

	resp := &DocumentStatusResponse{
		Url:             task.DocumentUrl,
		Text:            task.DocumentText,
		ParseStatus:     task.DocumentParseStatus,
		Error:           task.DocumentError,
		FileType:        task.DocumentFileType,
		MimeType:        task.DocumentMimeType,
		FileSize:        task.DocumentFileSize,
		FileName:        task.DocumentFileName,
		ResourceId:      task.DocumentResourceId,
		TypeSource:      task.DocumentTypeSource,
		TypeConflict:    task.DocumentTypeConflict,
		ConflictMessage: task.DocumentConflictMsg,
		UploadSuccess:   task.DocumentUploadSuccess,
		ParseSuccess:    task.DocumentParseStatus == DocumentParseStatusSuccess,
	}

	if extra != nil {
		resp.FileNameExt = extra.FileNameExt
		resp.MimeTypeExt = extra.MimeTypeExt
	}

	return resp
}

type DocumentTypeDetectionExtra struct {
	FileNameExt string
	MimeTypeExt string
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

	// return affected != 0
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

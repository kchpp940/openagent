// Copyright 2025 The OpenAgent Authors. All Rights Reserved.
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
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"sort"
	"strings"

	"github.com/the-open-agent/openagent/i18n"
	"github.com/the-open-agent/openagent/storage"
	"github.com/the-open-agent/openagent/util"
	"xorm.io/core"
)

type KnowledgeFileState string

const (
	KnowledgeFileStateUploaded      KnowledgeFileState = "Uploaded"
	KnowledgeFileStateParsing       KnowledgeFileState = "Parsing"
	KnowledgeFileStateVectorizing   KnowledgeFileState = "Vectorizing"
	KnowledgeFileStatePartialFailed KnowledgeFileState = "PartialFailed"
	KnowledgeFileStateReady         KnowledgeFileState = "Ready"
	KnowledgeFileStateFailed        KnowledgeFileState = "Failed"
)

type VectorBuildState string

const (
	VectorBuildStatePending   VectorBuildState = "Pending"
	VectorBuildStateBuilding  VectorBuildState = "Building"
	VectorBuildStatePartial   VectorBuildState = "Partial"
	VectorBuildStateCompleted VectorBuildState = "Completed"
	VectorBuildStateFailed    VectorBuildState = "Failed"
)

type FileStateDetail struct {
	State         KnowledgeFileState `json:"state"`
	VectorState   VectorBuildState   `json:"vectorState"`
	ErrorText     string             `json:"errorText"`
	VectorError   string             `json:"vectorError"`
	Progress      int                `json:"progress"`
	TotalSections int                `json:"totalSections"`
	CanRetry      bool               `json:"canRetry"`
	CanDelete     bool               `json:"canDelete"`
	Label         string             `json:"label"`
	LabelColor    string             `json:"labelColor"`
}

type FileStatus string

const (
	FileStatusPending    FileStatus = "Pending"
	FileStatusProcessing FileStatus = "Processing"
	FileStatusFinished   FileStatus = "Finished"
	FileStatusError      FileStatus = "Error"
)

type File struct {
	Owner       string `xorm:"varchar(100) notnull pk" json:"owner"`
	Name        string `xorm:"varchar(512) notnull pk" json:"name"`
	CreatedTime string `xorm:"varchar(100)" json:"createdTime"`

	Filename        string     `xorm:"varchar(255)" json:"filename"`
	Size            int64      `json:"size"`
	Store           string     `xorm:"varchar(100)" json:"store"`
	StorageProvider string     `xorm:"varchar(100)" json:"storageProvider"`
	Url             string     `xorm:"varchar(500)" json:"url"`
	TokenCount      int        `json:"tokenCount"`
	VectorCount     int        `xorm:"-" json:"vectorCount"`
	Status          FileStatus `xorm:"varchar(100)" json:"status"`
	ErrorText       string     `xorm:"mediumtext" json:"errorText"`

	FileState     KnowledgeFileState `xorm:"varchar(100)" json:"fileState"`
	VectorState   VectorBuildState   `xorm:"varchar(100)" json:"vectorState"`
	VectorError   string             `xorm:"mediumtext" json:"vectorError"`
	Progress      int                `json:"progress"`
	TotalSections int                `json:"totalSections"`
	StateDetail   *FileStateDetail   `xorm:"-" json:"stateDetail"`
}

func populateFileVectorCounts(files []*File) error {
	type result struct {
		File  string
		Store string
		Count int64
	}
	var results []result
	err := adapter.engine.Table("vector").Select("file, store, count(*) as count").GroupBy("file, store").Find(&results)
	if err != nil {
		return err
	}

	countMap := make(map[string]int, len(results))
	for _, r := range results {
		countMap[r.Store+"/"+r.File] = int(r.Count)
	}

	for _, file := range files {
		objectKey := file.getObjectKey()
		file.VectorCount = countMap[file.Store+"/"+objectKey]
	}
	return nil
}

func GetGlobalFiles() ([]*File, error) {
	files := []*File{}
	err := adapter.engine.Asc("owner").Desc("created_time").Find(&files)
	if err != nil {
		return files, err
	}

	err = populateFileVectorCounts(files)
	if err != nil {
		return files, err
	}

	err = populateFileStateDetails(files)
	if err != nil {
		return files, err
	}

	return files, nil
}

func GetFiles(owner string) ([]*File, error) {
	files := []*File{}
	err := adapter.engine.Desc("created_time").Find(&files, &File{Owner: owner})
	if err != nil {
		return files, err
	}

	err = populateFileVectorCounts(files)
	if err != nil {
		return files, err
	}

	err = populateFileStateDetails(files)
	if err != nil {
		return files, err
	}

	return files, nil
}

func GetFilesByStore(owner string, store string) ([]*File, error) {
	files := []*File{}
	err := adapter.engine.Desc("created_time").Find(&files, &File{Owner: owner, Store: store})
	if err != nil {
		return files, err
	}

	err = populateFileVectorCounts(files)
	if err != nil {
		return files, err
	}

	err = populateFileStateDetails(files)
	if err != nil {
		return files, err
	}

	return files, nil
}

func getFile(owner string, name string) (*File, error) {
	file := File{Owner: owner, Name: name}
	existed, err := adapter.engine.Get(&file)
	if err != nil {
		return &file, err
	}

	if existed {
		return &file, nil
	} else {
		return nil, nil
	}
}

func GetFile(id string) (*File, error) {
	owner, name := util.GetOwnerAndNameFromIdNoCheck(id)
	file, err := getFile(owner, name)
	if err != nil || file == nil {
		return file, err
	}

	files := []*File{file}
	err = populateFileVectorCounts(files)
	if err != nil {
		return file, err
	}
	err = populateFileStateDetails(files)
	if err != nil {
		return file, err
	}
	return file, nil
}

func UpdateFile(id string, file *File) (bool, error) {
	owner, name := util.GetOwnerAndNameFromIdNoCheck(id)
	_, err := getFile(owner, name)
	if err != nil {
		return false, err
	}
	if file == nil {
		return false, nil
	}

	_, err = adapter.engine.ID(core.PK{owner, name}).AllCols().Update(file)
	if err != nil {
		return false, err
	}

	return true, nil
}

func AddFile(file *File) (bool, error) {
	affected, err := adapter.engine.Insert(file)
	if err != nil {
		return false, err
	}

	return affected != 0, nil
}

func DeleteFile(file *File, lang string) (bool, error) {
	objectKey := file.getObjectKey()
	if objectKey == "" {
		return false, fmt.Errorf(i18n.Translate(lang, "object:The file: %s is not found"), file.Name)
	}

	storageProviderObj, err := getDefaultStorageProviderObj(lang)
	if err != nil {
		return false, err
	}

	err = storageProviderObj.DeleteObject(objectKey)
	if err != nil {
		return false, err
	}

	_, err = DeleteVectorsByFile(file.Owner, file.Store, objectKey)
	if err != nil {
		return false, err
	}

	affected, err := adapter.engine.ID(core.PK{file.Owner, file.Name}).Delete(&File{})
	if err != nil {
		return false, err
	}

	return affected != 0, nil
}

func (file *File) GetId() string {
	return fmt.Sprintf("%s/%s", file.Owner, file.Name)
}

func getFileName(storeName string, objectKey string) string {
	return fmt.Sprintf("%s_%s", storeName, objectKey)
}

func getObjectKey(storeName string, fileName string) string {
	if storeName == "" {
		return fileName
	}
	prefix := fmt.Sprintf("%s_", storeName)
	if strings.HasPrefix(fileName, prefix) {
		return strings.TrimPrefix(fileName, prefix)
	}
	return fileName
}

func (file *File) getObjectKey() string {
	return getObjectKey(file.Store, file.Name)
}

func GetFileCount(owner, store, field, value string) (int64, error) {
	session := GetDbSession(owner, -1, -1, field, value, "", "")
	if store != "" {
		return session.Count(&File{Store: store})
	}
	return session.Count(&File{})
}

var fileVirtualSortFields = map[string]bool{
	"vectorCount": true,
}

func GetPaginationFiles(owner, store string, offset, limit int, field, value, sortField, sortOrder string) ([]*File, error) {
	files := []*File{}
	dbSortField, dbSortOrder := sortField, sortOrder
	if fileVirtualSortFields[sortField] {
		dbSortField, dbSortOrder = "", ""
	}
	session := GetDbSession(owner, offset, limit, field, value, dbSortField, dbSortOrder)
	var err error
	if store != "" {
		err = session.Find(&files, &File{Store: store})
	} else {
		err = session.Find(&files)
	}
	if err != nil {
		return files, err
	}

	err = populateFileVectorCounts(files)
	if err != nil {
		return files, err
	}

	err = populateFileStateDetails(files)
	if err != nil {
		return files, err
	}

	if fileVirtualSortFields[sortField] {
		sort.SliceStable(files, func(i, j int) bool {
			if sortOrder == "ascend" {
				return files[i].VectorCount < files[j].VectorCount
			}
			return files[i].VectorCount > files[j].VectorCount
		})
	}

	return files, nil
}

func updateFileStatus(owner string, storeName string, objectKey string, status FileStatus, errorText string, tokenCount int) error {
	name := getFileName(storeName, objectKey)
	cols := []string{"status", "error_text"}
	file := &File{Status: status, ErrorText: errorText}
	if status == FileStatusProcessing {
		cols = append(cols, "token_count")
		file.TokenCount = 0
	} else if status == FileStatusFinished || status == FileStatusError {
		cols = append(cols, "token_count")
		file.TokenCount = tokenCount
	}
	_, err := adapter.engine.ID(core.PK{owner, name}).Cols(cols...).Update(file)
	return err
}

func UpdateFilesStatusByStore(owner string, storeName string, status FileStatus) error {
	_, err := adapter.engine.Where("owner = ? and store = ?", owner, storeName).
		Cols("status", "error_text").Update(&File{Status: status, ErrorText: ""})
	return err
}

func ResetFilesStateByStore(owner string, storeName string) error {
	_, err := adapter.engine.Where("owner = ? and store = ?", owner, storeName).
		Cols("file_state", "vector_state", "error_text", "vector_error", "progress", "total_sections", "status").
		Update(&File{
			FileState:   KnowledgeFileStateUploaded,
			VectorState: VectorBuildStatePending,
			Status:      FileStatusPending,
		})
	return err
}

func (f *File) ComputeStateDetail() *FileStateDetail {
	detail := &FileStateDetail{
		State:         f.FileState,
		VectorState:   f.VectorState,
		ErrorText:     f.ErrorText,
		VectorError:   f.VectorError,
		Progress:      f.Progress,
		TotalSections: f.TotalSections,
		CanDelete:     true,
	}

	if f.FileState == "" {
		if f.Status == FileStatusFinished {
			detail.State = KnowledgeFileStateReady
			detail.VectorState = VectorBuildStateCompleted
		} else if f.Status == FileStatusProcessing {
			detail.State = KnowledgeFileStateVectorizing
			detail.VectorState = VectorBuildStateBuilding
		} else if f.Status == FileStatusError {
			detail.State = KnowledgeFileStateFailed
			detail.VectorState = VectorBuildStateFailed
		} else {
			detail.State = KnowledgeFileStateUploaded
			detail.VectorState = VectorBuildStatePending
		}
	}

	switch detail.State {
	case KnowledgeFileStateUploaded:
		detail.Label = "Uploaded"
		detail.LabelColor = "default"
		detail.CanRetry = false
	case KnowledgeFileStateParsing:
		detail.Label = "Parsing"
		detail.LabelColor = "processing"
		detail.CanRetry = false
	case KnowledgeFileStateVectorizing:
		detail.Label = "Vectorizing"
		detail.LabelColor = "processing"
		detail.CanRetry = false
	case KnowledgeFileStatePartialFailed:
		detail.Label = "Partial Failed"
		detail.LabelColor = "warning"
		detail.CanRetry = true
	case KnowledgeFileStateReady:
		detail.Label = "Ready"
		detail.LabelColor = "success"
		detail.CanRetry = true
	case KnowledgeFileStateFailed:
		detail.Label = "Failed"
		detail.LabelColor = "error"
		detail.CanRetry = true
	default:
		detail.Label = string(detail.State)
		detail.LabelColor = "default"
		detail.CanRetry = true
	}

	return detail
}

func populateFileStateDetails(files []*File) error {
	for _, f := range files {
		f.StateDetail = f.ComputeStateDetail()
	}
	return nil
}

type SetFileStateOptions struct {
	FileState     KnowledgeFileState
	VectorState   VectorBuildState
	ErrorText     string
	VectorError   string
	Progress      int
	TotalSections int
	TokenCount    int
	UpdateTokens  bool
}

func SetFileState(owner string, storeName string, objectKey string, opts SetFileStateOptions) error {
	name := getFileName(storeName, objectKey)
	cols := []string{}
	file := &File{}

	if opts.FileState != "" {
		cols = append(cols, "file_state")
		file.FileState = opts.FileState
	}
	if opts.VectorState != "" {
		cols = append(cols, "vector_state")
		file.VectorState = opts.VectorState
	}
	if opts.ErrorText != "" || opts.FileState == KnowledgeFileStateReady || opts.FileState == KnowledgeFileStateUploaded {
		cols = append(cols, "error_text")
		file.ErrorText = opts.ErrorText
	}
	if opts.VectorError != "" || opts.VectorState == VectorBuildStateCompleted || opts.VectorState == VectorBuildStatePending {
		cols = append(cols, "vector_error")
		file.VectorError = opts.VectorError
	}
	if opts.Progress > 0 || opts.TotalSections > 0 {
		cols = append(cols, "progress", "total_sections")
		file.Progress = opts.Progress
		file.TotalSections = opts.TotalSections
	}
	if opts.UpdateTokens {
		cols = append(cols, "token_count")
		file.TokenCount = opts.TokenCount
	}

	legacyStatus := mapKnowledgeStateToLegacy(opts.FileState, opts.VectorState)
	if legacyStatus != "" {
		cols = append(cols, "status")
		file.Status = legacyStatus
	}

	if len(cols) == 0 {
		return nil
	}

	_, err := adapter.engine.ID(core.PK{owner, name}).Cols(cols...).Update(file)
	return err
}

func mapKnowledgeStateToLegacy(fileState KnowledgeFileState, vectorState VectorBuildState) FileStatus {
	switch fileState {
	case KnowledgeFileStateUploaded:
		return FileStatusPending
	case KnowledgeFileStateParsing, KnowledgeFileStateVectorizing:
		return FileStatusProcessing
	case KnowledgeFileStateReady:
		return FileStatusFinished
	case KnowledgeFileStatePartialFailed, KnowledgeFileStateFailed:
		return FileStatusError
	default:
		if vectorState == VectorBuildStateBuilding {
			return FileStatusProcessing
		}
		if vectorState == VectorBuildStateCompleted {
			return FileStatusFinished
		}
		if vectorState == VectorBuildStateFailed {
			return FileStatusError
		}
		return ""
	}
}

func SetFileStateWithFallback(file *File, opts SetFileStateOptions) error {
	return SetFileState(file.Owner, file.Store, file.getObjectKey(), opts)
}

func deleteFileRecord(owner string, storeName string, objectKey string) error {
	name := getFileName(storeName, objectKey)
	_, err := adapter.engine.ID(core.PK{owner, name}).Delete(&File{})
	return err
}

// getDefaultStorageProviderObj returns the storage provider marked as IsDefault.
func getDefaultStorageProviderObj(lang string) (storage.StorageProvider, error) {
	provider, err := GetDefaultStorageProvider()
	if err != nil {
		return nil, err
	}
	if provider == nil {
		return nil, fmt.Errorf(i18n.Translate(lang, "object:The provider: %s does not exist"), "default storage provider")
	}
	return provider.GetStorageProviderObj("", lang)
}

func UploadFile(owner string, userName string, filename string, fileData multipart.File, lang string, origin string) (*File, error) {
	provider, err := GetDefaultStorageProvider()
	if err != nil {
		return nil, err
	}
	if provider == nil {
		return nil, fmt.Errorf(i18n.Translate(lang, "object:The provider: %s does not exist"), "default storage provider")
	}

	defaultStore, err := GetDefaultStore(owner)
	if err != nil {
		return nil, err
	}
	if defaultStore == nil {
		return nil, fmt.Errorf(i18n.Translate(lang, "object:No store found for user: %s, please create a store first"), owner)
	}
	storeName := defaultStore.Name

	storageProviderObj, err := provider.GetStorageProviderObj("", lang)
	if err != nil {
		return nil, err
	}

	objectKey := fmt.Sprintf("file_%s/%s", util.GetRandomName(), filename)
	fileBuffer := bytes.NewBuffer(nil)
	_, err = io.Copy(fileBuffer, fileData)
	if err != nil {
		return nil, err
	}

	fileSize := int64(fileBuffer.Len())
	rawUrl, err := storageProviderObj.PutObject(userName, provider.Name, objectKey, fileBuffer)
	if err != nil {
		return nil, err
	}

	fileUrl, err := getUrlFromPath(rawUrl, origin)
	if err != nil {
		return nil, err
	}

	fileRecord := &File{
		Owner:           owner,
		Name:            objectKey,
		CreatedTime:     util.GetCurrentTime(),
		Filename:        filename,
		Size:            fileSize,
		Store:           storeName,
		StorageProvider: provider.Name,
		Url:             fileUrl,
		TokenCount:      0,
		Status:          FileStatusPending,
		FileState:       KnowledgeFileStateUploaded,
		VectorState:     VectorBuildStatePending,
	}

	_, err = AddFile(fileRecord)
	if err != nil {
		return nil, err
	}

	return fileRecord, nil
}

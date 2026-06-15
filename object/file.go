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
	"crypto/sha256"
	"fmt"
	"io"
	"mime/multipart"
	"sort"
	"strings"

	"github.com/beego/beego/logs"
	"github.com/the-open-agent/openagent/i18n"
	"github.com/the-open-agent/openagent/storage"
	"github.com/the-open-agent/openagent/util"
	"xorm.io/core"
)

type FileStatus string

const (
	FileStatusPending       FileStatus = "Pending"
	FileStatusParsing       FileStatus = "Parsing"
	FileStatusVectorizing   FileStatus = "Vectorizing"
	FileStatusProcessing    FileStatus = "Processing"
	FileStatusFinished      FileStatus = "Finished"
	FileStatusPartialFailed FileStatus = "PartialFailed"
	FileStatusError         FileStatus = "Error"
	FileStatusObsolete      FileStatus = "Obsolete"
	FileStatusSkipped       FileStatus = "Skipped"
)

type ChunkDiffType string

const (
	ChunkDiffAdded     ChunkDiffType = "Added"
	ChunkDiffDeleted   ChunkDiffType = "Deleted"
	ChunkDiffModified  ChunkDiffType = "Modified"
	ChunkDiffUnchanged ChunkDiffType = "Unchanged"
)

type ChunkDiff struct {
	Index   int           `json:"index"`
	Type    ChunkDiffType `json:"type"`
	OldText string        `json:"oldText,omitempty"`
	NewText string        `json:"newText,omitempty"`
	OldHash string        `json:"oldHash,omitempty"`
	NewHash string        `json:"newHash,omitempty"`
}

type FileVersionDiff struct {
	Owner       string `xorm:"varchar(100) notnull pk" json:"owner"`
	Name        string `xorm:"varchar(512) notnull pk" json:"name"`
	Version     int    `xorm:"notnull pk" json:"version"`
	CreatedTime string `xorm:"varchar(100)" json:"createdTime"`
	JobId       string `xorm:"varchar(100)" json:"jobId"`

	FromParseVersion int    `json:"fromParseVersion"`
	ToParseVersion   int    `json:"toParseVersion"`
	FromContentHash  string `xorm:"varchar(64)" json:"fromContentHash"`
	ToContentHash    string `xorm:"varchar(64)" json:"toContentHash"`
	FromChunkHash    string `xorm:"varchar(64)" json:"fromChunkHash"`
	ToChunkHash      string `xorm:"varchar(64)" json:"toChunkHash"`

	AddedCount     int `json:"addedCount"`
	DeletedCount   int `json:"deletedCount"`
	ModifiedCount  int `json:"modifiedCount"`
	UnchangedCount int `json:"unchangedCount"`

	AddedChunks    []ChunkDiff `xorm:"mediumtext" json:"addedChunks"`
	DeletedChunks  []ChunkDiff `xorm:"mediumtext" json:"deletedChunks"`
	ModifiedChunks []ChunkDiff `xorm:"mediumtext" json:"modifiedChunks"`

	Status                 FileStatus `xorm:"varchar(100)" json:"status"`
	ParseError             string     `xorm:"mediumtext" json:"parseError,omitempty"`
	DeleteVectorsError     string     `xorm:"mediumtext" json:"deleteVectorsError,omitempty"`
	VectorGenerationErrors []string   `xorm:"mediumtext" json:"vectorGenerationErrors,omitempty"`
	FailedChunkIndices     []int      `xorm:"mediumtext" json:"failedChunkIndices,omitempty"`
	ErrorText              string     `xorm:"mediumtext" json:"errorText"`
	ObsoleteReason         string     `xorm:"mediumtext" json:"obsoleteReason,omitempty"`
	VectorsWritten         bool       `json:"vectorsWritten"`
	OrphanVectorsCleaned   bool       `json:"orphanVectorsCleaned"`
}

type FileParseVersion struct {
	Owner       string `xorm:"varchar(100) notnull pk" json:"owner"`
	Name        string `xorm:"varchar(512) notnull pk" json:"name"`
	Version     int    `xorm:"notnull pk" json:"version"`
	CreatedTime string `xorm:"varchar(100)" json:"createdTime"`
	JobId       string `xorm:"varchar(100)" json:"jobId"`

	ContentHash   string            `xorm:"varchar(64)" json:"contentHash"`
	ChunkHash     string            `xorm:"varchar(64)" json:"chunkHash"`
	VectorVersion int               `json:"vectorVersion"`
	ChunkCount    int               `json:"chunkCount"`
	ChunkHashes   map[string]string `xorm:"mediumtext" json:"chunkHashes"`
	Status        FileStatus        `xorm:"varchar(100)" json:"status"`

	ParseError             string   `xorm:"mediumtext" json:"parseError,omitempty"`
	DeleteVectorsError     string   `xorm:"mediumtext" json:"deleteVectorsError,omitempty"`
	VectorGenerationErrors []string `xorm:"mediumtext" json:"vectorGenerationErrors,omitempty"`
	FailedChunkIndices     []int    `xorm:"mediumtext" json:"failedChunkIndices,omitempty"`
	ErrorText              string   `xorm:"mediumtext" json:"errorText"`
	ObsoleteReason         string   `xorm:"mediumtext" json:"obsoleteReason,omitempty"`
	VectorsWritten         bool     `json:"vectorsWritten"`
	OrphanVectorsCleaned   bool     `json:"orphanVectorsCleaned"`
}

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

	ParseVersion  int              `json:"parseVersion"`
	ContentHash   string           `xorm:"varchar(64)" json:"contentHash"`
	ChunkHash     string           `xorm:"varchar(64)" json:"chunkHash"`
	VectorVersion int              `json:"vectorVersion"`
	CurrentJobId  string           `xorm:"varchar(100)" json:"currentJobId"`
	LatestDiff    *FileVersionDiff `xorm:"-" json:"latestDiff,omitempty"`
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

	return files, nil
}

func GetFiles(owner string) ([]*File, error) {
	files := []*File{}
	err := adapter.engine.Desc("created_time").Find(&files, &File{Owner: owner})
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
	return getFile(owner, name)
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

	existingFile, _ := getFile(owner, getFileName(storeName, objectKey))

	fileRecord := &File{
		Owner:           owner,
		Name:            getFileName(storeName, objectKey),
		CreatedTime:     util.GetCurrentTime(),
		Filename:        filename,
		Size:            fileSize,
		Store:           storeName,
		StorageProvider: provider.Name,
		Url:             fileUrl,
		TokenCount:      0,
		Status:          FileStatusParsing,
		ParseVersion:    0,
		VectorVersion:   0,
	}

	if existingFile != nil {
		fileRecord.ParseVersion = existingFile.ParseVersion
		fileRecord.VectorVersion = existingFile.VectorVersion
		fileRecord.ContentHash = existingFile.ContentHash
		fileRecord.ChunkHash = existingFile.ChunkHash
	}

	var upserted bool
	if existingFile != nil {
		upserted, err = UpdateFile(fileRecord.GetId(), fileRecord)
	} else {
		upserted, err = AddFile(fileRecord)
	}
	if err != nil {
		return nil, err
	}
	if !upserted {
		return nil, fmt.Errorf("failed to save file record")
	}

	go func() {
		logs.Info("Starting async vector generation for uploaded file: store=%s, file=%s", storeName, objectKey)
		_, _, err := AddVectorsForFileIncremental(defaultStore, objectKey, fileUrl, lang)
		if err != nil {
			logs.Error("Async vector generation failed for file %s: %v", objectKey, err)
		}
	}()

	return fileRecord, nil
}

func calculateHash(text string) string {
	h := sha256.New()
	h.Write([]byte(text))
	return fmt.Sprintf("%x", h.Sum(nil))
}

func calculateChunksHash(chunks []string) string {
	h := sha256.New()
	for _, chunk := range chunks {
		h.Write([]byte(chunk))
		h.Write([]byte("\x00"))
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

func calculateChunkHashes(chunks []string) map[string]string {
	hashes := make(map[string]string, len(chunks))
	for i, chunk := range chunks {
		key := fmt.Sprintf("%d", i)
		hashes[key] = calculateHash(chunk)
	}
	return hashes
}

func getFileParseVersion(owner string, name string, version int) (*FileParseVersion, error) {
	pv := FileParseVersion{Owner: owner, Name: name, Version: version}
	existed, err := adapter.engine.Get(&pv)
	if err != nil {
		return nil, err
	}
	if existed {
		return &pv, nil
	}
	return nil, nil
}

func GetLatestFileParseVersion(owner string, name string) (*FileParseVersion, error) {
	var versions []*FileParseVersion
	err := adapter.engine.Where("owner = ? AND name = ?", owner, name).
		Desc("version").Limit(1).Find(&versions)
	if err != nil {
		return nil, err
	}
	if len(versions) > 0 {
		return versions[0], nil
	}
	return nil, nil
}

func AddFileParseVersion(pv *FileParseVersion) (bool, error) {
	affected, err := adapter.engine.Insert(pv)
	if err != nil {
		return false, err
	}
	return affected != 0, nil
}

func getFileVersionDiff(owner string, name string, version int) (*FileVersionDiff, error) {
	diff := FileVersionDiff{Owner: owner, Name: name, Version: version}
	existed, err := adapter.engine.Get(&diff)
	if err != nil {
		return nil, err
	}
	if existed {
		return &diff, nil
	}
	return nil, nil
}

func GetLatestFileVersionDiff(owner string, name string) (*FileVersionDiff, error) {
	var diffs []*FileVersionDiff
	err := adapter.engine.Where("owner = ? AND name = ?", owner, name).
		Desc("version").Limit(1).Find(&diffs)
	if err != nil {
		return nil, err
	}
	if len(diffs) > 0 {
		return diffs[0], nil
	}
	return nil, nil
}

func GetFileVersionDiffs(owner string, name string) ([]*FileVersionDiff, error) {
	var diffs []*FileVersionDiff
	err := adapter.engine.Where("owner = ? AND name = ?", owner, name).
		Desc("version").Find(&diffs)
	if err != nil {
		return nil, err
	}
	return diffs, nil
}

func GetFileParseVersions(owner string, name string) ([]*FileParseVersion, error) {
	var versions []*FileParseVersion
	err := adapter.engine.Where("owner = ? AND name = ?", owner, name).
		Desc("version").Find(&versions)
	if err != nil {
		return nil, err
	}
	return versions, nil
}

func AddFileVersionDiff(diff *FileVersionDiff) (bool, error) {
	affected, err := adapter.engine.Insert(diff)
	if err != nil {
		return false, err
	}
	return affected != 0, nil
}

func UpdateFileVersionDiff(diff *FileVersionDiff) (bool, error) {
	_, err := adapter.engine.ID(core.PK{diff.Owner, diff.Name, diff.Version}).AllCols().Update(diff)
	if err != nil {
		return false, err
	}
	return true, nil
}

func computeChunkDiff(oldChunks []string, newChunks []string) []ChunkDiff {
	oldHashes := make([]string, len(oldChunks))
	newHashes := make([]string, len(newChunks))
	for i, c := range oldChunks {
		oldHashes[i] = calculateHash(c)
	}
	for i, c := range newChunks {
		newHashes[i] = calculateHash(c)
	}

	diffs := []ChunkDiff{}
	i, j := 0, 0

	for i < len(oldHashes) && j < len(newHashes) {
		if oldHashes[i] == newHashes[j] {
			diffs = append(diffs, ChunkDiff{
				Index:   j,
				Type:    ChunkDiffUnchanged,
				OldText: oldChunks[i],
				NewText: newChunks[j],
				OldHash: oldHashes[i],
				NewHash: newHashes[j],
			})
			i++
			j++
			continue
		}

		nextMatchOld, nextMatchNew := findNextMatch(oldHashes, newHashes, i, j)

		if nextMatchOld == -1 && nextMatchNew == -1 {
			break
		}

		if nextMatchOld != -1 && (nextMatchNew == -1 || (nextMatchOld - i) <= (nextMatchNew - j)) {
			for k := i; k < nextMatchOld; k++ {
				diffs = append(diffs, ChunkDiff{
					Index:   j,
					Type:    ChunkDiffDeleted,
					OldText: oldChunks[k],
					OldHash: oldHashes[k],
				})
			}
			i = nextMatchOld
		} else if nextMatchNew != -1 {
			for k := j; k < nextMatchNew; k++ {
				diffs = append(diffs, ChunkDiff{
					Index:   k,
					Type:    ChunkDiffAdded,
					NewText: newChunks[k],
					NewHash: newHashes[k],
				})
			}
			j = nextMatchNew
		}
	}

	for i < len(oldHashes) {
		diffs = append(diffs, ChunkDiff{
			Index:   j,
			Type:    ChunkDiffDeleted,
			OldText: oldChunks[i],
			OldHash: oldHashes[i],
		})
		i++
	}

	for j < len(newHashes) {
		diffs = append(diffs, ChunkDiff{
			Index:   j,
			Type:    ChunkDiffAdded,
			NewText: newChunks[j],
			NewHash: newHashes[j],
		})
		j++
	}

	sort.SliceStable(diffs, func(i, j int) bool {
		return diffs[i].Index < diffs[j].Index
	})

	return diffs
}

func findNextMatch(oldHashes, newHashes []string, i, j int) (int, int) {
	lookahead := 20

	oldHashToIndices := make(map[string][]int)
	endOld := i + lookahead
	if endOld > len(oldHashes) {
		endOld = len(oldHashes)
	}
	for k := i; k < endOld; k++ {
		oldHashToIndices[oldHashes[k]] = append(oldHashToIndices[oldHashes[k]], k)
	}

	endNew := j + lookahead
	if endNew > len(newHashes) {
		endNew = len(newHashes)
	}

	bestOld, bestNew := -1, -1
	for k := j; k < endNew; k++ {
		if oldIndices, ok := oldHashToIndices[newHashes[k]]; ok {
			for _, oldIdx := range oldIndices {
				if oldIdx >= i {
					if bestOld == -1 || (oldIdx - i) + (k - j) < (bestOld - i) + (bestNew - j) {
						bestOld = oldIdx
						bestNew = k
					}
					break
				}
			}
		}
	}

	return bestOld, bestNew
}

func summarizeDiff(diffs []ChunkDiff, maxSamples int) (added, deleted, modified, unchanged []ChunkDiff) {
	for _, d := range diffs {
		switch d.Type {
		case ChunkDiffAdded:
			if len(added) < maxSamples {
				added = append(added, d)
			}
		case ChunkDiffDeleted:
			if len(deleted) < maxSamples {
				deleted = append(deleted, d)
			}
		case ChunkDiffModified:
			if len(modified) < maxSamples {
				modified = append(modified, d)
			}
		case ChunkDiffUnchanged:
			if len(unchanged) < maxSamples {
				unchanged = append(unchanged, d)
			}
		}
	}
	return added, deleted, modified, unchanged
}

func generateJobId() string {
	return fmt.Sprintf("job_%s_%s", util.GetCurrentTime(), util.GetRandomName())
}

func AcquireFileJob(owner, fileName string) (string, int, error) {
	file, err := getFile(owner, fileName)
	if err != nil {
		return "", 0, err
	}

	jobId := generateJobId()
	newVersion := 1
	if file != nil {
		newVersion = file.ParseVersion + 1
	}

	if file == nil {
		return jobId, newVersion, nil
	}

	file.CurrentJobId = jobId
	_, err = UpdateFile(file.GetId(), file)
	if err != nil {
		return "", 0, err
	}

	return jobId, newVersion, nil
}

func IsFileJobCurrent(owner, fileName, jobId string) (bool, *File, error) {
	file, err := getFile(owner, fileName)
	if err != nil {
		return false, nil, err
	}
	if file == nil {
		return true, nil, nil
	}
	if file.CurrentJobId == "" {
		return true, file, nil
	}
	return file.CurrentJobId == jobId, file, nil
}

func MarkJobObsolete(owner, fileName string, version int, jobId, reason string, vectorsWritten bool) error {
	diff, err := getFileVersionDiff(owner, fileName, version)
	if err != nil {
		return err
	}
	if diff == nil {
		return nil
	}
	diff.Status = FileStatusObsolete
	diff.ObsoleteReason = reason
	diff.JobId = jobId
	diff.VectorsWritten = vectorsWritten || diff.VectorsWritten

	if diff.VectorsWritten {
		file, fileErr := getFile(owner, fileName)
		if fileErr == nil && file != nil {
			objectKey := getObjectKeyFromFile(file)
			deleted, delErr := DeleteVectorsByFileAndParseVersion(owner, file.Store, objectKey, version)
			if delErr != nil {
				logs.Error("Failed to clean orphan vectors for obsolete job [%s], file [%s], version %d: %v", jobId, fileName, version, delErr)
				diff.OrphanVectorsCleaned = false
			} else {
				if deleted > 0 {
					logs.Info("Cleaned %d orphan vectors for obsolete job [%s], file [%s], version %d", deleted, jobId, fileName, version)
				}
				diff.OrphanVectorsCleaned = true
			}
		}
	}

	_, err = UpdateFileVersionDiff(diff)
	return err
}

func getObjectKeyFromFile(file *File) string {
	prefix := fmt.Sprintf("%s_", file.Store)
	if strings.HasPrefix(file.Name, prefix) {
		return strings.TrimPrefix(file.Name, prefix)
	}
	return file.Name
}

func CompareAndSwapFileVersion(owner, fileName, jobId string, expectedParseVersion int, updateFunc func(*File) error) (bool, error) {
	file, err := getFile(owner, fileName)
	if err != nil {
		return false, err
	}
	if file == nil {
		return false, fmt.Errorf("file not found")
	}

	if file.CurrentJobId != jobId {
		return false, nil
	}
	if expectedParseVersion > 0 && file.ParseVersion != expectedParseVersion-1 {
		return false, nil
	}

	err = updateFunc(file)
	if err != nil {
		return false, err
	}

	_, err = UpdateFile(file.GetId(), file)
	if err != nil {
		return false, err
	}
	return true, nil
}

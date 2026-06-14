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

package controllers

import (
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/beego/beego/logs"
	"github.com/the-open-agent/openagent/object"
	"github.com/the-open-agent/openagent/txt"
)

func decodeFileBase64(fileBase64 string) ([]byte, error) {
	data := strings.TrimSpace(fileBase64)

	if data == "" {
		return nil, fmt.Errorf("file data is empty")
	}

	if idx := strings.Index(data, ","); idx != -1 {
		data = data[idx+1:]
		data = strings.TrimSpace(data)
	}

	data = strings.ReplaceAll(data, "\n", "")
	data = strings.ReplaceAll(data, "\r", "")
	data = strings.ReplaceAll(data, "\t", "")
	data = strings.ReplaceAll(data, " ", "")

	if l := len(data) % 4; l != 0 {
		data += strings.Repeat("=", 4-l)
	}

	var dec []byte
	var err error

	dec, err = base64.StdEncoding.DecodeString(data)
	if err != nil {
		dec, err = base64.URLEncoding.DecodeString(data)
	}
	if err != nil {
		dec, err = base64.RawStdEncoding.DecodeString(strings.TrimRight(data, "="))
	}
	if err != nil {
		dec, err = base64.RawURLEncoding.DecodeString(strings.TrimRight(data, "="))
	}
	if err != nil {
		return nil, fmt.Errorf("invalid base64 data: %v", err)
	}

	if len(dec) == 0 {
		return nil, fmt.Errorf("decoded file data is empty")
	}

	return dec, nil
}

// UploadTaskDocument
// @Title UploadTaskDocument
// @Tag Task API
// @Description upload document for a task and parse its text
// @Param id query string true "The id (owner/name) of the task"
// @Param file formData string true "The base64 encoded file data"
// @Param type formData string true "The file type/extension"
// @Param name formData string true "The file name"
// @Success 200 {object} controllers.Response The Response object
// @router /upload-task-document [post]
func (c *ApiController) UploadTaskDocument() {
	userName, ok := c.RequireSignedIn()
	if !ok {
		return
	}

	taskId := c.Input().Get("id")
	fileBase64 := c.GetString("file")
	fileMimeType := c.GetString("type")
	fileName := c.GetString("name")

	if taskId == "" || fileBase64 == "" || fileName == "" {
		c.ResponseError(c.T("application:Missing required parameters"))
		return
	}

	task, err := object.GetTask(taskId)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}
	if task == nil {
		c.ResponseError(c.T("general:The task does not exist"))
		return
	}

	if !c.IsAdmin() {
		if task.Owner != userName {
			c.ResponseError(c.T("auth:Unauthorized operation"))
			return
		}
	}

	task.ResetDocumentFields()

	allowedExtensions := []string{".docx", ".pdf"}
	typeDetection := txt.DetectTaskDocumentType(fileName, fileMimeType, allowedExtensions)

	// Phase 1: Decode base64 - only THIS is a true upload failure (no storage, no resource)
	fileBytes, err := decodeFileBase64(fileBase64)
	if err != nil {
		syncErr := object.SyncDocumentStatus(taskId, task, nil, &object.DocumentParseResult{
			Status: object.DocumentParseStatusFailed,
			Error:  fmt.Sprintf("无效的文件数据格式: %v", err),
		})
		if syncErr != nil {
			logs.Warning("Failed to sync document status for base64 decode error: %v", syncErr)
		}
		c.ResponseError(c.T("resource:Invalid file data format"))
		return
	}
	fileSize := int64(len(fileBytes))

	// Phase 2: Upload to storage - only THIS is a true upload failure (no resource yet)
	safeFileName := strings.ReplaceAll(fileName, "+", "_")
	filePath := fmt.Sprintf("openagent/task-documents/%s/%s", userName, safeFileName)
	host := c.Ctx.Request.Host
	origin := getOriginFromHost(host)
	fileUrl, err := object.UploadFileToStorageSafe(filePath, fileBytes, origin, c.GetAcceptLanguage())
	if err != nil {
		syncErr := object.SyncDocumentStatus(taskId, task, nil, &object.DocumentParseResult{
			Status: object.DocumentParseStatusFailed,
			Error:  fmt.Sprintf("文件上传失败: %v", err),
		})
		if syncErr != nil {
			logs.Warning("Failed to sync document status for storage upload error: %v", syncErr)
		}
		c.ResponseError(err.Error())
		return
	}

	// Phase 3: Create Resource record (always succeeds from here: decode + storage worked)
	detectedType := typeDetection.DetectedType
	if detectedType == "" {
		detectedType = typeDetection.FileNameExt
	}
	if detectedType == "" {
		detectedType = typeDetection.MimeTypeExt
	}
	resource := object.NewResourceFromUploadWithMetadata(
		"admin", userName, "document", fileName, fileMimeType,
		"application", detectedType,
		fileUrl, filePath, fileSize, "task", taskId,
		object.DocumentParseStatusPending, "",
	)
	resourceId := ""
	if _, addErr := object.AddResource(resource); addErr != nil {
		logs.Warning("Failed to save resource record for task document: %v", addErr)
	} else {
		resourceId = resource.GetId()
	}

	// Phase 4: Determine parse result
	var parseResult *object.DocumentParseResult

	if typeDetection.Unsupported {
		parseResult = &object.DocumentParseResult{
			Status: object.DocumentParseStatusUnsupported,
			Error:  typeDetection.UnsupportedReason,
		}
	} else {
		parsedText, parseErr := txt.GetParsedTextFromUrl(fileUrl, detectedType, c.GetAcceptLanguage())
		if parseErr != nil {
			logs.Error("Failed to parse text from %s: %v", fileUrl, parseErr)
			parseResult = &object.DocumentParseResult{
				Status: object.DocumentParseStatusFailed,
				Error:  fmt.Sprintf("文档解析失败: %v", parseErr),
			}
		} else if strings.TrimSpace(parsedText) == "" {
			parseResult = &object.DocumentParseResult{
				Status: object.DocumentParseStatusEmpty,
				Error:  "文档已上传但未提取到文本内容，可能是扫描件或空文档",
			}
		} else {
			parseResult = &object.DocumentParseResult{
				Status: object.DocumentParseStatusSuccess,
				Text:   parsedText,
			}
		}
	}

	// Phase 5: Sync status to both Task and Resource atomically
	uploadInfo := &object.DocumentUploadInfo{
		FileName:     fileName,
		MimeType:     fileMimeType,
		FileType:     detectedType,
		FileUrl:      fileUrl,
		ResourceId:   resourceId,
		FileSize:     fileSize,
		TypeSource:   typeDetection.Source,
		TypeConflict: typeDetection.Conflict,
		ConflictMsg:  typeDetection.ConflictMessage,
	}
	syncErr := object.SyncDocumentStatus(taskId, task, uploadInfo, parseResult)
	if syncErr != nil {
		c.ResponseError(syncErr.Error())
		return
	}

	// Phase 6: Return unified response
	resp := task.BuildDocumentStatusResponse(&object.DocumentTypeDetectionExtra{
		FileNameExt: typeDetection.FileNameExt,
		MimeTypeExt: typeDetection.MimeTypeExt,
	})
	c.ResponseOk(resp)
}

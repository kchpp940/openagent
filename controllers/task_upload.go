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
		task.SetDocumentUploadError(fmt.Sprintf("无效的文件数据格式: %v", err))
		_, updateErr := object.UpdateTask(taskId, task)
		if updateErr != nil {
			logs.Warning("Failed to update task with base64 decode error: %v", updateErr)
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
		task.SetDocumentUploadError(fmt.Sprintf("文件上传失败: %v", err))
		_, updateErr := object.UpdateTask(taskId, task)
		if updateErr != nil {
			logs.Warning("Failed to update task with storage upload error: %v", updateErr)
		}
		c.ResponseError(err.Error())
		return
	}

	// Phase 3: Create Resource record (always succeeds from here: decode + storage worked)
	// At this point, the file is in storage, so we always report uploadSuccess=true.
	// We start with parseStatus=pending for all files.
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
		fileUrl, filePath, int(fileSize), "task", taskId,
		object.DocumentParseStatusPending, "",
	)
	resourceId := ""
	if _, addErr := object.AddResource(resource); addErr != nil {
		logs.Warning("Failed to save resource record for task document: %v", addErr)
	} else {
		resourceId = resource.GetId()
	}

	// Phase 4: Save upload success info to task (uploadSuccess=true from here onwards)
	task.SetDocumentUploadSuccess(
		fileName, fileMimeType, detectedType,
		fileUrl, resourceId, fileSize,
	)
	task.DocumentTypeSource = typeDetection.Source
	task.DocumentTypeConflict = typeDetection.Conflict
	task.DocumentConflictMsg = typeDetection.ConflictMessage

	// Phase 5: Determine parse status based on type support and parser result
	// NOTE: Even if type is unsupported, the file was still UPLOADED successfully.
	// We just mark parseStatus=unsupported instead of trying to parse.
	finalParseStatus := object.DocumentParseStatusSuccess
	finalParseError := ""
	documentText := ""

	if typeDetection.Unsupported {
		finalParseStatus = object.DocumentParseStatusUnsupported
		finalParseError = typeDetection.UnsupportedReason
	} else {
		parsedText, parseErr := txt.GetParsedTextFromUrl(fileUrl, detectedType, c.GetAcceptLanguage())
		if parseErr != nil {
			logs.Error("Failed to parse text from %s: %v", fileUrl, parseErr)
			finalParseStatus = object.DocumentParseStatusFailed
			finalParseError = fmt.Sprintf("文档解析失败: %v", parseErr)
		} else if strings.TrimSpace(parsedText) == "" {
			finalParseStatus = object.DocumentParseStatusEmpty
			finalParseError = "文档已上传但未提取到文本内容，可能是扫描件或空文档"
		} else {
			finalParseStatus = object.DocumentParseStatusSuccess
			documentText = parsedText
		}
	}

	// Phase 6: Apply final parse status to both Task and Resource
	switch finalParseStatus {
	case object.DocumentParseStatusSuccess:
		task.SetDocumentParseSuccess(documentText)
	case object.DocumentParseStatusFailed:
		task.SetDocumentParseFailed(finalParseError)
	case object.DocumentParseStatusEmpty:
		task.SetDocumentParseEmpty(finalParseError)
	case object.DocumentParseStatusUnsupported:
		task.SetDocumentUnsupported(finalParseError)
	}

	// Sync parse status back to resource for traceability
	if resourceId != "" {
		_, updateResErr := object.UpdateResourceParseStatus(resourceId, finalParseStatus, finalParseError)
		if updateResErr != nil {
			logs.Warning("Failed to update resource parse status: %v", updateResErr)
		}
	}

	// Re-apply type detection fields (SetDocument* may not preserve them for unsupported)
	task.DocumentFileType = detectedType
	task.DocumentTypeSource = typeDetection.Source
	task.DocumentTypeConflict = typeDetection.Conflict
	task.DocumentConflictMsg = typeDetection.ConflictMessage

	// Phase 7: Persist and return unified response
	_, err = object.UpdateTask(taskId, task)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	resp := task.BuildDocumentStatusResponse(&object.DocumentTypeDetectionExtra{
		FileNameExt: typeDetection.FileNameExt,
		MimeTypeExt: typeDetection.MimeTypeExt,
	})
	c.ResponseOk(resp)
}

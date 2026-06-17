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
	"fmt"
	"strings"

	"github.com/beego/beego/logs"
	"github.com/the-open-agent/openagent/object"
	"github.com/the-open-agent/openagent/txt"
)

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
	fileType := c.GetString("type")
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

	allowedExtensions := []string{".docx", ".pdf"}
	typeDetection := txt.DetectTaskDocumentType(fileName, fileType, allowedExtensions)

	if typeDetection.Unsupported {
		c.ResponseError(typeDetection.UnsupportedReason)
		return
	}

	host := c.Ctx.Request.Host
	origin := getOriginFromHost(host)
	lang := c.GetAcceptLanguage()

	uploadOpts := object.UploadOptions{
		FileName:          fileName,
		AllowedExtensions: allowedExtensions,
		StoragePathPrefix: fmt.Sprintf("openagent/task-documents/%s", userName),
		AddRandomSuffix:   false,
		Origin:            origin,
		Lang:              lang,
		User:              userName,
	}

	uploadResult, err := object.UploadFromBase64(fileBase64, uploadOpts)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	fileUrl := uploadResult.Url
	filePath := uploadResult.StorageKey
	fileSize := int(uploadResult.FileSize)

	resource := object.NewResourceFromUpload("admin", userName, "document", fileName, "application", typeDetection.DetectedType, fileUrl, filePath, fileSize, "task", taskId)
	if _, addErr := object.AddResource(resource); addErr != nil {
		logs.Warning("Failed to save resource record for task document: %v", addErr)
	}

	task.DocumentUrl = fileUrl
	task.DocumentFileType = typeDetection.DetectedType
	task.DocumentTypeSource = typeDetection.Source
	task.DocumentTypeConflict = typeDetection.Conflict
	task.DocumentConflictMsg = typeDetection.ConflictMessage
	task.DocumentError = ""
	task.DocumentText = ""
	task.DocumentParseStatus = object.DocumentParseStatusPending
	task.AnalyzeError = ""

	documentText, parseErr := txt.GetParsedTextFromUrl(fileUrl, typeDetection.DetectedType, c.GetAcceptLanguage())
	if parseErr != nil {
		logs.Error("Failed to parse text from %s: %v", fileUrl, parseErr)
		task.DocumentParseStatus = object.DocumentParseStatusFailed
		task.DocumentError = fmt.Sprintf("文档解析失败: %v", parseErr)
	} else if strings.TrimSpace(documentText) == "" {
		task.DocumentParseStatus = object.DocumentParseStatusEmpty
		task.DocumentError = "文档已上传但未提取到文本内容，可能是扫描件或空文档"
	} else {
		task.DocumentParseStatus = object.DocumentParseStatusSuccess
		task.DocumentText = documentText
	}

	success, err := object.UpdateTask(taskId, task)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	if !success {
		c.ResponseError(c.T("general:Failed to update"))
		return
	}

	result := map[string]interface{}{
		"url":             fileUrl,
		"text":            task.DocumentText,
		"parseStatus":     task.DocumentParseStatus,
		"error":           task.DocumentError,
		"fileType":        task.DocumentFileType,
		"typeSource":      task.DocumentTypeSource,
		"typeConflict":    task.DocumentTypeConflict,
		"conflictMessage": task.DocumentConflictMsg,
		"fileNameExt":     typeDetection.FileNameExt,
		"mimeTypeExt":     typeDetection.MimeTypeExt,
		"parseSuccess":    task.DocumentParseStatus == object.DocumentParseStatusSuccess,
		"fileName":        uploadResult.FileName,
		"fileSize":        uploadResult.FileSize,
		"fileFormat":      uploadResult.FileFormat,
		"mimeType":        uploadResult.MimeType,
		"storageKey":      uploadResult.StorageKey,
	}
	c.ResponseOk(result)
}

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
	"mime"
	"path/filepath"
	"strings"

	"github.com/beego/beego/logs"
	"github.com/the-open-agent/openagent/object"
	"github.com/the-open-agent/openagent/txt"
)

func detectFileExtension(fileName, fileType string, supportedTypes []string) string {
	ext := strings.ToLower(filepath.Ext(fileName))

	extFromMime := ""
	if fileType != "" {
		exts, _ := mime.ExtensionsByType(fileType)
		if len(exts) > 0 {
			extFromMime = strings.ToLower(exts[0])
		}
	}

	if ext == "" && extFromMime != "" {
		ext = extFromMime
	}

	for _, supported := range supportedTypes {
		if ext == supported {
			return ext
		}
	}

	if extFromMime != "" {
		for _, supported := range supportedTypes {
			if extFromMime == supported {
				return extFromMime
			}
		}
	}

	return ""
}

func decodeFileBase64(fileBase64 string) ([]byte, error) {
	data := fileBase64

	if idx := strings.Index(data, ","); idx != -1 {
		data = data[idx+1:]
	}

	data = strings.TrimSpace(data)

	dec, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		dec, err = base64.URLEncoding.DecodeString(data)
		if err != nil {
			dec, err = base64.RawStdEncoding.DecodeString(data)
			if err != nil {
				dec, err = base64.RawURLEncoding.DecodeString(data)
				if err != nil {
					return nil, fmt.Errorf("invalid base64 data: %v", err)
				}
			}
		}
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

	supportedTypes := txt.GetSupportedFileTypes()
	allowedExtensions := []string{".docx", ".pdf"}
	ext := detectFileExtension(fileName, fileType, supportedTypes)

	isValid := false
	for _, allowed := range allowedExtensions {
		if ext == allowed {
			isValid = true
			break
		}
	}
	if !isValid {
		c.ResponseError(c.T("resource:Only docx and pdf files are allowed"))
		return
	}

	fileBytes, err := decodeFileBase64(fileBase64)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	safeFileName := strings.ReplaceAll(fileName, "+", "_")
	filePath := fmt.Sprintf("openagent/task-documents/%s/%s", userName, safeFileName)
	host := c.Ctx.Request.Host
	origin := getOriginFromHost(host)
	fileUrl, err := object.UploadFileToStorageSafe(filePath, fileBytes, origin, c.GetAcceptLanguage())
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	resource := object.NewResourceFromUpload("admin", userName, "document", fileName, "application", ext, fileUrl, filePath, len(fileBytes), "task", taskId)
	if _, addErr := object.AddResource(resource); addErr != nil {
		logs.Warning("Failed to save resource record for task document: %v", addErr)
	}

	task.DocumentUrl = fileUrl
	task.DocumentFileType = ext
	task.DocumentError = ""
	task.DocumentText = ""

	documentText, err := txt.GetParsedTextFromUrl(fileUrl, ext, c.GetAcceptLanguage())
	if err != nil {
		logs.Error("Failed to parse text from %s: %v", fileUrl, err)
		task.DocumentError = fmt.Sprintf("文档解析失败: %v", err)
	} else {
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
		"url":          fileUrl,
		"text":         task.DocumentText,
		"error":        task.DocumentError,
		"fileType":     task.DocumentFileType,
		"parseSuccess": task.DocumentError == "" && task.DocumentText != "",
	}
	c.ResponseOk(result)
}

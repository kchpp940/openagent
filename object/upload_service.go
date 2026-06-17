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

package object

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"path/filepath"
	"strings"

	"github.com/the-open-agent/openagent/i18n"
	"github.com/the-open-agent/openagent/storage"
	"github.com/the-open-agent/openagent/util"
)

type UploadResult struct {
	FileName    string `json:"fileName"`
	FileSize    int64  `json:"fileSize"`
	FileType    string `json:"fileType"`
	FileFormat  string `json:"fileFormat"`
	MimeType    string `json:"mimeType"`
	StorageKey  string `json:"storageKey"`
	Url         string `json:"url"`
}

type UploadOptions struct {
	FileName           string
	AllowedExtensions  []string
	MaxFileSize        int64
	StoragePathPrefix  string
	FullStorageKey     string
	AddRandomSuffix    bool
	Origin             string
	Lang               string
	StorageProvider    storage.StorageProvider
	User               string
	Parent             string
}

func DefaultUploadOptions() UploadOptions {
	return UploadOptions{
		MaxFileSize:     50 * 1024 * 1024,
		Lang:            "en",
		AddRandomSuffix: true,
	}
}

func decodeBase64File(fileBase64 string) ([]byte, error) {
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

func sanitizeFileName(fileName string) string {
	safe := strings.ReplaceAll(fileName, "+", "_")
	safe = strings.ReplaceAll(safe, " ", "_")
	safe = strings.ReplaceAll(safe, "/", "_")
	safe = strings.ReplaceAll(safe, "\\", "_")
	return safe
}

func detectMimeType(fileName string, fileBytes []byte) string {
	ext := strings.ToLower(filepath.Ext(fileName))
	if mimeType := mime.TypeByExtension(ext); mimeType != "" {
		return mimeType
	}
	return "application/octet-stream"
}

func getFileTypeFromMimeType(mimeType string) string {
	parts := strings.SplitN(mimeType, "/", 2)
	if len(parts) > 0 {
		return parts[0]
	}
	return "unknown"
}

func validateFileExtension(fileName string, allowedExtensions []string, lang string) error {
	ext := strings.ToLower(filepath.Ext(fileName))

	legacyOfficeExtensions := map[string]bool{
		".doc": true, ".ppt": true, ".xls": true,
		".dot": true, ".pot": true, ".xlt": true,
		".pps": true,
	}
	if legacyOfficeExtensions[ext] {
		tmpl := i18n.Translate(lang, "resource:Unsupported legacy file format %s, please convert to a modern format (e.g. .docx, .pptx, .xlsx) before uploading")
		return fmt.Errorf("%s", fmt.Sprintf(tmpl, ext))
	}

	if len(allowedExtensions) > 0 {
		allowed := false
		for _, allowedExt := range allowedExtensions {
			if strings.EqualFold(ext, allowedExt) {
				allowed = true
				break
			}
		}
		if !allowed {
			return fmt.Errorf(i18n.Translate(lang, "upload:File extension %s is not allowed"), ext)
		}
	}

	return nil
}

func generateStorageKey(prefix string, fileName string, addRandomSuffix bool) string {
	safeName := sanitizeFileName(fileName)
	ext := filepath.Ext(safeName)
	base := strings.TrimSuffix(safeName, ext)

	var finalName string
	if addRandomSuffix {
		randomName := util.GetRandomName()
		finalName = fmt.Sprintf("%s_%s%s", base, randomName, ext)
	} else {
		finalName = safeName
	}

	if prefix == "" {
		return finalName
	}

	cleanPrefix := strings.Trim(prefix, "/")
	return fmt.Sprintf("%s/%s", cleanPrefix, finalName)
}

func UploadFromBase64(fileBase64 string, opts UploadOptions) (*UploadResult, error) {
	fileBytes, err := decodeBase64File(fileBase64)
	if err != nil {
		return nil, err
	}

	return uploadFile(fileBytes, opts)
}

func UploadFromMultipartFile(file multipart.File, header *multipart.FileHeader, opts UploadOptions) (*UploadResult, error) {
	if opts.FileName == "" && header != nil {
		opts.FileName = header.Filename
	}

	fileBytes := make([]byte, header.Size)
	_, err := file.Read(fileBytes)
	if err != nil {
		return nil, err
	}

	return uploadFile(fileBytes, opts)
}

func UploadFromBytes(fileBytes []byte, opts UploadOptions) (*UploadResult, error) {
	return uploadFile(fileBytes, opts)
}

func uploadFile(fileBytes []byte, opts UploadOptions) (*UploadResult, error) {
	if opts.FileName == "" {
		return nil, fmt.Errorf(i18n.Translate(opts.Lang, "upload:File name is required"))
	}

	fileSize := int64(len(fileBytes))
	if opts.MaxFileSize > 0 && fileSize > opts.MaxFileSize {
		return nil, fmt.Errorf(i18n.Translate(opts.Lang, "upload:File size exceeds the maximum allowed size of %d bytes"), opts.MaxFileSize)
	}

	if err := validateFileExtension(opts.FileName, opts.AllowedExtensions, opts.Lang); err != nil {
		return nil, err
	}

	mimeType := detectMimeType(opts.FileName, fileBytes)
	fileType := getFileTypeFromMimeType(mimeType)
	fileFormat := strings.ToLower(filepath.Ext(opts.FileName))

	var storageKey string
	if opts.FullStorageKey != "" {
		storageKey = opts.FullStorageKey
	} else {
		storageKey = generateStorageKey(opts.StoragePathPrefix, opts.FileName, opts.AddRandomSuffix)
	}

	var storageProvider storage.StorageProvider
	if opts.StorageProvider != nil {
		storageProvider = opts.StorageProvider
	} else {
		provider, err := GetDefaultStorageProvider()
		if err != nil {
			return nil, err
		}
		if provider == nil {
			return nil, fmt.Errorf("no default storage provider configured")
		}
		storageProvider, err = provider.GetStorageProviderObj("", opts.Lang)
		if err != nil {
			return nil, err
		}
	}

	fileBuffer := bytes.NewBuffer(fileBytes)
	user := opts.User
	if user == "" {
		user = "admin"
	}
	parent := opts.Parent
	if parent == "" {
		parent = ""
	}
	rawUrl, err := storageProvider.PutObject(user, parent, storageKey, fileBuffer)
	if err != nil {
		return nil, err
	}

	fileUrl := rawUrl
	if opts.Origin != "" {
		adjustedUrl, err := getUrlFromPath(rawUrl, opts.Origin)
		if err != nil {
			return nil, err
		}
		fileUrl = adjustedUrl
	}

	return &UploadResult{
		FileName:   opts.FileName,
		FileSize:   fileSize,
		FileType:   fileType,
		FileFormat: fileFormat,
		MimeType:   mimeType,
		StorageKey: storageKey,
		Url:        fileUrl,
	}, nil
}

func UploadFromReader(reader io.Reader, opts UploadOptions) (*UploadResult, error) {
	fileBuffer := bytes.NewBuffer(nil)
	_, err := io.Copy(fileBuffer, reader)
	if err != nil {
		return nil, err
	}

	return uploadFile(fileBuffer.Bytes(), opts)
}

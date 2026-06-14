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

package txt

import (
	"fmt"
	"mime"
	"os"
	"path/filepath"
	"strings"

	"github.com/beego/beego/logs"
	"github.com/the-open-agent/openagent/i18n"
)

const (
	TypeSourceFileName   = "filename"
	TypeSourceMimeType   = "mimeType"
	TypeSourceFallback   = "fallback"
)

type DocumentTypeDetection struct {
	DetectedType       string `json:"detectedType"`
	Source             string `json:"source"`
	FileNameExt        string `json:"fileNameExt"`
	MimeTypeExt        string `json:"mimeTypeExt"`
	Conflict           bool   `json:"conflict"`
	ConflictMessage    string `json:"conflictMessage"`
	Unsupported        bool   `json:"unsupported"`
	UnsupportedReason  string `json:"unsupportedReason"`
}

func GetSupportedFileTypes() []string {
	return []string{".txt", ".md", ".yaml", ".csv", ".pdf", ".docx", ".xlsx", ".pptx"}
}

func GetParserSupportedTypes() map[string]bool {
	return map[string]bool{
		".txt":  true,
		".md":   true,
		".yaml": true,
		".csv":  true,
		".pdf":  true,
		".docx": true,
		".xlsx": true,
		".pptx": true,
	}
}

var mimeTypeToExtension = map[string]string{
	"application/pdf":                     ".pdf",
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document":   ".docx",
	"application/msword":                  ".docx",
	"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":         ".xlsx",
	"application/vnd.ms-excel":            ".xlsx",
	"application/vnd.openxmlformats-officedocument.presentationml.presentation": ".pptx",
	"application/vnd.ms-powerpoint":       ".pptx",
	"text/plain":                          ".txt",
	"text/markdown":                       ".md",
	"text/csv":                            ".csv",
	"text/yaml":                           ".yaml",
	"application/x-yaml":                  ".yaml",
	"application/yaml":                    ".yaml",
	"application/octet-stream":            "",
}

func isSupportedType(ext string) bool {
	supported := GetParserSupportedTypes()
	return supported[strings.ToLower(ext)]
}

func DetectTaskDocumentType(fileName, mimeType string, allowedExtensions []string) *DocumentTypeDetection {
	result := &DocumentTypeDetection{}

	result.FileNameExt = strings.ToLower(filepath.Ext(fileName))

	if mimeType != "" {
		mimeTypeNormalized := strings.TrimSpace(strings.ToLower(mimeType))
		if ext, ok := mimeTypeToExtension[mimeTypeNormalized]; ok && ext != "" {
			result.MimeTypeExt = ext
		} else {
			exts, _ := mime.ExtensionsByType(mimeType)
			if len(exts) > 0 {
				for _, ext := range exts {
					extLower := strings.ToLower(ext)
					if isSupportedType(extLower) {
						result.MimeTypeExt = extLower
						break
					}
				}
				if result.MimeTypeExt == "" {
					result.MimeTypeExt = strings.ToLower(exts[0])
				}
			}
		}
	}

	allowedMap := map[string]bool{}
	for _, ext := range allowedExtensions {
		allowedMap[strings.ToLower(ext)] = true
	}

	if result.FileNameExt != "" && result.MimeTypeExt != "" && result.FileNameExt != result.MimeTypeExt {
		extSupported := isSupportedType(result.FileNameExt)
		mimeSupported := isSupportedType(result.MimeTypeExt)
		extAllowed := len(allowedExtensions) == 0 || allowedMap[result.FileNameExt]
		mimeAllowed := len(allowedExtensions) == 0 || allowedMap[result.MimeTypeExt]

		if extSupported && extAllowed && (!mimeSupported || !mimeAllowed) {
			result.DetectedType = result.FileNameExt
			result.Source = TypeSourceFileName
			result.Conflict = true
			result.ConflictMessage = fmt.Sprintf(
				"文件名后缀 %s 与 MIME 类型 %s (解析为 %s) 不一致，已按文件名后缀处理",
				result.FileNameExt, mimeType, result.MimeTypeExt,
			)
		} else if mimeSupported && mimeAllowed && (!extSupported || !extAllowed) {
			result.DetectedType = result.MimeTypeExt
			result.Source = TypeSourceMimeType
			result.Conflict = true
			result.ConflictMessage = fmt.Sprintf(
				"文件名后缀 %s 与 MIME 类型 %s (解析为 %s) 不一致，已按 MIME 类型处理",
				result.FileNameExt, mimeType, result.MimeTypeExt,
			)
		} else if extSupported && extAllowed && mimeSupported && mimeAllowed {
			result.DetectedType = result.FileNameExt
			result.Source = TypeSourceFileName
			result.Conflict = true
			result.ConflictMessage = fmt.Sprintf(
				"文件名后缀 %s 与 MIME 类型 %s (解析为 %s) 不一致，已按文件名后缀处理",
				result.FileNameExt, mimeType, result.MimeTypeExt,
			)
		} else {
			result.Unsupported = true
			result.UnsupportedReason = fmt.Sprintf(
				"文件名后缀 %s 和 MIME 类型 %s 都不是支持的格式",
				result.FileNameExt, mimeType,
			)
			return result
		}
	} else if result.FileNameExt != "" {
		result.DetectedType = result.FileNameExt
		result.Source = TypeSourceFileName
	} else if result.MimeTypeExt != "" {
		result.DetectedType = result.MimeTypeExt
		result.Source = TypeSourceMimeType
	} else {
		result.Unsupported = true
		result.UnsupportedReason = "无法从文件名或 MIME 类型推断文件格式"
		return result
	}

	if !isSupportedType(result.DetectedType) {
		result.Unsupported = true
		result.UnsupportedReason = fmt.Sprintf(
			"检测到的文件类型 %s (来源: %s) 不在解析器支持列表中 (支持: %v)",
			result.DetectedType, result.Source, GetSupportedFileTypes(),
		)
		return result
	}

	if len(allowedExtensions) > 0 {
		if !allowedMap[strings.ToLower(result.DetectedType)] {
			result.Unsupported = true
			result.UnsupportedReason = fmt.Sprintf(
				"检测到的文件类型 %s 不在任务文档允许的类型 %v 中",
				result.DetectedType, allowedExtensions,
			)
			return result
		}
	}

	return result
}

func GetParsedTextFromUrl(url string, ext string, lang string) (string, error) {
	var path string
	var err error
	if !strings.HasPrefix(url, "http") {
		path = url
	} else {
		path, err = getTempFilePathFromUrl(url)
		if err != nil {
			return "", err
		}
		defer func() {
			err = os.Remove(path)
			if err != nil {
				logs.Error("%v", err.Error())
			}
		}()
	}

	var res string
	if ext == "" || ext == ".txt" || ext == ".md" || ext == ".yaml" {
		res, err = getTextFromPlain(path)
	} else if ext == ".csv" {
		res, err = getTextFromCsv(path)
	} else if ext == ".pdf" {
		res, err = getTextFromPdf(path)
	} else if ext == ".docx" {
		res, err = GetTextFromDocx(path, lang)
	} else if ext == ".xlsx" {
		res, err = getTextFromXlsx(path)
	} else if ext == ".pptx" {
		res, err = getTextFromPptx(path)
	} else {
		return "", fmt.Errorf(i18n.Translate(lang, "txt:unsupported file type: %s"), ext)
	}
	if err != nil {
		return "", err
	}

	return res, nil
}

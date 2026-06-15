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
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"strings"

	"github.com/beego/beego/logs"
	"github.com/the-open-agent/openagent/util"
)

func UpdateTreeFile(storeId string, key string, file *TreeFile) bool {
	return true
}

func AddTreeFile(storeId string, userName string, key string, isLeaf bool, filename string, file multipart.File, lang string) (bool, []byte, error) {
	store, err := GetStore(storeId)
	if err != nil {
		return false, nil, err
	}
	if store == nil {
		return false, nil, nil
	}

	storageProviderObj, err := store.GetStorageProviderObj(lang)
	if err != nil {
		return false, nil, err
	}

	var objectKey string
	var fileBuffer *bytes.Buffer
	if isLeaf {
		objectKey = fmt.Sprintf("%s/%s", key, filename)
		objectKey = strings.TrimLeft(objectKey, "/")
		fileBuffer = bytes.NewBuffer(nil)
		_, err = io.Copy(fileBuffer, file)
		if err != nil {
			return false, nil, err
		}

		bs := fileBuffer.Bytes()
		fileUrl, err := storageProviderObj.PutObject(userName, store.Name, objectKey, fileBuffer)
		if err != nil {
			return false, nil, err
		}

		// Persist file information in the file table
		fileName := getFileName(store.Name, objectKey)
		existingFile, _ := getFile(store.Owner, fileName)

		fileRecord := &File{
			Owner:           store.Owner,
			Name:            fileName,
			CreatedTime:     util.GetCurrentTime(),
			Filename:        filename,
			Size:            int64(len(bs)),
			Store:           store.Name,
			StorageProvider: store.StorageProvider,
			Url:             fileUrl,
			TokenCount:      0,
			Status:          FileStatusParsing,
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
			return false, nil, err
		}
		if !upserted {
			return false, nil, fmt.Errorf("failed to save file record")
		}

		go func() {
			logs.Info("Starting async vector generation for tree file: store=%s, file=%s", store.Name, objectKey)
			_, _, vectorErr := AddVectorsForFileIncremental(store, objectKey, fileUrl, lang)
			if vectorErr != nil {
				logs.Error("Failed to generate vectors for file %s: %v", objectKey, vectorErr)
			}
		}()

		return true, bs, nil
	} else {
		objectKey = fmt.Sprintf("%s/%s/_hidden.ini", key, filename)
		objectKey = strings.TrimLeft(objectKey, "/")
		fileBuffer = bytes.NewBuffer(nil)
		bs := fileBuffer.Bytes()
		_, err = storageProviderObj.PutObject(userName, store.Name, objectKey, fileBuffer)
		if err != nil {
			return false, nil, err
		}

		return true, bs, nil
	}
}

func DeleteTreeFile(storeId string, key string, isLeaf bool, lang string) (bool, error) {
	owner, name, err := util.GetOwnerAndNameFromIdWithError(storeId)
	if err != nil {
		return false, err
	}

	store, err := getStore(owner, name)
	if err != nil {
		return false, err
	}
	if store == nil {
		return false, nil
	}

	storageProviderObj, err := store.GetStorageProviderObj(lang)
	if err != nil {
		return false, err
	}

	if isLeaf {
		err = storageProviderObj.DeleteObject(key)
		if err != nil {
			return false, err
		}

		_, err = DeleteVectorsByFile(store.Owner, store.Name, key)
		if err != nil {
			logs.Error("Failed to delete vectors for file %s: %v", key, err)
			return false, err
		}

		// Delete file record from the file table
		if err := deleteFileRecord(owner, name, key); err != nil {
			return false, err
		}
	} else {
		objects, err := storageProviderObj.ListObjects(key)
		if err != nil {
			return false, err
		}

		for _, object := range objects {
			err = storageProviderObj.DeleteObject(object.Key)
			if err != nil {
				return false, err
			}

			_, err = DeleteVectorsByFile(store.Owner, store.Name, object.Key)
			if err != nil {
				logs.Error("Failed to delete vectors for file %s: %v", object.Key, err)
				return false, err
			}

			// Delete file record from the file table
			if err := deleteFileRecord(owner, name, object.Key); err != nil {
				return false, err
			}
		}
	}
	return true, nil
}

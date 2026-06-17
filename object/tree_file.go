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
	"mime/multipart"
	"strings"

	"github.com/beego/beego/logs"
	"github.com/the-open-agent/openagent/util"
)

func UpdateTreeFile(storeId string, key string, file *TreeFile) bool {
	return true
}

func AddTreeFile(storeId string, userName string, key string, isLeaf bool, filename string, file multipart.File, lang string) (*UploadResult, error) {
	store, err := GetStore(storeId)
	if err != nil {
		return nil, err
	}
	if store == nil {
		return nil, nil
	}

	storageProviderObj, err := store.GetStorageProviderObj(lang)
	if err != nil {
		return nil, err
	}

	if isLeaf {
		fullKey := fmt.Sprintf("%s/%s", key, filename)
		fullKey = strings.TrimLeft(fullKey, "/")

		uploadOpts := UploadOptions{
			FileName:        filename,
			FullStorageKey:  fullKey,
			AddRandomSuffix: false,
			Lang:            lang,
			StorageProvider: storageProviderObj,
			User:            userName,
			Parent:          store.Name,
		}

		uploadResult, err := UploadFromReader(file, uploadOpts)
		if err != nil {
			return nil, err
		}

		objectKey := uploadResult.StorageKey
		fileUrl := uploadResult.Url
		fileSize := uploadResult.FileSize

		fileRecord := &File{
			Owner:           store.Owner,
			Name:            getFileName(store.Name, objectKey),
			CreatedTime:     util.GetCurrentTime(),
			Filename:        filename,
			Size:            fileSize,
			Store:           store.Name,
			StorageProvider: store.StorageProvider,
			Url:             fileUrl,
			TokenCount:      0,
			Status:          FileStatusPending,
		}
		_, err = AddFile(fileRecord)
		if err != nil {
			return nil, err
		}

		go func() {
			_, vectorErr := AddVectorsForFile(store, objectKey, fileUrl, lang)
			if vectorErr != nil {
				logs.Error("Failed to generate vectors for file %s: %v", objectKey, vectorErr)
			}
		}()

		return uploadResult, nil
	} else {
		objectKey := fmt.Sprintf("%s/%s/_hidden.ini", key, filename)
		objectKey = strings.TrimLeft(objectKey, "/")

		uploadOpts := UploadOptions{
			FileName:        "_hidden.ini",
			FullStorageKey:  objectKey,
			AddRandomSuffix: false,
			Lang:            lang,
			StorageProvider: storageProviderObj,
			User:            userName,
			Parent:          store.Name,
		}

		uploadResult, err := UploadFromBytes([]byte{}, uploadOpts)
		if err != nil {
			return nil, err
		}

		return uploadResult, nil
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

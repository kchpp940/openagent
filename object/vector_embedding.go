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
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/beego/beego/logs"
	"github.com/cenkalti/backoff/v4"
	"github.com/the-open-agent/openagent/embedding"
	"github.com/the-open-agent/openagent/i18n"
	"github.com/the-open-agent/openagent/model"
	"github.com/the-open-agent/openagent/split"
	"github.com/the-open-agent/openagent/storage"
	"github.com/the-open-agent/openagent/txt"
	"github.com/the-open-agent/openagent/util"
)

func filterTextFiles(files []*storage.Object) []*storage.Object {
	fileTypes := txt.GetSupportedFileTypes()
	fileTypeMap := map[string]bool{}
	for _, fileType := range fileTypes {
		fileTypeMap[fileType] = true
	}

	res := []*storage.Object{}
	for _, file := range files {
		ext := filepath.Ext(file.Key)
		if fileTypeMap[ext] {
			res = append(res, file)
		}
	}
	return res
}

func addEmbeddedVector(embeddingProviderObj embedding.EmbeddingProvider, text string, storeName string, fileName string, index int, embeddingProviderName string, modelSubType string, lang string) (bool, int, error) {
	data, embeddingResult, err := queryVectorSafe(embeddingProviderObj, text, embeddingProviderName, lang)
	if err != nil {
		return false, 0, err
	}

	displayName := text
	if len(text) > 25 {
		displayName = string([]rune(text)[:25])
	}

	tokenCount := 0
	price := 0.0
	currency := ""
	if embeddingResult != nil {
		tokenCount = embeddingResult.TokenCount
		price = embeddingResult.Price
		currency = embeddingResult.Currency
	}

	defaultEmbeddingResult, err := embedding.GetDefaultEmbeddingResult(modelSubType, text)
	if err != nil {
		return false, 0, err
	}

	if tokenCount == 0 {
		tokenCount = defaultEmbeddingResult.TokenCount
	}
	if price == 0 {
		price = defaultEmbeddingResult.Price
	}
	if currency == "" {
		currency = defaultEmbeddingResult.Currency
	}

	vector := &Vector{
		Owner:       "admin",
		Name:        fmt.Sprintf("vector_%s", util.GetRandomName()),
		CreatedTime: util.GetCurrentTime(),
		DisplayName: displayName,
		Store:       storeName,
		Provider:    embeddingProviderName,
		File:        fileName,
		Index:       index,
		Text:        text,
		TokenCount:  tokenCount,
		Price:       price,
		Currency:    currency,
		Data:        data,
		Dimension:   len(data),
	}
	affected, err := AddVector(vector)
	return affected, tokenCount, err
}

func addVectorsForFile(embeddingProviderObj embedding.EmbeddingProvider, storeName string, fileKey string, fileUrl string, splitProviderName string, embeddingProviderName string, modelSubType string, lang string) (bool, int, error) {
	return addVectorsForFileIncremental(embeddingProviderObj, storeName, fileKey, fileUrl, splitProviderName, embeddingProviderName, modelSubType, lang, false)
}

func addVectorsForFileIncremental(embeddingProviderObj embedding.EmbeddingProvider, storeName string, fileKey string, fileUrl string, splitProviderName string, embeddingProviderName string, modelSubType string, lang string, incremental bool) (bool, int, error) {
	var (
		affected        bool
		totalTokenCount int
	)

	owner := "admin"
	fileName := getFileName(storeName, fileKey)
	existingFile, err := getFile(owner, fileName)
	if err != nil {
		return false, 0, err
	}

	var newVersion int = 1
	if existingFile != nil {
		newVersion = existingFile.ParseVersion + 1
	}

	var oldParseVersion *FileParseVersion
	if existingFile != nil {
		oldParseVersion, _ = GetLatestFileParseVersion(owner, fileName)
	}

	var oldChunks []string
	var oldVectors []*Vector
	if oldParseVersion != nil {
		oldVectors, _ = GetVectorsByFile(owner, storeName, fileKey)
		oldChunks = make([]string, len(oldVectors))
		for _, v := range oldVectors {
			if v.Index < len(oldChunks) {
				oldChunks[v.Index] = v.Text
			}
		}
	}

	diff := &FileVersionDiff{
		Owner:            owner,
		Name:             fileName,
		Version:          newVersion,
		CreatedTime:      util.GetCurrentTime(),
		FromParseVersion: 0,
		ToParseVersion:   newVersion,
		FromContentHash:  "",
		ToContentHash:    "",
		FromChunkHash:    "",
		ToChunkHash:      "",
		Status:           FileStatusParsing,
	}

	if existingFile != nil {
		diff.FromParseVersion = existingFile.ParseVersion
		diff.FromContentHash = existingFile.ContentHash
		diff.FromChunkHash = existingFile.ChunkHash
	}

	_, _ = AddFileVersionDiff(diff)

	fileExt := filepath.Ext(fileKey)
	text, err := txt.GetParsedTextFromUrl(fileUrl, fileExt, lang)
	if err != nil {
		diff.ParseError = err.Error()
		diff.ErrorText = "File parsing failed: " + err.Error()
		diff.Status = FileStatusError
		_, _ = UpdateFileVersionDiff(diff)
		saveParseVersionOnError(owner, fileName, newVersion, "", "", 0, nil, FileStatusError, diff)
		if existingFile != nil {
			existingFile.Status = FileStatusError
			existingFile.ErrorText = diff.ErrorText
			_, _ = UpdateFile(existingFile.GetId(), existingFile)
		}
		return false, 0, err
	}

	contentHash := calculateHash(text)
	diff.ToContentHash = contentHash

	splitProviderType := splitProviderName
	if splitProviderType == "" {
		splitProviderType = "Default"
	}

	if strings.HasPrefix(fileKey, "QA") && fileExt == ".docx" {
		splitProviderType = "QA"
	}

	if fileExt == ".md" {
		splitProviderType = "Markdown"
	}

	splitProvider, err := split.GetSplitProvider(splitProviderType)
	if err != nil {
		diff.ParseError = err.Error()
		diff.ErrorText = "Failed to get split provider: " + err.Error()
		diff.Status = FileStatusError
		_, _ = UpdateFileVersionDiff(diff)
		saveParseVersionOnError(owner, fileName, newVersion, contentHash, "", 0, nil, FileStatusError, diff)
		if existingFile != nil {
			existingFile.Status = FileStatusError
			existingFile.ErrorText = diff.ErrorText
			_, _ = UpdateFile(existingFile.GetId(), existingFile)
		}
		return false, 0, err
	}

	newChunks, err := splitProvider.SplitText(text)
	if err != nil {
		diff.ParseError = err.Error()
		diff.ErrorText = "Text splitting failed: " + err.Error()
		diff.Status = FileStatusError
		_, _ = UpdateFileVersionDiff(diff)
		saveParseVersionOnError(owner, fileName, newVersion, contentHash, "", 0, nil, FileStatusError, diff)
		if existingFile != nil {
			existingFile.Status = FileStatusError
			existingFile.ErrorText = diff.ErrorText
			_, _ = UpdateFile(existingFile.GetId(), existingFile)
		}
		return false, 0, err
	}

	newChunkHash := calculateChunksHash(newChunks)
	newChunkHashes := calculateChunkHashes(newChunks)
	diff.ToChunkHash = newChunkHash

	if incremental && existingFile != nil && existingFile.ContentHash != "" {
		if existingFile.ContentHash == contentHash && existingFile.ChunkHash == newChunkHash {
			logs.Info("File content unchanged, skipping re-parse for store: [%s], file: [%s]", storeName, fileKey)
			diff.Status = FileStatusFinished
			diff.UnchangedCount = len(newChunks)
			_, _ = UpdateFileVersionDiff(diff)
			saveParseVersion(owner, fileName, newVersion, contentHash, newChunkHash, existingFile.VectorVersion, newChunks, newChunkHashes, FileStatusFinished, diff)
			if existingFile != nil {
				existingFile.ParseVersion = newVersion
				existingFile.VectorVersion = existingFile.VectorVersion + 1
				existingFile.LatestDiff = diff
				existingFile.Status = FileStatusFinished
				existingFile.ErrorText = ""
				_, _ = UpdateFile(existingFile.GetId(), existingFile)
			}
			return false, 0, nil
		}
	}

	diff.Status = FileStatusVectorizing
	_, _ = UpdateFileVersionDiff(diff)

	var diffs []ChunkDiff
	var chunksToProcess []int
	vectorGenerationErrors := []string{}
	failedChunkIndices := []int{}

	if incremental && existingFile != nil && existingFile.ContentHash != "" {
		diffs = computeChunkDiff(oldChunks, newChunks)

		addedSamples, deletedSamples, modifiedSamples, _ := summarizeDiff(diffs, 5)
		for _, d := range diffs {
			switch d.Type {
			case ChunkDiffAdded:
				diff.AddedCount++
			case ChunkDiffDeleted:
				diff.DeletedCount++
			case ChunkDiffModified:
				diff.ModifiedCount++
			case ChunkDiffUnchanged:
				diff.UnchangedCount++
			}
		}
		diff.AddedChunks = addedSamples
		diff.DeletedChunks = deletedSamples
		diff.ModifiedChunks = modifiedSamples

		vectorsToDelete := make(map[int]bool)
		for _, d := range diffs {
			if d.Type == ChunkDiffDeleted || d.Type == ChunkDiffModified {
				vectorsToDelete[d.Index] = true
			}
		}

		oldVectorMap := make(map[int]*Vector)
		for _, v := range oldVectors {
			oldVectorMap[v.Index] = v
		}

		var deleteErrors []string
		for idx := range vectorsToDelete {
			if v, ok := oldVectorMap[idx]; ok {
				_, delErr := DeleteVector(v)
				if delErr != nil {
					deleteErrors = append(deleteErrors, fmt.Sprintf("index %d: %v", idx, delErr))
				}
			}
		}
		if len(deleteErrors) > 0 {
			diff.DeleteVectorsError = strings.Join(deleteErrors, "; ")
		}

		chunksToProcess = make([]int, 0)
		for i, d := range diffs {
			if d.Type == ChunkDiffAdded || d.Type == ChunkDiffModified {
				chunksToProcess = append(chunksToProcess, i)
			}
		}

		logs.Info("Incremental re-parse for store: [%s], file: [%s]: added=%d, deleted=%d, modified=%d, unchanged=%d, processing=%d chunks",
			storeName, fileKey, diff.AddedCount, diff.DeletedCount, diff.ModifiedCount, diff.UnchangedCount, len(chunksToProcess))
	} else {
		if !incremental && existingFile != nil && len(oldVectors) > 0 {
			logs.Info("Full refresh, deleting all existing vectors for store: [%s], file: [%s], count=%d", storeName, fileKey, len(oldVectors))
			_, delErr := DeleteVectorsByFile(owner, storeName, fileKey)
			if delErr != nil {
				diff.DeleteVectorsError = delErr.Error()
			}
		}

		diff.AddedCount = len(newChunks)
		chunksToProcess = make([]int, len(newChunks))
		for i := range newChunks {
			chunksToProcess[i] = i
		}
		addedSamples, _, _, _ := summarizeDiff(diffs, 5)
		if len(addedSamples) == 0 {
			for i := 0; i < len(newChunks) && i < 5; i++ {
				addedSamples = append(addedSamples, ChunkDiff{
					Index:   i,
					Type:    ChunkDiffAdded,
					NewText: newChunks[i],
					NewHash: calculateHash(newChunks[i]),
				})
			}
		}
		diff.AddedChunks = addedSamples
	}

	hasErrors := false
	for _, i := range chunksToProcess {
		textSection := newChunks[i]
		logs.Info("[%d/%d] Generating embedding for store: [%s], file: [%s], index: [%d]: %s", i+1, len(chunksToProcess), storeName, fileKey, i, textSection)

		var (
			sectionAffected   bool
			sectionTokenCount int
		)
		operation := func() error {
			var opErr error
			sectionAffected, sectionTokenCount, opErr = addEmbeddedVector(embeddingProviderObj, textSection, storeName, fileKey, i, embeddingProviderName, modelSubType, lang)
			if opErr != nil {
				if isRetryableError(opErr) {
					return opErr
				}
				return backoff.Permanent(opErr)
			}
			return nil
		}
		err = backoff.Retry(operation, backoff.NewExponentialBackOff())
		if err != nil {
			logs.Error("Failed to generate embedding for index %d after retries: %v", i, err)
			hasErrors = true
			vectorGenerationErrors = append(vectorGenerationErrors, fmt.Sprintf("index %d: %v", i, err))
			failedChunkIndices = append(failedChunkIndices, i)
			continue
		}

		affected = affected || sectionAffected
		totalTokenCount += sectionTokenCount
	}

	if len(vectorGenerationErrors) > 0 {
		diff.VectorGenerationErrors = vectorGenerationErrors
		diff.FailedChunkIndices = failedChunkIndices
	}

	if hasErrors {
		diff.Status = FileStatusPartialFailed
		if diff.ErrorText == "" {
			diff.ErrorText = fmt.Sprintf("Some vectors failed to generate (%d/%d)", len(failedChunkIndices), len(chunksToProcess))
		}
	} else if diff.ParseError != "" || diff.DeleteVectorsError != "" {
		diff.Status = FileStatusPartialFailed
		if diff.ErrorText == "" {
			diff.ErrorText = "Completed with some errors"
		}
	} else {
		diff.Status = FileStatusFinished
		diff.ErrorText = ""
	}
	_, _ = UpdateFileVersionDiff(diff)

	nextVectorVersion := 1
	if existingFile != nil {
		if incremental {
			nextVectorVersion = existingFile.VectorVersion + 1
		}
		existingFile.ParseVersion = newVersion
		existingFile.ContentHash = contentHash
		existingFile.ChunkHash = newChunkHash
		existingFile.VectorVersion = nextVectorVersion
		existingFile.LatestDiff = diff
		existingFile.Status = diff.Status
		existingFile.ErrorText = diff.ErrorText
		_, _ = UpdateFile(existingFile.GetId(), existingFile)
	}

	saveParseVersion(owner, fileName, newVersion, contentHash, newChunkHash, nextVectorVersion, newChunks, newChunkHashes, diff.Status, diff)

	return affected, totalTokenCount, nil
}

func saveParseVersion(owner string, fileName string, version int, contentHash string, chunkHash string, vectorVersion int, chunks []string, chunkHashes map[string]string, status FileStatus, diff *FileVersionDiff) {
	parseVersion := &FileParseVersion{
		Owner:                  owner,
		Name:                   fileName,
		Version:                version,
		CreatedTime:            util.GetCurrentTime(),
		ContentHash:            contentHash,
		ChunkHash:              chunkHash,
		VectorVersion:          vectorVersion,
		ChunkCount:             len(chunks),
		ChunkHashes:            chunkHashes,
		Status:                 status,
		ParseError:             diff.ParseError,
		DeleteVectorsError:     diff.DeleteVectorsError,
		VectorGenerationErrors: diff.VectorGenerationErrors,
		FailedChunkIndices:     diff.FailedChunkIndices,
		ErrorText:              diff.ErrorText,
	}
	_, _ = AddFileParseVersion(parseVersion)
}

func saveParseVersionOnError(owner string, fileName string, version int, contentHash string, chunkHash string, vectorVersion int, chunks []string, status FileStatus, diff *FileVersionDiff) {
	var chunkHashes map[string]string
	if chunks != nil {
		chunkHashes = calculateChunkHashes(chunks)
	}
	chunkCount := 0
	if chunks != nil {
		chunkCount = len(chunks)
	}
	parseVersion := &FileParseVersion{
		Owner:                  owner,
		Name:                   fileName,
		Version:                version,
		CreatedTime:            util.GetCurrentTime(),
		ContentHash:            contentHash,
		ChunkHash:              chunkHash,
		VectorVersion:          vectorVersion,
		ChunkCount:             chunkCount,
		ChunkHashes:            chunkHashes,
		Status:                 status,
		ParseError:             diff.ParseError,
		DeleteVectorsError:     diff.DeleteVectorsError,
		VectorGenerationErrors: diff.VectorGenerationErrors,
		FailedChunkIndices:     diff.FailedChunkIndices,
		ErrorText:              diff.ErrorText,
	}
	_, _ = AddFileParseVersion(parseVersion)
}

func withFileStatus(owner string, storeName string, fileKey string, op func() (bool, int, error)) (bool, error) {
	err := updateFileStatus(owner, storeName, fileKey, FileStatusProcessing, "", 0)
	if err != nil {
		logs.Error("Failed to update file status for store: [%s], file: [%s]: %v", storeName, fileKey, err)
		return false, err
	}

	affected, tokenCount, opErr := op()

	fileStatus := FileStatusFinished
	errorText := ""
	if opErr != nil {
		fileStatus = FileStatusError
		errorText = opErr.Error()
	}

	err = updateFileStatus(owner, storeName, fileKey, fileStatus, errorText, tokenCount)
	if err != nil {
		logs.Error("Failed to update file status for store: [%s], file: [%s]: %v", storeName, fileKey, err)
		return affected, errors.Join(opErr, err)
	}

	return affected, opErr
}

func withFileStatusIncremental(owner string, storeName string, fileKey string, op func() (bool, int, error)) (bool, error) {
	err := updateFileStatus(owner, storeName, fileKey, FileStatusParsing, "", 0)
	if err != nil {
		logs.Error("Failed to update file status for store: [%s], file: [%s]: %v", storeName, fileKey, err)
		return false, err
	}

	affected, tokenCount, opErr := op()

	fileStatus := FileStatusFinished
	errorText := ""
	if opErr != nil {
		fileStatus = FileStatusError
		errorText = opErr.Error()
	}

	err = updateFileStatus(owner, storeName, fileKey, fileStatus, errorText, tokenCount)
	if err != nil {
		logs.Error("Failed to update file status for store: [%s], file: [%s]: %v", storeName, fileKey, err)
		return affected, errors.Join(opErr, err)
	}

	return affected, opErr
}

func addVectorsForStore(storageProviderObj storage.StorageProvider, embeddingProviderObj embedding.EmbeddingProvider, prefix string, owner string, storeName string, splitProviderName string, embeddingProviderName string, modelSubType string, lang string) (bool, error) {
	var (
		affected bool
		fileErr  error
	)

	files, err := storageProviderObj.ListObjects(prefix)
	if err != nil {
		return false, err
	}

	files = filterTextFiles(files)

	for _, file := range files {
		fileAffected, err := withFileStatus(owner, storeName, file.Key, func() (bool, int, error) {
			return addVectorsForFile(embeddingProviderObj, storeName, file.Key, file.Url, splitProviderName, embeddingProviderName, modelSubType, lang)
		})
		if err != nil {
			logs.Error("Failed to add vectors for store: [%s], file: [%s]: %v", storeName, file.Key, err)
			fileErr = errors.Join(fileErr, err)
			continue
		}

		affected = affected || fileAffected
	}

	return affected, fileErr
}

func getRelatedVectors(relatedStores []string, provider string) ([]*Vector, error) {
	vectors, err := getVectorsByProvider(relatedStores, provider)
	if err != nil {
		return nil, err
	}
	if len(vectors) == 0 {
		return nil, fmt.Errorf("no knowledge vectors found")
	}

	return vectors, nil
}

func queryVectorWithContext(embeddingProvider embedding.EmbeddingProvider, text string, timeout int, lang string) ([]float32, *embedding.EmbeddingResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(30+timeout*2)*time.Second)
	defer cancel()
	vector, embeddingResult, err := embeddingProvider.QueryVector(text, ctx, lang)
	return vector, embeddingResult, err
}

func queryVectorSafe(embeddingProvider embedding.EmbeddingProvider, text string, providerName string, lang string) ([]float32, *embedding.EmbeddingResult, error) {
	var res []float32
	var embeddingResult *embedding.EmbeddingResult
	var err error
	for i := 0; i < 10; i++ {
		res, embeddingResult, err = queryVectorWithContext(embeddingProvider, text, i, lang)
		if err != nil {
			err = fmt.Errorf(i18n.Translate(lang, "object:queryVectorSafe() error, provider: %s, %s"), providerName, err.Error())
			if i > 0 {
				logs.Error("\tFailed (%d): %s", i+1, err.Error())
			}
		} else {
			break
		}
	}

	if err != nil {
		return nil, nil, err
	} else {
		return res, embeddingResult, nil
	}
}

func GetNearestKnowledge(storeName string, vectorStores []string, searchProviderType string, embeddingProvider *Provider, embeddingProviderObj embedding.EmbeddingProvider, modelProvider *Provider, owner string, text string, knowledgeCount int, lang string) ([]*model.RawMessage, []VectorScore, *embedding.EmbeddingResult, error) {
	searchProvider, err := GetSearchProvider(searchProviderType, owner)
	if err != nil {
		return nil, nil, nil, err
	}

	relatedStores := append(vectorStores, storeName)
	vectors, embeddingResult, err := searchProvider.Search(relatedStores, embeddingProvider.Name, embeddingProviderObj, modelProvider.Name, text, knowledgeCount, lang)
	if err != nil {
		if err.Error() == "no knowledge vectors found" {
			return nil, nil, embeddingResult, err
		} else {
			return nil, nil, nil, err
		}
	}

	vectorScores := []VectorScore{}
	knowledge := []*model.RawMessage{}
	for _, vector := range vectors {
		// if embeddingProvider.Name != vector.Provider {
		//	return "", nil, fmt.Errorf(i18n.Translate(lang, "object:The store's embedding provider: [%s] should equal to vector's embedding provider: [%s], vector = %v"), embeddingProvider.Name, vector.Provider, vector)
		// }

		vectorScores = append(vectorScores, VectorScore{
			Vector: vector.Name,
			Score:  vector.Score,
		})
		knowledge = append(knowledge, &model.RawMessage{
			Text:           vector.Text,
			Author:         "System",
			TextTokenCount: vector.TokenCount,
		})
	}

	return knowledge, vectorScores, embeddingResult, nil
}

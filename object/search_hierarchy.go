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
	"fmt"
	"strings"

	"github.com/beego/beego/logs"
	"github.com/the-open-agent/openagent/embedding"
	"github.com/the-open-agent/openagent/i18n"
	"github.com/the-open-agent/openagent/model"
)

type HierarchySearchProvider struct {
	owner string
}

func NewHierarchySearchProvider(owner string) (*HierarchySearchProvider, error) {
	return &HierarchySearchProvider{owner: owner}, nil
}

func extractMarkdownTitlesFromVectors(vectors []*Vector) []string {
	titleMap := make(map[string]bool)
	for _, v := range vectors {
		if v == nil || v.File == "" || !strings.HasSuffix(strings.ToLower(v.File), ".md") {
			continue
		}
		if len(v.Data) == 0 {
			continue
		}
		parts := strings.SplitN(v.Text, "\n\n", 2)
		if len(parts) > 0 && strings.TrimSpace(parts[0]) != "" {
			titleMap[parts[0]] = true
		}
	}
	titles := make([]string, 0, len(titleMap))
	for t := range titleMap {
		titles = append(titles, t)
	}
	return titles
}

func (p *HierarchySearchProvider) Search(relatedStores []string, embeddingProviderName string, embeddingProviderObj embedding.EmbeddingProvider, modelProviderName string, text string, knowledgeCount int, lang string) ([]Vector, *embedding.EmbeddingResult, error) {
	vectors, err := getRelatedVectors(relatedStores, embeddingProviderName)
	if err != nil {
		return nil, nil, err
	}

	titleCandidates := extractMarkdownTitlesFromVectors(vectors)

	question, _, err := getEnhancedQuestionByModel(modelProviderName, text, titleCandidates, knowledgeCount, lang)
	if err != nil {
		return nil, nil, err
	}

	qVector, embeddingResult, err := queryVectorSafe(embeddingProviderObj, question, embeddingProviderName, lang)
	if err != nil {
		return nil, embeddingResult, err
	}
	if qVector == nil || len(qVector) == 0 {
		return nil, embeddingResult, fmt.Errorf(i18n.Translate(lang, "object:no qVector found"))
	}

	candidates := buildVectorCandidates(vectors)
	if len(candidates) == 0 {
		return nil, embeddingResult, fmt.Errorf(i18n.Translate(lang, "object:no valid candidate vectors available"))
	}

	similarities, err := getNearestVectors(qVector, candidates, knowledgeCount, lang)
	if err != nil {
		return nil, embeddingResult, err
	}

	res := make([]Vector, 0, len(similarities))
	for _, sr := range similarities {
		vector := *sr.Candidate.Vector
		vector.Score = sr.Similarity
		res = append(res, vector)
	}

	return res, embeddingResult, nil
}

func getEnhancedQuestionByModel(modelProviderName string, text string, titleCandidates []string, candidateTitlesNum int, lang string) (string, *model.ModelResult, error) {
	candidateTitlesNum = validateKnowledgeCount(candidateTitlesNum)
	if candidateTitlesNum > len(titleCandidates) {
		candidateTitlesNum = len(titleCandidates)
	}

	prompt := fmt.Sprintf("Please help me select the top %d titles that are most likely to contain the answer. Just return the title list. No other content.", candidateTitlesNum)

	question := fmt.Sprintf("Please select the titles most relevant to the following question and choose the %v most relevant items. Just return the title list. No other content.\nquestion:\n %s \n\nTitles: \n%s", candidateTitlesNum, text, "• "+strings.Join(titleCandidates, "\n• "))

	history := []*model.RawMessage{}
	knowledge := []*model.RawMessage{}
	res, modelResult, err := GetAnswerWithContext(modelProviderName, question, history, knowledge, prompt, lang)
	if err != nil {
		return "", nil, err
	}
	enhancedQuestion := fmt.Sprintf("%s\n\nrelated contents: %s", text, res)
	return enhancedQuestion, modelResult, nil
}

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

	"github.com/the-open-agent/openagent/embedding"
	"github.com/the-open-agent/openagent/i18n"
)

type DefaultSearchProvider struct {
	owner string
}

func NewDefaultSearchProvider(owner string) (*DefaultSearchProvider, error) {
	return &DefaultSearchProvider{owner: owner}, nil
}

func (p *DefaultSearchProvider) Search(relatedStores []string, embeddingProviderName string, embeddingProviderObj embedding.EmbeddingProvider, modelProviderName string, text string, knowledgeCount int, lang string) (*SearchResultSet, *embedding.EmbeddingResult, error) {
	vectors, err := getRelatedVectors(relatedStores, embeddingProviderName)
	if err != nil {
		return nil, nil, err
	}

	qVector, embeddingResult, err := queryVectorSafe(embeddingProviderObj, text, embeddingProviderName, lang)
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

	rs, err := buildSearchResultSet(similarities, lang)
	return rs, embeddingResult, err
}

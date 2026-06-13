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
	"math"
	"sort"

	"github.com/beego/beego/logs"
	"github.com/the-open-agent/openagent/i18n"
)

func dot(vec1, vec2 []float32) float32 {
	if len(vec1) != len(vec2) {
		panic("Vector lengths do not match")
	}

	dotProduct := float32(0.0)
	for i := range vec1 {
		dotProduct += vec1[i] * vec2[i]
	}
	return dotProduct
}

func norm(vec []float32) float32 {
	normSquared := float32(0.0)
	for _, val := range vec {
		normSquared += val * val
	}
	return float32(math.Sqrt(float64(normSquared)))
}

func cosineSimilarity(vec1, vec2 []float32, vec1Norm float32) float32 {
	dotProduct := dot(vec1, vec2)
	vec2Norm := norm(vec2)
	if vec2Norm == 0 {
		return 0.0
	}
	return dotProduct / (vec1Norm * vec2Norm)
}

type VectorCandidate struct {
	Vector   *Vector
	Data     []float32
	FileName string
	ChunkIdx int
}

type SimilarityResult struct {
	Similarity float32
	Candidate  *VectorCandidate
}

func buildVectorCandidates(vectors []*Vector) []*VectorCandidate {
	candidates := make([]*VectorCandidate, 0, len(vectors))
	for _, v := range vectors {
		if v == nil {
			logs.Warn("Skipping nil vector")
			continue
		}
		if len(v.Data) == 0 {
			logs.Warn("Skipping empty vector, file=%s, index=%d", v.File, v.Index)
			continue
		}
		candidates = append(candidates, &VectorCandidate{
			Vector:   v,
			Data:     v.Data,
			FileName: v.File,
			ChunkIdx: v.Index,
		})
	}
	return candidates
}

func validateKnowledgeCount(n int) int {
	if n <= 0 {
		return 1
	}
	if n > 100 {
		return 100
	}
	return n
}

func getNearestVectors(target []float32, candidates []*VectorCandidate, n int, lang string) ([]SimilarityResult, error) {
	if len(target) == 0 {
		return nil, fmt.Errorf(i18n.Translate(lang, "object:target vector is empty"))
	}
	if len(candidates) == 0 {
		return nil, fmt.Errorf(i18n.Translate(lang, "object:no candidate vectors available"))
	}

	n = validateKnowledgeCount(n)
	targetNorm := norm(target)
	if targetNorm == 0 {
		return nil, fmt.Errorf(i18n.Translate(lang, "object:target vector has zero norm"))
	}

	similarities := []SimilarityResult{}
	for _, candidate := range candidates {
		if candidate == nil || len(candidate.Data) == 0 {
			continue
		}
		if len(target) != len(candidate.Data) {
			logs.Warn("The target vector's length: [%d] should equal to knowledge vector's length: [%d], file=%s, index=%d",
				len(target), len(candidate.Data), candidate.FileName, candidate.ChunkIdx)
			continue
		}

		similarity := cosineSimilarity(target, candidate.Data, targetNorm)
		similarities = append(similarities, SimilarityResult{
			Similarity: similarity,
			Candidate:  candidate,
		})
	}

	if len(similarities) == 0 {
		return nil, fmt.Errorf(i18n.Translate(lang, "object:no valid candidate vectors after dimension check"))
	}

	sort.Slice(similarities, func(i, j int) bool {
		return similarities[i].Similarity > similarities[j].Similarity
	})

	if n > len(similarities) {
		n = len(similarities)
	}
	return similarities[:n], nil
}

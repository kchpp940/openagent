// Copyright 2024 The OpenAgent Authors. All Rights Reserved.
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
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/beego/beego/logs"
)

const analyzeTaskPrompt = `请对以下教学设计文本进行深度分析，根据提供的评价量表对每个二级评价项进行评分和详细分析。

评分要求：每个二级评价项的得分必须在 60-100 分之间（整数）。请根据达成程度在该区间内给分：表现较好给高分（80-100），一般给中分（70-79），有待改进给较低分（60-69），以更好区分不同水平的教学设计。

评价量表：
%s

教学设计文本：
%s

请严格按照以下JSON格式返回分析结果，不要包含任何其他内容，只返回合法的JSON：
{
  "title": "从文档中提取的课题/单元名称",
  "designer": "从文档中提取的设计/实施者姓名或团队",
  "stage": "从文档中提取的学段（如：小学、初中、高中）",
  "participants": "从文档中提取的参与者描述",
  "grade": "从文档中提取的年级",
  "instructor": "从文档中提取的指导教师姓名",
  "subject": "从文档中提取的学科",
  "school": "从文档中提取的学校名称",
  "otherSubjects": "从文档中提取的其他相关领域或学科（逗号分隔）",
  "textbook": "从文档中提取的主要教材信息",
  "score": 所有二级评价项得分的平均值（保留一位小数的数字，不是字符串）,
  "categories": [
    {
      "name": "一级评价项名称",
      "score": 该类别下所有二级评价项得分的平均值（保留两位小数的数字）,
      "items": [
        {
          "name": "二级评价项名称",
          "score": 该项得分（0-100的整数）,
          "advantage": "优点分析（详细说明教学设计在该项的优势和亮点）",
          "disadvantage": "不足分析（详细说明教学设计在该项存在的问题和不足）",
          "suggestion": "改进建议（提供具体可操作的改进措施和建议）"
        }
      ]
    }
  ]
}`

func extractJSON(raw string) string {
	raw = strings.TrimSpace(raw)

	lower := strings.ToLower(raw)

	codeFencePatterns := []string{
		"```json",
		"``` json",
		"```javascript",
		"``` js",
		"```typescript",
		"``` ts",
		"```",
	}

	for _, pattern := range codeFencePatterns {
		idx := strings.Index(lower, pattern)
		if idx != -1 {
			afterFence := raw[idx+len(pattern):]
			endIdx := strings.Index(afterFence, "```")
			if endIdx != -1 {
				raw = strings.TrimSpace(afterFence[:endIdx])
				break
			}
		}
	}

	raw = strings.TrimSpace(raw)

	raw = strings.ReplaceAll(raw, "\\\n", "")
	raw = strings.ReplaceAll(raw, "\\\r", "")

	firstBrace := strings.Index(raw, "{")
	firstBracket := strings.Index(raw, "[")
	start := -1
	end := -1

	if firstBrace != -1 && (firstBracket == -1 || firstBrace < firstBracket) {
		start = firstBrace
		depth := 0
		inString := false
		escaped := false
		for i := start; i < len(raw); i++ {
			ch := raw[i]
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == '"' {
				inString = !inString
				continue
			}
			if inString {
				continue
			}
			if ch == '{' {
				depth++
			} else if ch == '}' {
				depth--
				if depth == 0 {
					end = i + 1
					break
				}
			}
		}
	} else if firstBracket != -1 {
		start = firstBracket
		depth := 0
		inString := false
		escaped := false
		for i := start; i < len(raw); i++ {
			ch := raw[i]
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == '"' {
				inString = !inString
				continue
			}
			if inString {
				continue
			}
			if ch == '[' {
				depth++
			} else if ch == ']' {
				depth--
				if depth == 0 {
					end = i + 1
					break
				}
			}
		}
	}

	if start != -1 && end != -1 && end > start {
		extracted := raw[start:end]
		return strings.TrimSpace(extracted)
	}

	return strings.TrimSpace(raw)
}

func parseFloat(v interface{}) float64 {
	switch val := v.(type) {
	case float64:
		return val
	case float32:
		return float64(val)
	case int:
		return float64(val)
	case int64:
		return float64(val)
	case int32:
		return float64(val)
	case string:
		s := strings.TrimSpace(val)
		s = strings.Trim(s, "\"'`")
		s = strings.ReplaceAll(s, ",", "")
		if f, err := strconv.ParseFloat(s, 64); err == nil {
			return f
		}
	}
	return 0
}

func parseString(v interface{}) string {
	if v == nil {
		return ""
	}
	switch val := v.(type) {
	case string:
		return strings.TrimSpace(val)
	default:
		return strings.TrimSpace(fmt.Sprintf("%v", val))
	}
}

func getFieldIgnoreCase(data map[string]interface{}, keys ...string) interface{} {
	for _, key := range keys {
		if v, ok := data[key]; ok && v != nil {
			return v
		}
		lowerKey := strings.ToLower(key)
		for k, v := range data {
			if strings.ToLower(k) == lowerKey && v != nil {
				return v
			}
		}
	}
	return nil
}

func normalizeTaskResult(result *TaskResult) {
	if result == nil {
		return
	}

	var allItemScores []float64

	for i, cat := range result.Categories {
		if cat == nil {
			continue
		}

		var catItemScores []float64
		for j, item := range cat.Items {
			if item == nil {
				continue
			}
			if item.Score <= 0 {
				item.Score = 70
			}
			item.Score = math.Round(item.Score)
			if item.Score < 60 {
				item.Score = 60
			}
			if item.Score > 100 {
				item.Score = 100
			}
			catItemScores = append(catItemScores, item.Score)
			allItemScores = append(allItemScores, item.Score)
			cat.Items[j] = item
		}

		if cat.Score <= 0 && len(catItemScores) > 0 {
			sum := 0.0
			for _, s := range catItemScores {
				sum += s
			}
			cat.Score = math.Round((sum/float64(len(catItemScores)))*100) / 100
		}
		result.Categories[i] = cat
	}

	if result.Score <= 0 && len(allItemScores) > 0 {
		sum := 0.0
		for _, s := range allItemScores {
			sum += s
		}
		result.Score = math.Round((sum/float64(len(allItemScores)))*10) / 10
	}
}

func normalizeTaskResultFromMap(data map[string]interface{}) *TaskResult {
	result := &TaskResult{
		Title:         parseString(getFieldIgnoreCase(data, "title", "Title", "subject", "topic")),
		Designer:      parseString(getFieldIgnoreCase(data, "designer", "Designer", "author", "Author")),
		Stage:         parseString(getFieldIgnoreCase(data, "stage", "Stage", "section", "Section")),
		Participants:  parseString(getFieldIgnoreCase(data, "participants", "Participants", "student", "students")),
		Grade:         parseString(getFieldIgnoreCase(data, "grade", "Grade", "class", "Class")),
		Instructor:    parseString(getFieldIgnoreCase(data, "instructor", "Instructor", "teacher", "Teacher", "guide", "Guide")),
		Subject:       parseString(getFieldIgnoreCase(data, "subject", "Subject", "discipline", "Discipline")),
		School:        parseString(getFieldIgnoreCase(data, "school", "School", "institution", "Institution")),
		OtherSubjects: parseString(getFieldIgnoreCase(data, "otherSubjects", "other_subjects", "OtherSubjects", "other", "Other")),
		Textbook:      parseString(getFieldIgnoreCase(data, "textbook", "Textbook", "material", "Material", "courseware", "Courseware")),
		Score:         parseFloat(getFieldIgnoreCase(data, "score", "Score", "totalScore", "total_score", "average", "Average")),
	}

	if categoriesRaw, ok := getFieldIgnoreCase(data, "categories", "Categories", "dimensions", "Dimensions", "sections", "Sections").([]interface{}); ok {
		for _, catRaw := range categoriesRaw {
			if catMap, ok := catRaw.(map[string]interface{}); ok {
				cat := &TaskResultCategory{
					Name:  parseString(getFieldIgnoreCase(catMap, "name", "Name", "title", "Title")),
					Score: parseFloat(getFieldIgnoreCase(catMap, "score", "Score", "average", "Average")),
				}

				if itemsRaw, ok := getFieldIgnoreCase(catMap, "items", "Items", "subitems", "SubItems", "criteria", "Criteria").([]interface{}); ok {
					for _, itemRaw := range itemsRaw {
						if itemMap, ok := itemRaw.(map[string]interface{}); ok {
							item := &TaskResultItem{
								Name:         parseString(getFieldIgnoreCase(itemMap, "name", "Name", "title", "Title", "criterion", "Criterion")),
								Score:        parseFloat(getFieldIgnoreCase(itemMap, "score", "Score", "point", "Point", "mark", "Mark")),
								Advantage:    parseString(getFieldIgnoreCase(itemMap, "advantage", "Advantage", "strength", "Strength", "merit", "Merit", "highlight", "Highlight")),
								Disadvantage: parseString(getFieldIgnoreCase(itemMap, "disadvantage", "Disadvantage", "weakness", "Weakness", "shortcoming", "Shortcoming", "problem", "Problem")),
								Suggestion:   parseString(getFieldIgnoreCase(itemMap, "suggestion", "Suggestion", "recommendation", "Recommendation", "advice", "Advice", "improvement", "Improvement")),
							}
							cat.Items = append(cat.Items, item)
						}
					}
				}

				result.Categories = append(result.Categories, cat)
			}
		}
	}

	normalizeTaskResult(result)
	return result
}

func AnalyzeTask(task *Task, lang string) (*TaskResult, error) {
	taskID := task.GetId()
	logs.Info("[analyze-task] start task=%s provider=%s lang=%s", taskID, task.Provider, lang)

	effectiveScale, err := GetTaskEffectiveScale(task)
	if err != nil {
		logs.Error("[analyze-task] GetTaskEffectiveScale failed task=%s: %v", taskID, err)
		task.AnalyzeError = fmt.Sprintf("获取评价量表失败: %v", err)
		return nil, fmt.Errorf(task.AnalyzeError)
	}
	if effectiveScale == "" {
		task.AnalyzeError = "任务量表不能为空"
		return nil, fmt.Errorf(task.AnalyzeError)
	}
	scaleRunes := utf8.RuneCountInString(effectiveScale)
	logs.Info("[analyze-task] rubric loaded task=%s scaleRef=%s rubricLen=%d runes", taskID, task.Scale, scaleRunes)

	if task.DocumentParseStatus == DocumentParseStatusNone || task.DocumentParseStatus == "" {
		if task.DocumentUrl == "" && task.DocumentResourceId == "" {
			task.AnalyzeError = "任务文档不能为空，请先上传文档"
			return nil, fmt.Errorf(task.AnalyzeError)
		}
		task.AnalyzeError = "任务文档已上传但未进行解析，请重新上传文档以触发解析"
		return nil, fmt.Errorf(task.AnalyzeError)
	}

	switch task.DocumentParseStatus {
	case DocumentParseStatusFailed:
		task.AnalyzeError = fmt.Sprintf("文档解析失败，无法进行分析: %s", task.DocumentError)
		return nil, fmt.Errorf(task.AnalyzeError)
	case DocumentParseStatusEmpty:
		task.AnalyzeError = "文档已上传但未提取到文本内容，可能是扫描件或空文档"
		return nil, fmt.Errorf(task.AnalyzeError)
	case DocumentParseStatusUnsupported:
		task.AnalyzeError = fmt.Sprintf("文档类型不支持: %s", task.DocumentError)
		return nil, fmt.Errorf(task.AnalyzeError)
	case DocumentParseStatusPending:
		task.AnalyzeError = "文档解析中，请稍候再试"
		return nil, fmt.Errorf(task.AnalyzeError)
	case DocumentParseStatusSuccess:
	default:
		task.AnalyzeError = fmt.Sprintf("未知的文档解析状态: %s，请重新上传文档", task.DocumentParseStatus)
		return nil, fmt.Errorf(task.AnalyzeError)
	}

	docRunes := utf8.RuneCountInString(task.DocumentText)
	logs.Info("[analyze-task] document ready task=%s documentLen=%d runes", taskID, docRunes)

	question := fmt.Sprintf(analyzeTaskPrompt, effectiveScale, task.DocumentText)
	promptRunes := utf8.RuneCountInString(question)
	logs.Info("[analyze-task] prompt built task=%s fullPromptLen=%d runes (rubric+template+document)", taskID, promptRunes)

	var answer string
	aiStart := time.Now()
	if strings.Contains(strings.ToLower(task.Name), "demo") {
		logs.Info("[analyze-task] using GetAnswerFake (task name contains \"demo\") task=%s", taskID)
		answer, _, err = GetAnswerFake(task.Provider, question, lang)
	} else {
		logs.Info("[analyze-task] calling AI model task=%s provider=%s (this may take several minutes)...", taskID, task.Provider)
		answer, _, err = GetAnswer(task.Provider, question, lang)
	}
	aiElapsed := time.Since(aiStart)
	if err != nil {
		logs.Error("[analyze-task] AI call failed task=%s after %v: %v", taskID, aiElapsed, err)
		task.AnalyzeError = fmt.Sprintf("从AI模型获取分析失败: %v", err)
		return nil, fmt.Errorf(task.AnalyzeError)
	}
	logs.Info("[analyze-task] AI returned task=%s elapsed=%v answerLen=%d bytes", taskID, aiElapsed, len(answer))

	jsonStr := extractJSON(answer)
	logs.Info("[analyze-task] extracted JSON task=%s jsonLen=%d bytes", taskID, len(jsonStr))

	var rawData map[string]interface{}
	if err = json.Unmarshal([]byte(jsonStr), &rawData); err != nil {
		logs.Error("[analyze-task] JSON unmarshal failed task=%s: %v\nraw: %s", taskID, err, jsonStr)
		truncated := answer
		if len(truncated) > 500 {
			truncated = truncated[:500] + "..."
		}
		task.AnalyzeError = fmt.Sprintf("AI返回的JSON解析失败: %v。请稍后重试。原始回复片段: %s", err, truncated)
		return nil, fmt.Errorf(task.AnalyzeError)
	}

	result := normalizeTaskResultFromMap(rawData)
	if len(result.Categories) == 0 {
		task.AnalyzeError = "AI返回的分析结果中没有评价维度，请检查文档内容或稍后重试"
		return nil, fmt.Errorf(task.AnalyzeError)
	}

	task.AnalyzeError = ""
	logs.Info("[analyze-task] done task=%s score=%.2f categories=%d", taskID, result.Score, len(result.Categories))
	return result, nil
}

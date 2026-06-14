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
	"sync"
	"time"

	"github.com/the-open-agent/openagent/util"
)

type ExecutionStepType string

const (
	StepTypeModelStart        ExecutionStepType = "model_start"
	StepTypeModelEnd          ExecutionStepType = "model_end"
	StepTypeReasoning         ExecutionStepType = "reasoning"
	StepTypeKnowledgeRetrieval ExecutionStepType = "knowledge_retrieval"
	StepTypeToolCallStart     ExecutionStepType = "tool_call_start"
	StepTypeToolCallEnd       ExecutionStepType = "tool_call_end"
	StepTypeToolError         ExecutionStepType = "tool_error"
	StepTypeRetry             ExecutionStepType = "retry"
	StepTypeFinalOutput       ExecutionStepType = "final_output"
	StepTypeInfo              ExecutionStepType = "info"
	StepTypeError             ExecutionStepType = "error"
)

type ExecutionStepStatus string

const (
	StepStatusPending   ExecutionStepStatus = "pending"
	StepStatusRunning   ExecutionStepStatus = "running"
	StepStatusCompleted ExecutionStepStatus = "completed"
	StepStatusFailed    ExecutionStepStatus = "failed"
	StepStatusSkipped   ExecutionStepStatus = "skipped"
)

type ExecutionStep struct {
	ID          string             `json:"id"`
	Type        ExecutionStepType  `json:"type"`
	Status      ExecutionStepStatus `json:"status"`
	Title       string             `json:"title"`
	Description string             `json:"description,omitempty"`
	StartTime   string             `json:"startTime"`
	EndTime     string             `json:"endTime,omitempty"`
	DurationMs  int64              `json:"durationMs,omitempty"`
	Input       string             `json:"input,omitempty"`
	Output      string             `json:"output,omitempty"`
	Error       string             `json:"error,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	Round       int                `json:"round,omitempty"`
	ParentID    string             `json:"parentId,omitempty"`
}

type ExecutionTracer struct {
	mu     sync.Mutex
	steps  []*ExecutionStep
	stepMap map[string]*ExecutionStep
	messageId string
}

func NewExecutionTracer(messageId string) *ExecutionTracer {
	return &ExecutionTracer{
		steps:     make([]*ExecutionStep, 0),
		stepMap:   make(map[string]*ExecutionStep),
		messageId: messageId,
	}
}

func (t *ExecutionTracer) AddStep(step *ExecutionStep) string {
	t.mu.Lock()
	defer t.mu.Unlock()

	if step.ID == "" {
		step.ID = fmt.Sprintf("step_%s", util.GetRandomName())
	}
	if step.StartTime == "" {
		step.StartTime = util.GetCurrentTimeWithMilli()
	}
	if step.Status == "" {
		step.Status = StepStatusRunning
	}

	t.steps = append(t.steps, step)
	t.stepMap[step.ID] = step
	return step.ID
}

func (t *ExecutionTracer) StartStep(stepType ExecutionStepType, title string, metadata map[string]interface{}) string {
	step := &ExecutionStep{
		Type:      stepType,
		Status:    StepStatusRunning,
		Title:     title,
		StartTime: util.GetCurrentTimeWithMilli(),
		Metadata:  metadata,
	}
	return t.AddStep(step)
}

func (t *ExecutionTracer) UpdateStep(stepId string, updates map[string]interface{}) {
	t.mu.Lock()
	defer t.mu.Unlock()

	step, ok := t.stepMap[stepId]
	if !ok {
		return
	}

	if status, ok := updates["status"].(ExecutionStepStatus); ok {
		step.Status = status
	}
	if title, ok := updates["title"].(string); ok {
		step.Title = title
	}
	if description, ok := updates["description"].(string); ok {
		step.Description = description
	}
	if input, ok := updates["input"].(string); ok {
		step.Input = input
	}
	if output, ok := updates["output"].(string); ok {
		step.Output = output
	}
	if errorMsg, ok := updates["error"].(string); ok {
		step.Error = errorMsg
	}
	if round, ok := updates["round"].(int); ok {
		step.Round = round
	}
	if metadata, ok := updates["metadata"].(map[string]interface{}); ok {
		if step.Metadata == nil {
			step.Metadata = make(map[string]interface{})
		}
		for k, v := range metadata {
			step.Metadata[k] = v
		}
	}
}

func (t *ExecutionTracer) EndStep(stepId string, status ExecutionStepStatus, output string, errorMsg string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	step, ok := t.stepMap[stepId]
	if !ok {
		return
	}

	step.Status = status
	step.EndTime = util.GetCurrentTimeWithMilli()
	if output != "" {
		step.Output = output
	}
	if errorMsg != "" {
		step.Error = errorMsg
	}

	if step.StartTime != "" && step.EndTime != "" {
		startTime, err1 := time.Parse(time.RFC3339Nano, step.StartTime)
		endTime, err2 := time.Parse(time.RFC3339Nano, step.EndTime)
		if err1 == nil && err2 == nil {
			step.DurationMs = endTime.Sub(startTime).Milliseconds()
		}
	}
}

func (t *ExecutionTracer) AddSimpleStep(stepType ExecutionStepType, title string, description string, metadata map[string]interface{}) string {
	step := &ExecutionStep{
		Type:        stepType,
		Status:      StepStatusCompleted,
		Title:       title,
		Description: description,
		StartTime:   util.GetCurrentTimeWithMilli(),
		EndTime:     util.GetCurrentTimeWithMilli(),
		Metadata:    metadata,
	}
	return t.AddStep(step)
}

func (t *ExecutionTracer) GetSteps() []*ExecutionStep {
	t.mu.Lock()
	defer t.mu.Unlock()

	result := make([]*ExecutionStep, len(t.steps))
	copy(result, t.steps)
	return result
}

func (t *ExecutionTracer) ToJSON() (string, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	bytes, err := json.Marshal(t.steps)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

func (t *ExecutionTracer) LoadFromJSON(jsonStr string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	var steps []*ExecutionStep
	err := json.Unmarshal([]byte(jsonStr), &steps)
	if err != nil {
		return err
	}

	t.steps = steps
	t.stepMap = make(map[string]*ExecutionStep)
	for _, step := range steps {
		t.stepMap[step.ID] = step
	}
	return nil
}

func GetMessageExecutionSteps(messageId string) ([]*ExecutionStep, error) {
	message, err := GetMessage(messageId)
	if err != nil {
		return nil, err
	}
	if message == nil {
		return nil, fmt.Errorf("message not found")
	}

	if message.ExecutionSteps == "" {
		return []*ExecutionStep{}, nil
	}

	var steps []*ExecutionStep
	err = json.Unmarshal([]byte(message.ExecutionSteps), &steps)
	if err != nil {
		return nil, err
	}
	return steps, nil
}

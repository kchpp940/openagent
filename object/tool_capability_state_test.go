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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCapabilityStatus_IsAtLeast(t *testing.T) {
	assert.True(t, CapabilityStatusPass.IsAtLeast(CapabilityStatusPass))
	assert.True(t, CapabilityStatusPass.IsAtLeast(CapabilityStatusWarning))
	assert.True(t, CapabilityStatusPass.IsAtLeast(CapabilityStatusFail))
	assert.True(t, CapabilityStatusStale.IsAtLeast(CapabilityStatusWarning))
	assert.False(t, CapabilityStatusWarning.IsAtLeast(CapabilityStatusPass))
	assert.False(t, CapabilityStatusFail.IsAtLeast(CapabilityStatusWarning))
}

func TestCheckToolCapability(t *testing.T) {
	t.Run("tool with empty name should fail", func(t *testing.T) {
		tool := &Tool{
			Owner: "admin",
			Name:  "",
			Type:  "time",
			State: "Active",
		}
		result := CheckToolCapability(tool)
		assert.Equal(t, EntityTypeTool, result.EntityType)
		assert.Equal(t, CapabilityStatusFail, result.Status)
		assert.False(t, result.CanMount)
		assert.True(t, len(result.Issues) > 0)
	})

	t.Run("tool with inactive state should fail", func(t *testing.T) {
		tool := &Tool{
			Owner: "admin",
			Name:  "test_tool",
			Type:  "time",
			State: "Inactive",
		}
		result := CheckToolCapability(tool)
		assert.Equal(t, CapabilityStatusFail, result.Status)
		assert.False(t, result.CanMount)
	})

	t.Run("normal active tool should pass", func(t *testing.T) {
		tool := &Tool{
			Owner: "admin",
			Name:  "test_tool",
			Type:  "time",
			State: "Active",
		}
		result := CheckToolCapability(tool)
		assert.Equal(t, CapabilityStatusPass, result.Status)
		assert.True(t, result.CanMount)
		assert.True(t, result.CanSaveStore)
		assert.False(t, result.NeedReview)
	})

	t.Run("google search with missing keys should have warning", func(t *testing.T) {
		tool := &Tool{
			Owner:        "admin",
			Name:         "test_search",
			Type:         "web_search",
			SubType:      "Google",
			ClientId:     "",
			ClientSecret: "",
			State:        "Active",
		}
		result := CheckToolCapability(tool)
		assert.Equal(t, CapabilityStatusWarning, result.Status)
		assert.True(t, result.CanMount)
		assert.True(t, result.CanSaveStore)
		assert.True(t, result.NeedReview)
	})
}

func TestCheckSkillCapability(t *testing.T) {
	t.Run("normal active skill should pass", func(t *testing.T) {
		skill := &Skill{
			Owner:   "admin",
			Name:    "test_skill",
			Content: "test content",
			State:   "Active",
		}
		result := CheckSkillCapability(skill)
		assert.Equal(t, CapabilityStatusPass, result.Status)
		assert.True(t, result.CanMount)
	})

	t.Run("skill with empty content should have warning", func(t *testing.T) {
		skill := &Skill{
			Owner:   "admin",
			Name:    "test_skill",
			Content: "",
			State:   "Active",
		}
		result := CheckSkillCapability(skill)
		assert.Equal(t, CapabilityStatusWarning, result.Status)
		assert.True(t, result.CanMount)
		assert.True(t, result.NeedReview)
	})
}

func TestCheckServerCapability(t *testing.T) {
	t.Run("server with empty URL should fail", func(t *testing.T) {
		server := &Server{
			Owner: "admin",
			Name:  "test_server",
			Url:   "",
		}
		result := CheckServerCapability(server)
		assert.Equal(t, CapabilityStatusFail, result.Status)
		assert.False(t, result.CanMount)
	})

	t.Run("server with invalid URL should have warning", func(t *testing.T) {
		server := &Server{
			Owner: "admin",
			Name:  "test_server",
			Url:   "not-a-valid-url",
			Tools: []*McpTool{
				{Name: "tool1", IsAllowed: true},
			},
		}
		result := CheckServerCapability(server)
		assert.Equal(t, CapabilityStatusWarning, result.Status)
		assert.True(t, result.CanMount)
	})

	t.Run("server with no tools synced should be pending", func(t *testing.T) {
		server := &Server{
			Owner: "admin",
			Name:  "test_server",
			Url:   "http://localhost:3000",
			Tools: nil,
		}
		result := CheckServerCapability(server)
		assert.Equal(t, CapabilityStatusPending, result.Status)
		assert.False(t, result.CanMount)
	})

	t.Run("server with valid URL and allowed tools should pass", func(t *testing.T) {
		server := &Server{
			Owner: "admin",
			Name:  "test_server",
			Url:   "http://localhost:3000",
			Tools: []*McpTool{
				{Name: "tool1", IsAllowed: true},
				{Name: "tool2", IsAllowed: false},
			},
		}
		result := CheckServerCapability(server)
		assert.Equal(t, CapabilityStatusPass, result.Status)
		assert.True(t, result.CanMount)
	})
}

func TestCheckToolMountable(t *testing.T) {
	assert.True(t, CheckToolMountable(&Tool{Owner: "admin", Name: "t1", Type: "time", State: "Active"}))
	assert.False(t, CheckToolMountable(&Tool{Owner: "admin", Name: "t1", Type: "time", State: "Inactive"}))
	assert.False(t, CheckToolMountable(&Tool{Owner: "admin", Name: "", Type: "time", State: "Active"}))
}

func TestCheckSkillMountable(t *testing.T) {
	assert.True(t, CheckSkillMountable(&Skill{Owner: "admin", Name: "s1", Content: "test", State: "Active"}))
	assert.False(t, CheckSkillMountable(&Skill{Owner: "admin", Name: "s1", Content: "test", State: "Inactive"}))
}

func TestCheckServerMountable(t *testing.T) {
	assert.True(t, CheckServerMountable(&Server{
		Owner: "admin", Name: "s1", Url: "http://localhost:3000",
		Tools: []*McpTool{{Name: "t1", IsAllowed: true}},
	}))
	assert.False(t, CheckServerMountable(&Server{
		Owner: "admin", Name: "s1", Url: "",
	}))
}

func TestValidateAndNormalizeToolState(t *testing.T) {
	tool := &Tool{Owner: "admin", Name: "t1", Type: "time", State: ""}
	err := ValidateAndNormalizeToolState(tool)
	assert.NoError(t, err)
	assert.Equal(t, "Active", tool.State)

	tool.State = "Invalid"
	err = ValidateAndNormalizeToolState(tool)
	assert.NoError(t, err)
	assert.Equal(t, "Active", tool.State)
}

func TestStatusToLegacyStateConversion(t *testing.T) {
	assert.Equal(t, LegacyStateActive, statusToLegacyState(CapabilityStatusPass))
	assert.Equal(t, LegacyStateActive, statusToLegacyState(CapabilityStatusWarning))
	assert.Equal(t, LegacyStateActive, statusToLegacyState(CapabilityStatusStale))
	assert.Equal(t, LegacyStateInactive, statusToLegacyState(CapabilityStatusFail))
	assert.Equal(t, LegacyStateInactive, statusToLegacyState(CapabilityStatusPending))
}

func TestLegacyStateToStatusConversion(t *testing.T) {
	assert.Equal(t, CapabilityStatusPass, legacyStateToStatus(LegacyStateActive))
	assert.Equal(t, CapabilityStatusFail, legacyStateToStatus(LegacyStateInactive))
	assert.Equal(t, CapabilityStatusPending, legacyStateToStatus(LegacyState("")))
}

func TestCapabilityDisplayText(t *testing.T) {
	assert.Equal(t, "通过", CapabilityStatusPass.DisplayText())
	assert.Equal(t, "警告", CapabilityStatusWarning.DisplayText())
	assert.Equal(t, "失败", CapabilityStatusFail.DisplayText())
	assert.Equal(t, "待检测", CapabilityStatusPending.DisplayText())
	assert.Equal(t, "需刷新", CapabilityStatusStale.DisplayText())
}

func TestCapabilityTagColor(t *testing.T) {
	assert.Equal(t, "success", CapabilityStatusPass.TagColor())
	assert.Equal(t, "warning", CapabilityStatusWarning.TagColor())
	assert.Equal(t, "error", CapabilityStatusFail.TagColor())
	assert.Equal(t, "default", CapabilityStatusPending.TagColor())
	assert.Equal(t, "processing", CapabilityStatusStale.TagColor())
}

package object

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/the-open-agent/openagent/mcp"
	"github.com/the-open-agent/openagent/tool"
	"github.com/the-open-agent/openagent/util"
)

const (
	capabilityStaleHours = 24 * 7
)

// ---------------------------------------------------------------------------
// Core types
// ---------------------------------------------------------------------------

type CapabilityState string

const (
	CapabilityStateActive   CapabilityState = "Active"
	CapabilityStatePending  CapabilityState = "Pending"
	CapabilityStateStale    CapabilityState = "Stale"
	CapabilityStateError    CapabilityState = "Error"
)

func (s CapabilityState) IsValid() bool {
	switch s {
	case CapabilityStateActive, CapabilityStatePending, CapabilityStateStale, CapabilityStateError:
		return true
	}
	return false
}

func normalizeCapabilityState(raw string) CapabilityState {
	s := CapabilityState(raw)
	if s.IsValid() {
		return s
	}
	return CapabilityStatePending
}

// CapabilityDecision is the unified output produced for every
// Server/Skill/Tool/Store entity by the single entry-point decision function.
type CapabilityDecision struct {
	State        CapabilityState `json:"state"`
	CanMount     bool            `json:"canMount"`
	CanSaveStore bool            `json:"canSaveStore,omitempty"`
	NeedsRecheck bool            `json:"needsRecheck"`
	BlockReason  string          `json:"blockReason,omitempty"`
	Warnings     []string        `json:"warnings,omitempty"`
	CheckedAt    string          `json:"checkedAt,omitempty"`
	ConfigHash   string          `json:"configHash,omitempty"`

	FailedResources []FailedResource `json:"failedResources,omitempty"`
}

type FailedResource struct {
	Kind   string `json:"kind"`
	Name   string `json:"name"`
	Reason string `json:"reason,omitempty"`
	State  string `json:"state,omitempty"`
}

// Deprecated: kept for API-compat, reads from corresponding Decision fields.
type CapabilityInfo struct {
	State        CapabilityState `json:"state"`
	CanMount     bool            `json:"canMount"`
	NeedsRecheck bool            `json:"needsRecheck"`
	Reason       string          `json:"reason,omitempty"`
}

func decisionToInfo(d *CapabilityDecision) *CapabilityInfo {
	if d == nil {
		return nil
	}
	return &CapabilityInfo{
		State:        d.State,
		CanMount:     d.CanMount,
		NeedsRecheck: d.NeedsRecheck,
		Reason:       d.BlockReason,
	}
}

// ---------------------------------------------------------------------------
// Config hash helpers
// ---------------------------------------------------------------------------

func hashStruct(v interface{}) string {
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func serverConfigHash(s *Server) string {
	return hashStruct(struct {
		U         string
		T         string
		ToolsHash string
	}{
		U: s.Url,
		T: s.Token,
		ToolsHash: hashStruct(s.Tools),
	})
}

func skillConfigHash(s *Skill) string {
	return hashStruct(struct {
		C  string
		S  string
		R  []SkillReference
	}{
		C: s.Content,
		S: s.SkillMd,
		R: s.References,
	})
}

func toolConfigHash(t *Tool) string {
	cfg := getToolConfig(t)
	return hashStruct(struct {
		Type         string
		SubType      string
		ProviderUrl  string
		ClientId     string
		ClientSecret string
		EnableProxy  bool
		Mode         string
	}{
		Type:         cfg.Type,
		SubType:      cfg.SubType,
		ProviderUrl:  cfg.ProviderUrl,
		ClientId:     cfg.ClientId,
		ClientSecret: cfg.ClientSecret,
		EnableProxy:  cfg.EnableProxy,
		Mode:         cfg.Mode,
	})
}

func storeConfigHash(store *Store) string {
	return hashStruct(struct {
		MP  string
		MS  string
		S   []string
		T   []string
		VSI string
	}{
		MP:  store.ModelProvider,
		MS:  store.McpServer,
		S:   store.Skills,
		T:   store.Tools,
		VSI: store.VectorStoreId,
	})
}

// ---------------------------------------------------------------------------
// Last-check timestamp + staleness helpers
// ---------------------------------------------------------------------------

func isStale(lastChecked string, currentHash, lastHash string) bool {
	if strings.TrimSpace(lastChecked) == "" {
		return true
	}
	if currentHash != lastHash && strings.TrimSpace(lastHash) != "" {
		return true
	}
	t, err := time.Parse(time.RFC3339, lastChecked)
	if err != nil {
		return true
	}
	return time.Since(t) > capabilityStaleHours*time.Hour
}

func nowTimestamp() string {
	return time.Now().UTC().Format(time.RFC3339)
}

// ---------------------------------------------------------------------------
// SINGLE ENTRY-POINT: ComputeCapabilityDecision
//
//     Store save, MergeMcpTools, BuildMcpToolSet, Builtin Registry
//     ALL go through this function.
// ---------------------------------------------------------------------------

type capabilityEntity string

const (
	entityServer capabilityEntity = "server"
	entitySkill  capabilityEntity = "skill"
	entityTool   capabilityEntity = "tool"
	entityStore  capabilityEntity = "store"
)

type decisionInputs struct {
	kind                capabilityEntity
	explicitDisabled    bool
	currentConfigHash   string
	lastChecked         string
	lastCheckHash       string
	lastStatus          CapabilityState
	lastError           string
	staticChecks        func() (blockReason string, warnings []string)
	runtimeCheck        func(lang string) (warnings []string, err error)
	extraStoreValidators []func() (blockReason string, warnings []string)
}

// computeCapabilityDecision is the single decision function.
//
// It runs:
//   1. nil / static field validation (cheap)
//   2. explicit-disabled override
//   3. cached last-check validity check
//   4. when stale or missing: the supplied runtimeCheck (expensive, e.g. dials MCP URL)
//
// and always returns the same output shape.
func computeCapabilityDecision(inputs *decisionInputs, runRuntime bool, lang string) (*CapabilityDecision, error) {
	d := &CapabilityDecision{
		ConfigHash: inputs.currentConfigHash,
	}

	if inputs.staticChecks != nil {
		reason, warnings := inputs.staticChecks()
		if reason != "" {
			d.State = CapabilityStateError
			d.CanMount = false
			d.NeedsRecheck = false
			d.BlockReason = reason
			d.Warnings = warnings
			return d, nil
		}
		d.Warnings = append(d.Warnings, warnings...)
	}

	if inputs.explicitDisabled {
		d.State = CapabilityStatePending
		d.CanMount = false
		d.NeedsRecheck = true
		d.BlockReason = "explicitly disabled"
		return d, nil
	}

	cachedValid := !isStale(inputs.lastChecked, inputs.currentConfigHash, inputs.lastCheckHash) && inputs.lastStatus.IsValid()
	if cachedValid {
		d.State = inputs.lastStatus
		d.CanMount = inputs.lastStatus == CapabilityStateActive
		d.NeedsRecheck = inputs.lastStatus == CapabilityStateStale || inputs.lastStatus == CapabilityStatePending
		d.BlockReason = inputs.lastError
		d.CheckedAt = inputs.lastChecked
		if d.State == CapabilityStateActive {
			return d, nil
		}
		if d.State == CapabilityStateError {
			return d, nil
		}
	}

	if !runRuntime || inputs.runtimeCheck == nil {
		if cachedValid {
			return d, nil
		}
		d.State = CapabilityStatePending
		d.CanMount = false
		d.NeedsRecheck = true
		d.BlockReason = "capability check pending"
		return d, nil
	}

	warnings, rerr := inputs.runtimeCheck(lang)
	d.Warnings = append(d.Warnings, warnings...)
	d.CheckedAt = nowTimestamp()

	if rerr != nil {
		d.State = CapabilityStateError
		d.CanMount = false
		d.NeedsRecheck = true
		d.BlockReason = rerr.Error()
		return d, nil
	}

	d.State = CapabilityStateActive
	d.CanMount = true
	d.NeedsRecheck = false
	return d, nil
}

// ---------------------------------------------------------------------------
// Public per-entity decision wrappers
// ---------------------------------------------------------------------------

// GetServerCapabilityDecision returns the computed CapabilityDecision for a Server.
// When runRuntime is true it actually dials the MCP URL and lists tools.
func GetServerCapabilityDecision(s *Server, runRuntime bool, lang string) (*CapabilityDecision, error) {
	if s == nil {
		return &CapabilityDecision{
			State:       CapabilityStateError,
			CanMount:    false,
			BlockReason: "server is nil",
		}, nil
	}

	inputs := &decisionInputs{
		kind:              entityServer,
		currentConfigHash: serverConfigHash(s),
		lastChecked:       s.LastCapabilityCheck,
		lastCheckHash:     s.LastCapabilityHash,
		lastStatus:        normalizeCapabilityState(s.LastCapabilityStatus),
		lastError:         s.LastCapabilityError,
		staticChecks: func() (string, []string) {
			if strings.TrimSpace(s.Url) == "" {
				return "server URL is empty", nil
			}
			var warns []string
			if len(s.Tools) == 0 {
				warns = append(warns, "tools list not yet synced")
			}
			return "", warns
		},
		runtimeCheck: func(lang string) ([]string, error) {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			cli, err := mcp.NewClient(s.Url, s.Token)
			if err != nil {
				return nil, err
			}
			defer cli.Close()
			list, lerr := cli.ListTools(ctx)
			if lerr != nil {
				return nil, lerr
			}
			var warns []string
			if len(list.Tools) == 0 {
				warns = append(warns, "remote MCP server reported zero tools")
			}
			return warns, nil
		},
	}
	return computeCapabilityDecision(inputs, runRuntime, lang)
}

// GetSkillCapabilityDecision returns the computed CapabilityDecision for a Skill.
func GetSkillCapabilityDecision(s *Skill, runRuntime bool, lang string) (*CapabilityDecision, error) {
	if s == nil {
		return &CapabilityDecision{
			State:       CapabilityStateError,
			CanMount:    false,
			BlockReason: "skill is nil",
		}, nil
	}

	inputs := &decisionInputs{
		kind:              entitySkill,
		currentConfigHash: skillConfigHash(s),
		lastChecked:       s.LastCapabilityCheck,
		lastCheckHash:     s.LastCapabilityHash,
		lastStatus:        normalizeCapabilityState(s.LastCapabilityStatus),
		lastError:         s.LastCapabilityError,
		staticChecks: func() (string, []string) {
			content := strings.TrimSpace(s.Content)
			md := strings.TrimSpace(s.SkillMd)
			if content == "" && md == "" {
				return "skill has no content and no skillMd", nil
			}
			var warns []string
			if strings.TrimSpace(s.Description) == "" {
				warns = append(warns, "skill description is empty")
			}
			if strings.TrimSpace(s.Type) == "" {
				warns = append(warns, "skill type is empty")
			}
			return "", warns
		},
		runtimeCheck: nil,
	}
	return computeCapabilityDecision(inputs, runRuntime, lang)
}

// GetToolCapabilityDecision returns the computed CapabilityDecision for a Tool.
// When runRuntime is true it actually constructs the builtin tool provider.
func GetToolCapabilityDecision(t *Tool, runRuntime bool, lang string) (*CapabilityDecision, error) {
	if t == nil {
		return &CapabilityDecision{
			State:       CapabilityStateError,
			CanMount:    false,
			BlockReason: "tool is nil",
		}, nil
	}

	inputs := &decisionInputs{
		kind:              entityTool,
		currentConfigHash: toolConfigHash(t),
		lastChecked:       t.LastCapabilityCheck,
		lastCheckHash:     t.LastCapabilityHash,
		lastStatus:        normalizeCapabilityState(t.LastCapabilityStatus),
		lastError:         t.LastCapabilityError,
		staticChecks: func() (string, []string) {
			if strings.TrimSpace(t.Type) == "" {
				return "tool type is empty", nil
			}
			return "", nil
		},
		runtimeCheck: func(lang string) ([]string, error) {
			cfg := getToolConfig(t)
			realSecret := cfg.ClientSecret
			if realSecret == "***" {
				dbT, err := getTool(t.Owner, t.Name)
				if err != nil {
					return nil, err
				}
				if dbT == nil {
					return nil, fmt.Errorf("tool not found for owner=%s name=%s", t.Owner, t.Name)
				}
				cfg.ClientSecret = dbT.ClientSecret
			}
			tp, err := tool.New(cfg, lang)
			if err != nil {
				return nil, err
			}
			var warns []string
			if len(tp.BuiltinTools()) == 0 {
				warns = append(warns, "tool provider returned zero builtin tools")
			}
			return warns, nil
		},
	}
	return computeCapabilityDecision(inputs, runRuntime, lang)
}

// GetStoreCapabilityDecision returns the computed CapabilityDecision for a Store,
// including the transitive validation of every Server/Skill/Tool it references.
func GetStoreCapabilityDecision(store *Store, runRuntime bool, lang string) (*CapabilityDecision, error) {
	if store == nil {
		return &CapabilityDecision{
			State:           CapabilityStateError,
			CanMount:        false,
			CanSaveStore:    false,
			BlockReason:     "store is nil",
			FailedResources: nil,
		}, nil
	}

	var blockReasons []string
	var warnings []string
	var failedResources []FailedResource

	if strings.TrimSpace(store.Name) == "" {
		blockReasons = append(blockReasons, "store name is empty")
	}
	if strings.TrimSpace(store.ModelProvider) == "" {
		blockReasons = append(blockReasons, "model provider is not configured")
	}

	if store.McpServer != "" {
		srv, err := GetServerByOwnerAndName(store.Owner, store.McpServer)
		if err != nil || srv == nil {
			blockReasons = append(blockReasons, fmt.Sprintf("MCP server not found: %s", store.McpServer))
			failedResources = append(failedResources, FailedResource{
				Kind:   "server",
				Name:   store.McpServer,
				Reason: "not found",
				State:  string(CapabilityStateError),
			})
		} else {
			d, derr := GetServerCapabilityDecision(srv, runRuntime, lang)
			if derr != nil {
				blockReasons = append(blockReasons, fmt.Sprintf("MCP server check error: %v", derr))
				failedResources = append(failedResources, FailedResource{
					Kind:   "server",
					Name:   srv.Name,
					Reason: fmt.Sprintf("check error: %v", derr),
					State:  string(CapabilityStateError),
				})
			} else if !d.CanMount {
				blockReasons = append(blockReasons, fmt.Sprintf("MCP server: %s", firstNonEmpty(d.BlockReason, "not mountable")))
				failedResources = append(failedResources, FailedResource{
					Kind:   "server",
					Name:   srv.Name,
					Reason: d.BlockReason,
					State:  string(d.State),
				})
			}
			warnings = append(warnings, d.Warnings...)
		}
	}

	skills, serr := resolveEnabledSkills(store.Owner, store.Skills)
	if serr == nil {
		for _, s := range skills {
			d, derr := GetSkillCapabilityDecision(s, runRuntime, lang)
			if derr != nil {
				blockReasons = append(blockReasons, fmt.Sprintf("skill %s check error: %v", s.Name, derr))
				failedResources = append(failedResources, FailedResource{
					Kind:   "skill",
					Name:   s.Name,
					Reason: fmt.Sprintf("check error: %v", derr),
					State:  string(CapabilityStateError),
				})
			} else if !d.CanMount {
				warnings = append(warnings, fmt.Sprintf("skill %s: %s", s.Name, firstNonEmpty(d.BlockReason, "pending")))
				failedResources = append(failedResources, FailedResource{
					Kind:   "skill",
					Name:   s.Name,
					Reason: d.BlockReason,
					State:  string(d.State),
				})
			}
			warnings = append(warnings, d.Warnings...)
		}
	}

	toolNames := store.Tools
	if len(toolNames) == 1 && toolNames[0] == "All" {
		if allTools, aerr := GetTools(store.Owner); aerr == nil {
			toolNames = make([]string, 0, len(allTools))
			for _, t := range allTools {
				toolNames = append(toolNames, t.Name)
			}
		}
	}
	for _, tname := range toolNames {
		id := util.GetIdFromOwnerAndName(store.Owner, tname)
		t, terr := GetTool(id)
		if terr != nil || t == nil {
			warnings = append(warnings, fmt.Sprintf("tool not found: %s", tname))
			failedResources = append(failedResources, FailedResource{
				Kind:   "tool",
				Name:   tname,
				Reason: "not found",
				State:  string(CapabilityStateError),
			})
			continue
		}
		d, derr := GetToolCapabilityDecision(t, runRuntime, lang)
		if derr != nil {
			warnings = append(warnings, fmt.Sprintf("tool %s check error: %v", tname, derr))
			failedResources = append(failedResources, FailedResource{
				Kind:   "tool",
				Name:   t.Name,
				Reason: fmt.Sprintf("check error: %v", derr),
				State:  string(CapabilityStateError),
			})
		} else if !d.CanMount {
			warnings = append(warnings, fmt.Sprintf("tool %s: %s", tname, firstNonEmpty(d.BlockReason, "pending")))
			failedResources = append(failedResources, FailedResource{
				Kind:   "tool",
				Name:   t.Name,
				Reason: d.BlockReason,
				State:  string(d.State),
			})
		}
		warnings = append(warnings, d.Warnings...)
	}

	inputs := &decisionInputs{
		kind:              entityStore,
		currentConfigHash: storeConfigHash(store),
		lastChecked:       store.LastCapabilityCheck,
		lastCheckHash:     store.LastCapabilityHash,
		lastStatus:        normalizeCapabilityState(store.LastCapabilityStatus),
		lastError:         store.LastCapabilityError,
		staticChecks: func() (string, []string) {
			if len(blockReasons) > 0 {
				return strings.Join(blockReasons, "; "), warnings
			}
			return "", warnings
		},
		runtimeCheck: nil,
	}

	d, err := computeCapabilityDecision(inputs, runRuntime, lang)
	if err != nil {
		return d, err
	}
	d.CanSaveStore = d.State == CapabilityStateActive
	if len(warnings) > 0 && d.Warnings == nil {
		d.Warnings = warnings
	}
	if len(failedResources) > 0 {
		d.FailedResources = failedResources
	}
	return d, nil
}

func firstNonEmpty(s ...string) string {
	for _, v := range s {
		if v != "" {
			return v
		}
	}
	return ""
}

// ---------------------------------------------------------------------------
// Backwards-compat Info helpers (thin wrappers over the new Decision API)
// ---------------------------------------------------------------------------

func GetServerCapabilityInfo(s *Server) *CapabilityInfo {
	d, _ := GetServerCapabilityDecision(s, false, "")
	return decisionToInfo(d)
}

func GetSkillCapabilityInfo(s *Skill) *CapabilityInfo {
	d, _ := GetSkillCapabilityDecision(s, false, "")
	return decisionToInfo(d)
}

func GetToolCapabilityInfo(t *Tool) *CapabilityInfo {
	d, _ := GetToolCapabilityDecision(t, false, "")
	return decisionToInfo(d)
}

func CanSaveStore(store *Store) *CapabilityInfo {
	d, _ := GetStoreCapabilityDecision(store, false, "")
	info := decisionToInfo(d)
	if info != nil && d != nil {
		info.Reason = firstNonEmpty(info.Reason, strings.Join(d.Warnings, "; "))
	}
	return info
}

func ValidateStoreCapabilities(store *Store) []CapabilityInfo {
	var infos []CapabilityInfo

	addInfo := func(d *CapabilityDecision, label string) {
		if d == nil {
			return
		}
		if !d.CanMount || d.State != CapabilityStateActive {
			reason := firstNonEmpty(d.BlockReason, strings.Join(d.Warnings, "; "))
			if label != "" {
				reason = label + ": " + reason
			}
			infos = append(infos, CapabilityInfo{
				State:        d.State,
				CanMount:     d.CanMount,
				NeedsRecheck: d.NeedsRecheck,
				Reason:       reason,
			})
		}
	}

	if store.McpServer != "" {
		srv, err := GetServerByOwnerAndName(store.Owner, store.McpServer)
		if err != nil || srv == nil {
			infos = append(infos, CapabilityInfo{
				State:    CapabilityStateError,
				CanMount: false,
				Reason:   fmt.Sprintf("MCP server not found: %s", store.McpServer),
			})
		} else {
			d, _ := GetServerCapabilityDecision(srv, false, "")
			addInfo(d, "MCP server")
		}
	}

	skills, err := resolveEnabledSkills(store.Owner, store.Skills)
	if err == nil {
		for _, s := range skills {
			d, _ := GetSkillCapabilityDecision(s, false, "")
			addInfo(d, fmt.Sprintf("skill %s", s.Name))
		}
	}

	toolNames := store.Tools
	if len(toolNames) == 1 && toolNames[0] == "All" {
		if allTools, aerr := GetTools(store.Owner); aerr == nil {
			toolNames = make([]string, 0, len(allTools))
			for _, t := range allTools {
				toolNames = append(toolNames, t.Name)
			}
		}
	}
	for _, tname := range toolNames {
		id := util.GetIdFromOwnerAndName(store.Owner, tname)
		t, terr := GetTool(id)
		if terr != nil || t == nil {
			infos = append(infos, CapabilityInfo{
				State:    CapabilityStateError,
				CanMount: false,
				Reason:   fmt.Sprintf("tool not found: %s", tname),
			})
			continue
		}
		d, _ := GetToolCapabilityDecision(t, false, "")
		addInfo(d, fmt.Sprintf("tool %s", tname))
	}

	return infos
}

// ---------------------------------------------------------------------------
// Capability-check persistence helpers
// ---------------------------------------------------------------------------

func applyServerDecisionResult(s *Server, d *CapabilityDecision) {
	if d == nil || s == nil {
		return
	}
	s.LastCapabilityCheck = d.CheckedAt
	s.LastCapabilityHash = d.ConfigHash
	s.LastCapabilityStatus = string(d.State)
	s.LastCapabilityError = d.BlockReason
}

func applySkillDecisionResult(s *Skill, d *CapabilityDecision) {
	if d == nil || s == nil {
		return
	}
	s.LastCapabilityCheck = d.CheckedAt
	s.LastCapabilityHash = d.ConfigHash
	s.LastCapabilityStatus = string(d.State)
	s.LastCapabilityError = d.BlockReason
}

func applyToolDecisionResult(t *Tool, d *CapabilityDecision) {
	if d == nil || t == nil {
		return
	}
	t.LastCapabilityCheck = d.CheckedAt
	t.LastCapabilityHash = d.ConfigHash
	t.LastCapabilityStatus = string(d.State)
	t.LastCapabilityError = d.BlockReason
}

func applyStoreDecisionResult(store *Store, d *CapabilityDecision) {
	if d == nil || store == nil {
		return
	}
	store.LastCapabilityCheck = d.CheckedAt
	store.LastCapabilityHash = d.ConfigHash
	store.LastCapabilityStatus = string(d.State)
	store.LastCapabilityError = d.BlockReason
}

// RunServerCapabilityCheck executes the actual runtime check against the
// server URL, persists the result back onto the model, and returns the
// updated server and its decision.
func RunServerCapabilityCheck(s *Server, lang string) (*Server, *CapabilityDecision, error) {
	d, err := GetServerCapabilityDecision(s, true, lang)
	if err != nil {
		return s, d, err
	}
	applyServerDecisionResult(s, d)
	return s, d, nil
}

func RunSkillCapabilityCheck(s *Skill, lang string) (*Skill, *CapabilityDecision, error) {
	d, err := GetSkillCapabilityDecision(s, true, lang)
	if err != nil {
		return s, d, err
	}
	applySkillDecisionResult(s, d)
	return s, d, nil
}

func RunToolCapabilityCheck(t *Tool, lang string) (*Tool, *CapabilityDecision, error) {
	d, err := GetToolCapabilityDecision(t, true, lang)
	if err != nil {
		return t, d, err
	}
	applyToolDecisionResult(t, d)
	return t, d, nil
}

func RunStoreCapabilityCheck(store *Store, lang string) (*Store, *CapabilityDecision, error) {
	d, err := GetStoreCapabilityDecision(store, true, lang)
	if err != nil {
		return store, d, err
	}
	applyStoreDecisionResult(store, d)
	return store, d, nil
}

// ---------------------------------------------------------------------------
// Populate helpers - attach computed decision to the transient CapabilityInfo
// field on every entity before controller serialization.
// ---------------------------------------------------------------------------

func PopulateServerCapabilityInfo(server *Server) {
	if server != nil {
		server.CapabilityInfo = GetServerCapabilityInfo(server)
		server.CapabilityDecision = nil
	}
}

func PopulateServersCapabilityInfo(servers []*Server) {
	for _, s := range servers {
		PopulateServerCapabilityInfo(s)
	}
}

func PopulateServerCapabilityDecision(server *Server, runRuntime bool, lang string) {
	if server == nil {
		return
	}
	d, _ := GetServerCapabilityDecision(server, runRuntime, lang)
	server.CapabilityDecision = d
	server.CapabilityInfo = decisionToInfo(d)
}

func PopulateSkillCapabilityInfo(skill *Skill) {
	if skill != nil {
		skill.CapabilityInfo = GetSkillCapabilityInfo(skill)
		skill.CapabilityDecision = nil
	}
}

func PopulateSkillsCapabilityInfo(skills []*Skill) {
	for _, s := range skills {
		PopulateSkillCapabilityInfo(s)
	}
}

func PopulateSkillCapabilityDecision(skill *Skill, runRuntime bool, lang string) {
	if skill == nil {
		return
	}
	d, _ := GetSkillCapabilityDecision(skill, runRuntime, lang)
	skill.CapabilityDecision = d
	skill.CapabilityInfo = decisionToInfo(d)
}

func PopulateToolCapabilityInfo(t *Tool) {
	if t != nil {
		t.CapabilityInfo = GetToolCapabilityInfo(t)
		t.CapabilityDecision = nil
	}
}

func PopulateToolsCapabilityInfo(tools []*Tool) {
	for _, t := range tools {
		PopulateToolCapabilityInfo(t)
	}
}

func PopulateToolCapabilityDecision(t *Tool, runRuntime bool, lang string) {
	if t == nil {
		return
	}
	d, _ := GetToolCapabilityDecision(t, runRuntime, lang)
	t.CapabilityDecision = d
	t.CapabilityInfo = decisionToInfo(d)
}

func PopulateStoreCapabilityInfo(store *Store) {
	if store != nil {
		store.CapabilityInfo = CanSaveStore(store)
		store.CapabilityDecision = nil
	}
}

func PopulateStoreCapabilityDecision(store *Store, runRuntime bool, lang string) {
	if store == nil {
		return
	}
	d, _ := GetStoreCapabilityDecision(store, runRuntime, lang)
	store.CapabilityDecision = d
	store.CapabilityInfo = decisionToInfo(d)
}

// ---------------------------------------------------------------------------
// Error type
// ---------------------------------------------------------------------------

type CapabilityError struct {
	Decision *CapabilityDecision
}

func (e *CapabilityError) Error() string {
	if e.Decision == nil {
		return "capability check failed"
	}
	if e.Decision.BlockReason != "" {
		return e.Decision.BlockReason
	}
	return "capability check failed: " + string(e.Decision.State)
}

// SortCapabilityInfos returns a deterministic ordering for UI display:
// Active last, Error first.

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

package controllers

import (
	"fmt"

	"github.com/the-open-agent/openagent/object"
	"github.com/the-open-agent/openagent/util"
)

type ResourceType string

const (
	ResourceTypeTask     ResourceType = "task"
	ResourceTypeStore    ResourceType = "store"
	ResourceTypeServer   ResourceType = "server"
	ResourceTypeSkill    ResourceType = "skill"
	ResourceTypeTool     ResourceType = "tool"
	ResourceTypeResource ResourceType = "resource"
)

type AccessLevel int

const (
	AccessRead AccessLevel = iota
	AccessWrite
)

type ResolvedResource struct {
	Type   ResourceType
	Id     string
	Owner  string
	Name   string
	Object any
}

func (rr *ResolvedResource) Task() *object.Task {
	return rr.Object.(*object.Task)
}

func (rr *ResolvedResource) Store() *object.Store {
	return rr.Object.(*object.Store)
}

func (rr *ResolvedResource) Server() *object.Server {
	return rr.Object.(*object.Server)
}

func (rr *ResolvedResource) Skill() *object.Skill {
	return rr.Object.(*object.Skill)
}

func (rr *ResolvedResource) Tool() *object.Tool {
	return rr.Object.(*object.Tool)
}

func (rr *ResolvedResource) Resource() *object.Resource {
	return rr.Object.(*object.Resource)
}

func (c *ApiController) ResolveResource(resType ResourceType, id string) *ResolvedResource {
	if id == "" {
		c.ResponseError(c.T("resource:Resource id is required"))
		return nil
	}

	owner, name, err := util.GetOwnerAndNameFromIdWithError(id)
	if err != nil {
		c.ResponseError(c.T("resource:Invalid resource id format"))
		return nil
	}

	switch resType {
	case ResourceTypeTask:
		return c.resolveTask(id, owner, name)
	case ResourceTypeStore:
		return c.resolveStore(id, owner, name)
	case ResourceTypeServer:
		return c.resolveServer(id, owner, name)
	case ResourceTypeSkill:
		return c.resolveSkill(id, owner, name)
	case ResourceTypeTool:
		return c.resolveTool(id, owner, name)
	case ResourceTypeResource:
		return c.resolveResource(id, owner, name)
	default:
		c.ResponseError(fmt.Sprintf("unsupported resource type: %s", resType))
		return nil
	}
}

func (c *ApiController) resolveTask(id, owner, name string) *ResolvedResource {
	task, err := object.GetTask(id)
	if err != nil {
		c.ResponseError(err.Error())
		return nil
	}
	if task == nil {
		c.ResponseError(c.T("resource:The task does not exist"))
		return nil
	}
	return &ResolvedResource{Type: ResourceTypeTask, Id: id, Owner: owner, Name: name, Object: task}
}

func (c *ApiController) resolveStore(id, _, _ string) *ResolvedResource {
	store, err := object.GetStore(id)
	if err != nil {
		c.ResponseError(err.Error())
		return nil
	}
	if store == nil {
		store, err = object.GetStoreForGetApi(id)
		if err != nil {
			c.ResponseError(err.Error())
			return nil
		}
	}
	if store == nil {
		c.ResponseError(c.T("resource:The store does not exist"))
		return nil
	}
	return &ResolvedResource{Type: ResourceTypeStore, Id: id, Owner: store.Owner, Name: store.Name, Object: store}
}

func (c *ApiController) resolveServer(id, owner, name string) *ResolvedResource {
	server, err := object.GetServer(id)
	if err != nil {
		c.ResponseError(err.Error())
		return nil
	}
	if server == nil {
		c.ResponseError(c.T("resource:The server does not exist"))
		return nil
	}
	return &ResolvedResource{Type: ResourceTypeServer, Id: id, Owner: owner, Name: name, Object: server}
}

func (c *ApiController) resolveSkill(id, owner, name string) *ResolvedResource {
	skill, err := object.GetSkill(id)
	if err != nil {
		c.ResponseError(err.Error())
		return nil
	}
	if skill == nil {
		c.ResponseError(c.T("resource:The skill does not exist"))
		return nil
	}
	return &ResolvedResource{Type: ResourceTypeSkill, Id: id, Owner: owner, Name: name, Object: skill}
}

func (c *ApiController) resolveTool(id, owner, name string) *ResolvedResource {
	tool, err := object.GetTool(id)
	if err != nil {
		c.ResponseError(err.Error())
		return nil
	}
	if tool == nil {
		c.ResponseError(c.T("resource:The tool does not exist"))
		return nil
	}
	return &ResolvedResource{Type: ResourceTypeTool, Id: id, Owner: owner, Name: name, Object: tool}
}

func (c *ApiController) resolveResource(id, owner, name string) *ResolvedResource {
	resource, err := object.GetResource(id)
	if err != nil {
		c.ResponseError(err.Error())
		return nil
	}
	if resource == nil {
		c.ResponseError(c.T("resource:The resource does not exist"))
		return nil
	}
	return &ResolvedResource{Type: ResourceTypeResource, Id: id, Owner: owner, Name: name, Object: resource}
}

func (c *ApiController) CheckAccess(rr *ResolvedResource, level AccessLevel) bool {
	if c.IsGlobalAdmin() {
		return true
	}

	switch rr.Type {
	case ResourceTypeTask:
		return c.checkTaskAccess(rr, level)
	case ResourceTypeStore:
		return c.checkStoreAccess(rr, level)
	case ResourceTypeServer, ResourceTypeSkill, ResourceTypeTool:
		return c.checkAdminOnlyAccess(rr, level)
	case ResourceTypeResource:
		return c.checkResourceAccess(rr, level)
	default:
		c.ResponseError(c.T("auth:Unauthorized operation"))
		return false
	}
}

func (c *ApiController) checkTaskAccess(rr *ResolvedResource, _ AccessLevel) bool {
	username := c.GetSessionUsername()
	task := rr.Task()
	if task.Owner != username {
		c.ResponseError(c.T("auth:Unauthorized operation"))
		return false
	}
	return true
}

func (c *ApiController) checkStoreAccess(rr *ResolvedResource, level AccessLevel) bool {
	if c.IsAdmin() {
		if c.IsStoreAdmin() {
			store := rr.Store()
			username := c.GetSessionUsername()
			if store.Owner == username {
				return true
			}
		}
		if level == AccessRead {
			return true
		}
	}
	c.ResponseError(c.T("auth:Unauthorized operation"))
	return false
}

func (c *ApiController) checkAdminOnlyAccess(_ *ResolvedResource, _ AccessLevel) bool {
	if !c.IsAdmin() {
		c.ResponseError(c.T("auth:this operation requires admin privilege"))
		return false
	}
	return true
}

func (c *ApiController) checkResourceAccess(rr *ResolvedResource, _ AccessLevel) bool {
	username := c.GetSessionUsername()
	resource := rr.Resource()
	if resource.User != username {
		c.ResponseError(c.T("auth:Unauthorized operation"))
		return false
	}
	return true
}

func (c *ApiController) RequireResource(resType ResourceType, id string, level AccessLevel) *ResolvedResource {
	rr := c.ResolveResource(resType, id)
	if rr == nil {
		return nil
	}
	if !c.CheckAccess(rr, level) {
		return nil
	}
	return rr
}

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

package controllers

import (
	"encoding/json"

	"github.com/beego/beego/logs"
	"github.com/beego/beego/utils/pagination"
	"github.com/the-open-agent/openagent/object"
	"github.com/the-open-agent/openagent/util"
)

// GetGlobalTasks
// @Title GetGlobalTasks
// @Tag Task API
// @Description get global tasks
// @Success 200 {array} object.Task The Response object
// @router /get-global-tasks [get]
func (c *ApiController) GetGlobalTasks() {
	owner := c.GetSessionUsername()
	if c.IsAdmin() {
		owner = ""
	}

	tasks, err := object.GetGlobalTasks(owner)
	if err != nil {
		c.ResponseErrorInternal(err, ResourceTypeTask)
		return
	}

	c.ResponseOk(object.GetMaskedTasks(tasks, true))
}

// GetTasks
// @Title GetTasks
// @Tag Task API
// @Description get tasks
// @Param owner query string true "The owner of task"
// @Success 200 {array} object.Task The Response object
// @router /get-tasks [get]
func (c *ApiController) GetTasks() {
	owner := c.Input().Get("owner")
	limit := c.Input().Get("pageSize")
	page := c.Input().Get("p")
	field := c.Input().Get("field")
	value := c.Input().Get("value")
	sortField := c.Input().Get("sortField")
	sortOrder := c.Input().Get("sortOrder")

	if c.IsAdmin() {
		owner = ""
	}

	if !c.IsAdmin() {
		username := c.GetSessionUsername()
		if username != "" {
			owner = username
		}
	}

	if limit == "" || page == "" {
		tasks, err := object.GetTasks(owner)
		if err != nil {
			c.ResponseErrorInternal(err, ResourceTypeTask)
			return
		}

		c.ResponseOk(object.GetMaskedTasks(tasks, true))
	} else {
		limit := util.ParseInt(limit)
		count, err := object.GetTaskCount(owner, field, value)
		if err != nil {
			c.ResponseErrorInternal(err, ResourceTypeTask)
			return
		}

		paginator := pagination.SetPaginator(c.Ctx, limit, count)
		tasks, err := object.GetPaginationTasks(owner, paginator.Offset(), limit, field, value, sortField, sortOrder)
		if err != nil {
			c.ResponseErrorInternal(err, ResourceTypeTask)
			return
		}
		c.ResponseOk(tasks, paginator.Nums())
	}
}

// GetTask
// @Title GetTask
// @Tag Task API
// @Description get task
// @Param id query string true "The id (owner/name) of task"
// @Success 200 {object} object.Task The Response object
// @router /get-task [get]
func (c *ApiController) GetTask() {
	id := c.Input().Get("id")

	rr := c.RequireResource(ResourceTypeTask, id, AccessRead)
	if rr == nil {
		return
	}

	c.ResponseOk(rr.Task())
}

// UpdateTask
// @Title UpdateTask
// @Tag Task API
// @Description update task
// @Param id query string true "The id (owner/name) of the task"
// @Param body body object.Task true "The details of the task"
// @Success 200 {object} controllers.Response The Response object
// @router /update-task [post]
func (c *ApiController) UpdateTask() {
	id := c.Input().Get("id")

	var task object.Task
	err := json.Unmarshal(c.Ctx.Input.RequestBody, &task)
	if err != nil {
		c.ResponseErrorBadRequest(err.Error(), ResourceTypeTask, "body")
		return
	}

	rr := c.RequireResource(ResourceTypeTask, id, AccessWrite)
	if rr == nil {
		return
	}

	success, err := object.UpdateTask(id, &task)
	if err != nil {
		c.ResponseErrorInternal(err, ResourceTypeTask)
		return
	}

	c.ResponseOk(success)
}

// AddTask
// @Title AddTask
// @Tag Task API
// @Description add task
// @Param body body object.Task true "The details of the task"
// @Success 200 {object} controllers.Response The Response object
// @router /add-task [post]
func (c *ApiController) AddTask() {
	var task object.Task
	err := json.Unmarshal(c.Ctx.Input.RequestBody, &task)
	if err != nil {
		c.ResponseErrorBadRequest(err.Error(), ResourceTypeTask, "body")
		return
	}

	success, err := object.AddTask(&task)
	if err != nil {
		c.ResponseErrorInternal(err, ResourceTypeTask)
		return
	}

	c.ResponseOk(success)
}

// DeleteTask
// @Title DeleteTask
// @Tag Task API
// @Description delete task
// @Param body body object.Task true "The details of the task"
// @Success 200 {object} controllers.Response The Response object
// @router /delete-task [post]
func (c *ApiController) DeleteTask() {
	var task object.Task
	err := json.Unmarshal(c.Ctx.Input.RequestBody, &task)
	if err != nil {
		c.ResponseErrorBadRequest(err.Error(), ResourceTypeTask, "body")
		return
	}

	rr := c.RequireResource(ResourceTypeTask, task.GetId(), AccessWrite)
	if rr == nil {
		return
	}

	success, err := object.DeleteTask(&task)
	if err != nil {
		c.ResponseErrorInternal(err, ResourceTypeTask)
		return
	}

	c.ResponseOk(success)
}

// AnalyzeTask
// @Title AnalyzeTask
// @Tag Task API
// @Description analyze task document and generate structured report
// @Param id query string true "The id (owner/name) of the task"
// @Success 200 {object} object.TaskResult The Response object
// @router /analyze-task [post]
func (c *ApiController) AnalyzeTask() {
	id := c.Input().Get("id")
	logs.Info("[analyze-task] HTTP request id=%s user=%s", id, c.GetSessionUsername())

	rr := c.RequireResource(ResourceTypeTask, id, AccessWrite)
	if rr == nil {
		return
	}
	task := rr.Task()

	result, err := object.AnalyzeTask(task, c.GetAcceptLanguage())
	if err != nil {
		logs.Error("[analyze-task] AnalyzeTask failed id=%s: %v", id, err)
		task.Result = ""
		if _, updateErr := object.UpdateTask(id, task); updateErr != nil {
			logs.Error("[analyze-task] failed to save analyze error state id=%s: %v", id, updateErr)
		}
		c.ResponseErrorInternal(err, ResourceTypeTask)
		return
	}

	logs.Info("[analyze-task] serializing result id=%s", id)
	resultBytes, err := json.Marshal(result)
	if err != nil {
		logs.Error("[analyze-task] json.Marshal failed id=%s: %v", id, err)
		c.ResponseErrorInternal(err, ResourceTypeTask)
		return
	}
	task.Result = string(resultBytes)
	task.Score = result.Score
	logs.Info("[analyze-task] saving task id=%s resultBytes=%d", id, len(resultBytes))
	_, err = object.UpdateTask(id, task)
	if err != nil {
		logs.Error("[analyze-task] UpdateTask failed id=%s: %v", id, err)
		c.ResponseErrorInternal(err, ResourceTypeTask)
		return
	}

	logs.Info("[analyze-task] HTTP OK id=%s", id)
	c.ResponseOk(result)
}

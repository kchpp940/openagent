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
	"fmt"
	"strconv"

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
		c.ResponseError(err.Error())
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

	// For non-admins, filter by their username
	if !c.IsAdmin() {
		username := c.GetSessionUsername()
		if username != "" {
			owner = username
		}
	}

	if limit == "" || page == "" {
		tasks, err := object.GetTasks(owner)
		if err != nil {
			c.ResponseError(err.Error())
			return
		}

		c.ResponseOk(object.GetMaskedTasks(tasks, true))
	} else {
		limit := util.ParseInt(limit)
		count, err := object.GetTaskCount(owner, field, value)
		if err != nil {
			c.ResponseError(err.Error())
			return
		}

		paginator := pagination.SetPaginator(c.Ctx, limit, count)
		tasks, err := object.GetPaginationTasks(owner, paginator.Offset(), limit, field, value, sortField, sortOrder)
		if err != nil {
			c.ResponseError(err.Error())
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

	task, err := object.GetTask(id)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	// Check if task exists
	if task == nil {
		c.ResponseError(c.T("general:The task does not exist"))
		return
	}

	// Check ownership for non-admins
	if !c.IsAdmin() {
		username := c.GetSessionUsername()
		if task.Owner != username {
			c.ResponseError(c.T("auth:Unauthorized operation"))
			return
		}
	}

	c.ResponseOk(task)
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
		c.ResponseError(err.Error())
		return
	}

	existingTask, err := object.GetTask(id)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}
	if existingTask == nil {
		c.ResponseError(c.T("general:The task does not exist"))
		return
	}

	// Check ownership for non-admins
	if !c.IsAdmin() {
		username := c.GetSessionUsername()
		if existingTask.Owner != username {
			c.ResponseError(c.T("auth:Unauthorized operation"))
			return
		}
	}

	success, err := object.UpdateTask(id, &task)
	if err != nil {
		c.ResponseError(err.Error())
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
		c.ResponseError(err.Error())
		return
	}

	success, err := object.AddTask(&task)
	if err != nil {
		c.ResponseError(err.Error())
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
		c.ResponseError(err.Error())
		return
	}

	// Check ownership for non-admins
	if !c.IsAdmin() {
		username := c.GetSessionUsername()
		// Fetch task from database to verify ownership
		id := task.GetId()
		existingTask, err := object.GetTask(id)
		if err != nil {
			c.ResponseError(err.Error())
			return
		}
		if existingTask == nil {
			c.ResponseError(c.T("general:The task does not exist"))
			return
		}
		if existingTask.Owner != username {
			c.ResponseError(c.T("auth:Unauthorized operation"))
			return
		}
	}

	success, err := object.DeleteTask(&task)
	if err != nil {
		c.ResponseError(err.Error())
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

	task, err := object.GetTask(id)
	if err != nil {
		logs.Error("[analyze-task] GetTask failed id=%s: %v", id, err)
		c.ResponseError(err.Error())
		return
	}
	if task == nil {
		c.ResponseError(c.T("general:The task does not exist"))
		return
	}

	if !c.IsAdmin() {
		username := c.GetSessionUsername()
		if task.Owner != username {
			logs.Warn("[analyze-task] forbidden id=%s taskOwner=%s user=%s", id, task.Owner, username)
			c.ResponseError(c.T("auth:Unauthorized operation"))
			return
		}
	}

	result, err := object.AnalyzeTask(task, c.GetAcceptLanguage())
	if err != nil {
		logs.Error("[analyze-task] AnalyzeTask failed id=%s: %v", id, err)
		task.Result = ""
		if _, updateErr := object.UpdateTask(id, task); updateErr != nil {
			logs.Error("[analyze-task] failed to save analyze error state id=%s: %v", id, updateErr)
		}
		c.ResponseError(err.Error())
		return
	}

	logs.Info("[analyze-task] serializing result id=%s", id)
	resultBytes, err := json.Marshal(result)
	if err != nil {
		logs.Error("[analyze-task] json.Marshal failed id=%s: %v", id, err)
		c.ResponseError(err.Error())
		return
	}
	task.Result = string(resultBytes)
	task.Score = result.Score
	logs.Info("[analyze-task] saving task id=%s resultBytes=%d", id, len(resultBytes))
	_, err = object.UpdateTask(id, task)
	if err != nil {
		logs.Error("[analyze-task] UpdateTask failed id=%s: %v", id, err)
		c.ResponseError(err.Error())
		return
	}

	logs.Info("[analyze-task] HTTP OK id=%s", id)
	c.ResponseOk(result)
}

func (c *ApiController) checkTaskOwnership(taskOwner string, taskName string) (*object.Task, error) {
	task, err := object.GetTask(fmt.Sprintf("%s/%s", taskOwner, taskName))
	if err != nil {
		return nil, err
	}
	if task == nil {
		return nil, fmt.Errorf(c.T("general:The task does not exist"))
	}
	if !c.IsAdmin() {
		username := c.GetSessionUsername()
		if task.Owner != username {
			return nil, fmt.Errorf(c.T("auth:Unauthorized operation"))
		}
	}
	return task, nil
}

// AddReportComment
// @Title AddReportComment
// @Tag Task API
// @Description add a report comment for a task analysis item
// @Param body body object.ReportComment true "The report comment details"
// @Success 200 {object} controllers.Response The Response object
// @router /add-report-comment [post]
func (c *ApiController) AddReportComment() {
	var comment object.ReportComment
	err := json.Unmarshal(c.Ctx.Input.RequestBody, &comment)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	task, err := c.checkTaskOwnership(comment.TaskOwner, comment.TaskName)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	username := c.GetSessionUsername()
	comment.Owner = task.Owner
	comment.Author = username

	affected, err := object.AddReportComment(&comment)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	c.ResponseOk(affected)
}

// GetReportComments
// @Title GetReportComments
// @Tag Task API
// @Description get report comments by task id
// @Param id query string true "The task id (owner/name)"
// @Success 200 {array} object.ReportComment The Response object
// @router /get-report-comments [get]
func (c *ApiController) GetReportComments() {
	id := c.Input().Get("id")
	owner, name, err := util.GetOwnerAndNameFromIdWithError(id)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	_, err = c.checkTaskOwnership(owner, name)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	comments, err := object.GetReportCommentsByTask(owner, name)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	c.ResponseOk(comments)
}

// GetReportCommentCount
// @Title GetReportCommentCount
// @Tag Task API
// @Description get report comment counts by task id
// @Param id query string true "The task id (owner/name)"
// @Success 200 {object} map[string]int64 The Response object
// @router /get-report-comment-count [get]
func (c *ApiController) GetReportCommentCount() {
	id := c.Input().Get("id")
	owner, name, err := util.GetOwnerAndNameFromIdWithError(id)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	_, err = c.checkTaskOwnership(owner, name)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	counts, err := object.GetReportCommentCountByTask(owner, name)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	c.ResponseOk(counts)
}

// UpdateReportComment
// @Title UpdateReportComment
// @Tag Task API
// @Description update a report comment
// @Param id query int true "The comment id"
// @Param body body object.ReportComment true "The report comment details"
// @Success 200 {object} controllers.Response The Response object
// @router /update-report-comment [post]
func (c *ApiController) UpdateReportComment() {
	idStr := c.Input().Get("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.ResponseError("invalid comment id")
		return
	}

	var comment object.ReportComment
	err = json.Unmarshal(c.Ctx.Input.RequestBody, &comment)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	existing, err := object.GetReportComment(id)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}
	if existing == nil {
		c.ResponseError(c.T("general:The report comment does not exist"))
		return
	}

	_, err = c.checkTaskOwnership(existing.TaskOwner, existing.TaskName)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	username := c.GetSessionUsername()
	if !c.IsAdmin() && existing.Author != username {
		c.ResponseError(c.T("auth:Unauthorized operation"))
		return
	}

	comment.Id = id
	success, err := object.UpdateReportComment(id, &comment)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	c.ResponseOk(success)
}

// ResolveReportComment
// @Title ResolveReportComment
// @Tag Task API
// @Description resolve a report comment
// @Param id query int true "The comment id"
// @Success 200 {object} controllers.Response The Response object
// @router /resolve-report-comment [post]
func (c *ApiController) ResolveReportComment() {
	idStr := c.Input().Get("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.ResponseError("invalid comment id")
		return
	}

	existing, err := object.GetReportComment(id)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}
	if existing == nil {
		c.ResponseError(c.T("general:The report comment does not exist"))
		return
	}

	_, err = c.checkTaskOwnership(existing.TaskOwner, existing.TaskName)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	username := c.GetSessionUsername()
	success, err := object.ResolveReportComment(id, username)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	c.ResponseOk(success)
}

// ReopenReportComment
// @Title ReopenReportComment
// @Tag Task API
// @Description reopen a resolved report comment
// @Param id query int true "The comment id"
// @Success 200 {object} controllers.Response The Response object
// @router /reopen-report-comment [post]
func (c *ApiController) ReopenReportComment() {
	idStr := c.Input().Get("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.ResponseError("invalid comment id")
		return
	}

	existing, err := object.GetReportComment(id)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}
	if existing == nil {
		c.ResponseError(c.T("general:The report comment does not exist"))
		return
	}

	_, err = c.checkTaskOwnership(existing.TaskOwner, existing.TaskName)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	username := c.GetSessionUsername()
	success, err := object.ReopenReportComment(id, username)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	c.ResponseOk(success)
}

// DisputeReportComment
// @Title DisputeReportComment
// @Tag Task API
// @Description mark a report comment as disputed
// @Param id query int true "The comment id"
// @Success 200 {object} controllers.Response The Response object
// @router /dispute-report-comment [post]
func (c *ApiController) DisputeReportComment() {
	idStr := c.Input().Get("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.ResponseError("invalid comment id")
		return
	}

	existing, err := object.GetReportComment(id)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}
	if existing == nil {
		c.ResponseError(c.T("general:The report comment does not exist"))
		return
	}

	_, err = c.checkTaskOwnership(existing.TaskOwner, existing.TaskName)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	username := c.GetSessionUsername()
	success, err := object.SetReportCommentDisputed(id, username)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	c.ResponseOk(success)
}

// DeleteReportComment
// @Title DeleteReportComment
// @Tag Task API
// @Description delete a report comment
// @Param id query int true "The comment id"
// @Success 200 {object} controllers.Response The Response object
// @router /delete-report-comment [post]
func (c *ApiController) DeleteReportComment() {
	idStr := c.Input().Get("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.ResponseError("invalid comment id")
		return
	}

	existing, err := object.GetReportComment(id)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}
	if existing == nil {
		c.ResponseError(c.T("general:The report comment does not exist"))
		return
	}

	_, err = c.checkTaskOwnership(existing.TaskOwner, existing.TaskName)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	username := c.GetSessionUsername()
	if !c.IsAdmin() && existing.Author != username {
		c.ResponseError(c.T("auth:Unauthorized operation"))
		return
	}

	success, err := object.DeleteReportComment(id)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	c.ResponseOk(success)
}

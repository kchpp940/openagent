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
	"net/url"
	"regexp"
	"strings"

	"github.com/sashabaranov/go-openai"
	"github.com/the-open-agent/openagent/util"
	"xorm.io/xorm"
)

type ScopeType string

const (
	ScopeAll     ScopeType = "all"
	ScopePublic  ScopeType = "public"
	ScopePrivate ScopeType = "private"
	ScopeMine    ScopeType = "mine"
)

const (
	OrderAscend  = "ascend"
	OrderDescend = "descend"
)

type SortItem struct {
	Field string
	Order string
}

type ListQueryOptions struct {
	Owner        string
	Owners       []string
	User         string
	Name         string
	Names        []string
	Offset       int
	Limit        int
	SortField    string
	SortOrder    string
	SortFields   []SortItem
	Field        string
	Value        string
	Keyword      string
	SearchFields []string
	Scope        ScopeType
	State        string
	PublishState string
	Cols         []string
}

type PaginationResult[T any] struct {
	Data  []*T  `json:"data"`
	Total int64 `json:"total"`
}

var safeFieldNameRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func isSafeDbFieldName(name string) bool {
	return safeFieldNameRe.MatchString(name)
}

func safeSnakeField(field string) (string, bool) {
	if !util.FilterField(field) {
		return "", false
	}
	snake := util.SnakeString(field)
	if !isSafeDbFieldName(snake) {
		return "", false
	}
	return snake, true
}

func normalizeSortOrder(order string) string {
	if order == OrderAscend {
		return OrderAscend
	}
	return OrderDescend
}

func (o *ListQueryOptions) normalize() {
	if o.Offset < 0 {
		o.Offset = -1
	}
	if o.Limit < 0 {
		o.Limit = -1
	}
	if len(o.SortFields) == 0 && o.SortField == "" {
		o.SortField = "created_time"
	}
}

func BuildListSession(opts ListQueryOptions) *xorm.Session {
	opts.normalize()
	session := adapter.engine.NewSession()

	if opts.Offset != -1 && opts.Limit != -1 {
		session.Limit(opts.Limit, opts.Offset)
	}

	if len(opts.Cols) > 0 {
		safeCols := make([]string, 0, len(opts.Cols))
		for _, c := range opts.Cols {
			if snake, ok := safeSnakeField(c); ok {
				safeCols = append(safeCols, snake)
			}
		}
		if len(safeCols) > 0 {
			session.Cols(safeCols...)
		}
	}

	if opts.Owner != "" {
		session = session.And("owner = ?", opts.Owner)
	}
	if len(opts.Owners) > 0 {
		args := make([]interface{}, len(opts.Owners))
		for i, o := range opts.Owners {
			args[i] = o
		}
		session = session.In("owner", args...)
	}

	if opts.User != "" {
		session = session.And("user = ?", opts.User)
	}

	if opts.Name != "" {
		session = session.And("name = ?", opts.Name)
	}
	if len(opts.Names) > 0 {
		args := make([]interface{}, len(opts.Names))
		for i, n := range opts.Names {
			args[i] = n
		}
		session = session.In("name", args...)
	}

	if opts.Field != "" && opts.Value != "" {
		if snake, ok := safeSnakeField(opts.Field); ok {
			session = session.And(fmt.Sprintf("%s like ?", snake), fmt.Sprintf("%%%s%%", opts.Value))
		}
	}

	if opts.Keyword != "" && len(opts.SearchFields) > 0 {
		var conditions []string
		var args []interface{}
		for _, f := range opts.SearchFields {
			if snake, ok := safeSnakeField(f); ok {
				conditions = append(conditions, fmt.Sprintf("%s like ?", snake))
				args = append(args, fmt.Sprintf("%%%s%%", opts.Keyword))
			}
		}
		if len(conditions) > 0 {
			session = session.And("("+strings.Join(conditions, " OR ")+")", args...)
		}
	}

	if opts.State != "" {
		session = session.And("state = ?", opts.State)
	}

	if opts.PublishState != "" {
		session = session.And("publish_state = ?", opts.PublishState)
	}

	if len(opts.SortFields) > 0 {
		for _, si := range opts.SortFields {
			if snake, ok := safeSnakeField(si.Field); ok {
				order := normalizeSortOrder(si.Order)
				if order == OrderAscend {
					session = session.Asc(snake)
				} else {
					session = session.Desc(snake)
				}
			}
		}
	} else {
		if snake, ok := safeSnakeField(opts.SortField); ok {
			order := normalizeSortOrder(opts.SortOrder)
			if order == OrderAscend {
				session = session.Asc(snake)
			} else {
				session = session.Desc(snake)
			}
		} else {
			session = session.Desc("created_time")
		}
	}

	return session
}

func BuildCountSession(opts ListQueryOptions) *xorm.Session {
	opts.Offset = -1
	opts.Limit = -1
	opts.Cols = nil
	opts.SortFields = nil
	opts.SortField = ""
	opts.SortOrder = ""
	return BuildListSession(opts)
}

func ListQueryOptionsFromLegacy(owner string, offset, limit int, field, value, sortField, sortOrder string) ListQueryOptions {
	return ListQueryOptions{
		Owner:     owner,
		Offset:    offset,
		Limit:     limit,
		SortField: sortField,
		SortOrder: sortOrder,
		Field:     field,
		Value:     value,
	}
}

func GetDbSession(owner string, offset, limit int, field, value, sortField, sortOrder string) *xorm.Session {
	return BuildListSession(ListQueryOptionsFromLegacy(owner, offset, limit, field, value, sortField, sortOrder))
}

func getUrlFromPath(path string, origin string) (string, error) {
	if strings.HasPrefix(path, "http") {
		return path, nil
	}

	res := strings.Replace(path, ":", "|", 1)
	res = fmt.Sprintf("storage/%s", res)
	res, err := url.JoinPath(origin, res)
	return res, err
}

func isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	retryableErrors := []string{
		string(openai.RunErrorRateLimitExceeded),
	}

	for _, retryableErr := range retryableErrors {
		if strings.Contains(err.Error(), retryableErr) {
			return true
		}
	}
	return false
}

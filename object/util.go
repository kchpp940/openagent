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
	"strings"

	"github.com/sashabaranov/go-openai"
	"github.com/the-open-agent/openagent/util"
	"xorm.io/xorm"
)

type ScopeType string

const (
	ScopeAll      ScopeType = "all"
	ScopePublic   ScopeType = "public"
	ScopePrivate  ScopeType = "private"
	ScopeMine     ScopeType = "mine"
)

type ListQueryOptions struct {
	Owner      string
	Owners     []string
	Name       string
	Offset     int
	Limit      int
	SortField  string
	SortOrder  string
	Field      string
	Value      string
	Keyword    string
	SearchFields []string
	Scope      ScopeType
	State      string
}

type PaginationResult[T any] struct {
	Data  []*T  `json:"data"`
	Total int64 `json:"total"`
}

func (o *ListQueryOptions) normalize() {
	if o.Offset < 0 {
		o.Offset = -1
	}
	if o.Limit < 0 {
		o.Limit = -1
	}
	if o.SortField == "" {
		o.SortField = "created_time"
	}
}

func BuildListSession(opts ListQueryOptions) *xorm.Session {
	opts.normalize()
	session := adapter.engine.NewSession()

	if opts.Offset != -1 && opts.Limit != -1 {
		session.Limit(opts.Limit, opts.Offset)
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

	if opts.Name != "" {
		session = session.And("name = ?", opts.Name)
	}

	if opts.Field != "" && opts.Value != "" {
		if util.FilterField(opts.Field) {
			session = session.And(fmt.Sprintf("%s like ?", util.SnakeString(opts.Field)), fmt.Sprintf("%%%s%%", opts.Value))
		}
	}

	if opts.Keyword != "" && len(opts.SearchFields) > 0 {
		var conditions []string
		var args []interface{}
		for _, f := range opts.SearchFields {
			if util.FilterField(f) {
				conditions = append(conditions, fmt.Sprintf("%s like ?", util.SnakeString(f)))
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

	snakeSortField := util.SnakeString(opts.SortField)
	if opts.SortOrder == "ascend" {
		session = session.Asc(snakeSortField)
	} else {
		session = session.Desc(snakeSortField)
	}

	return session
}

func BuildCountSession(opts ListQueryOptions) *xorm.Session {
	opts.Offset = -1
	opts.Limit = -1
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

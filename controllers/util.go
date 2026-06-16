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
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"regexp"
	"strings"

	"github.com/beego/beego/context"
	"github.com/the-open-agent/openagent/auth"
	"github.com/the-open-agent/openagent/conf"
	"github.com/the-open-agent/openagent/i18n"
	"github.com/the-open-agent/openagent/util"
)

type Response struct {
	Status string      `json:"status"`
	Code   int         `json:"code,omitempty"`
	Msg    string      `json:"msg"`
	Data   interface{} `json:"data"`
	Data2  interface{} `json:"data2"`
}

type ErrorDetail struct {
	Code           int          `json:"code"`
	Key            string       `json:"key"`
	ResourceType   ResourceType `json:"resourceType,omitempty"`
	ResourceId     string       `json:"resourceId,omitempty"`
	ResourceOwner  string       `json:"resourceOwner,omitempty"`
	ResourceName   string       `json:"resourceName,omitempty"`
	RequiredAccess string       `json:"requiredAccess,omitempty"`
	CurrentUser    string       `json:"currentUser,omitempty"`
	Field          string       `json:"field,omitempty"`
}

const (
	ErrCodeOK                    = 0
	ErrCodeBadRequest            = 40000
	ErrCodeInvalidResourceFormat = 40001
	ErrCodeResourceIdRequired    = 40002
	ErrCodeValidationFailed      = 40003
	ErrCodeJsonParseFailed       = 40004

	ErrCodeUnauthorized = 40100
	ErrCodeNotSignedIn  = 40101

	ErrCodeForbidden             = 40300
	ErrCodeAdminRequired         = 40301
	ErrCodeNotResourceOwner      = 40302
	ErrCodeStorePermissionDenied = 40303

	ErrCodeNotFound         = 40400
	ErrCodeTaskNotFound     = 40401
	ErrCodeStoreNotFound    = 40402
	ErrCodeServerNotFound   = 40403
	ErrCodeSkillNotFound    = 40404
	ErrCodeToolNotFound     = 40405
	ErrCodeResourceNotFound = 40406
	ErrCodeCommentNotFound  = 40407

	ErrCodeInternal = 50000
)

func accessLevelString(level AccessLevel) string {
	if level == AccessWrite {
		return "write"
	}
	return "read"
}

func (c *ApiController) ResponseOk(data ...interface{}) {
	resp := Response{Status: "ok", Code: ErrCodeOK}
	switch len(data) {
	case 2:
		resp.Data2 = data[1]
		fallthrough
	case 1:
		resp.Data = data[0]
	}
	c.Data["json"] = resp
	c.ServeJSON()
}

func (c *ApiController) ResponseError(error string, data ...interface{}) {
	c.ResponseErrorWithCode(ErrCodeBadRequest, error, data...)
}

func (c *ApiController) ResponseErrorInternal(err error, resType ...ResourceType) {
	detail := &ErrorDetail{Code: ErrCodeInternal, Key: err.Error()}
	if len(resType) > 0 {
		detail.ResourceType = resType[0]
	}
	c.ResponseErrorWithCode(ErrCodeInternal, err.Error(), detail)
}

func (c *ApiController) ResponseErrorValidation(msg string, field string) {
	detail := &ErrorDetail{Code: ErrCodeValidationFailed, Key: msg, Field: field}
	c.ResponseErrorWithCode(ErrCodeValidationFailed, msg, detail)
}

func (c *ApiController) ResponseErrorJsonParse(err error, field string, resType ...ResourceType) {
	msg := fmt.Sprintf("Invalid JSON in request body: %s", err.Error())
	detail := &ErrorDetail{Code: ErrCodeJsonParseFailed, Key: msg, Field: field}
	if len(resType) > 0 {
		detail.ResourceType = resType[0]
	}
	c.ResponseErrorWithCode(ErrCodeJsonParseFailed, msg, detail)
}

func (c *ApiController) ResponseErrorWithCode(code int, msg string, data ...interface{}) {
	detail := &ErrorDetail{Code: code, Key: msg}
	if len(data) > 0 {
		if ed, ok := data[0].(*ErrorDetail); ok {
			detail = ed
		}
	}
	resp := Response{Status: "error", Code: code, Msg: msg}
	switch len(data) {
	case 2:
		resp.Data2 = data[1]
		fallthrough
	case 1:
		resp.Data = data[0]
	default:
		resp.Data = detail
	}
	c.Data["json"] = resp
	c.ServeJSON()
}

func (c *ApiController) T(error string) string {
	return i18n.Translate(c.GetAcceptLanguage(), error)
}

func (c *ApiController) ResponseAudio(audioData []byte, contentType string, filename string) {
	if contentType == "" {
		contentType = "audio/mp3"
	}
	if filename == "" {
		filename = "audio.mp3"
	}

	c.Ctx.Output.Header("Content-Type", contentType)
	c.Ctx.Output.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	err := c.Ctx.Output.Body(audioData)
	if err != nil {
		responseError(c.Ctx, err.Error())
	}
}

func (c *ApiController) GetAcceptLanguage() string {
	language := c.Ctx.Request.Header.Get("Accept-Language")
	if len(language) > 2 {
		language = language[0:2]
	}
	return conf.GetLanguage(language)
}

func (c *ApiController) RequireSignedIn() (string, bool) {
	userId := c.GetSessionUsername()
	if userId == "" {
		c.ResponseErrorWithCode(ErrCodeNotSignedIn, c.T("auth:Please sign in first"))
		return "", false
	}
	return userId, true
}

func (c *ApiController) RequireSignedInUser() (*auth.User, bool) {
	user := c.GetSessionUser()
	if user == nil {
		c.ResponseErrorWithCode(ErrCodeNotSignedIn, c.T("auth:Please sign in first"))
		return nil, false
	}
	return user, true
}

func (c *ApiController) CheckSignedIn() (string, bool) {
	userId := c.GetSessionUsername()
	if userId == "" {
		return "", false
	}
	return userId, true
}

func (c *ApiController) RequireAdmin() bool {
	if !c.IsAdmin() {
		c.ResponseErrorWithCode(ErrCodeAdminRequired, c.T("auth:this operation requires admin privilege"))
		return false
	}

	return true
}

func (c *ApiController) IsAdmin() bool {
	user := c.GetSessionUser()
	return util.IsAdmin(user)
}

func (c *ApiController) IsGlobalAdmin() bool {
	user := c.GetSessionUser()
	return util.IsGlobalAdmin(user)
}

func (c *ApiController) IsStoreAdmin() bool {
	user := c.GetSessionUser()
	return util.IsStoreAdmin(user)
}

func DenyRequest(ctx *context.Context) {
	responseErrorWithCode(ctx, ErrCodeForbidden, "auth:Unauthorized operation")
}

func responseError(ctx *context.Context, error string, data ...interface{}) {
	responseErrorWithCode(ctx, ErrCodeBadRequest, error, data...)
}

func responseErrorWithCode(ctx *context.Context, code int, error string, data ...interface{}) {
	language := ctx.Request.Header.Get("Accept-Language")
	if len(language) > 2 {
		language = language[0:2]
	}
	language = conf.GetLanguage(language)

	translatedError := error
	if strings.Contains(error, ":") {
		translatedError = i18n.Translate(language, error)
	}

	detail := &ErrorDetail{Code: code, Key: error}
	if len(data) > 0 {
		if ed, ok := data[0].(*ErrorDetail); ok {
			detail = ed
		}
	}

	resp := Response{Status: "error", Code: code, Msg: translatedError}
	switch len(data) {
	case 2:
		resp.Data2 = data[1]
		fallthrough
	case 1:
		resp.Data = data[0]
	default:
		resp.Data = detail
	}

	err := ctx.Output.JSON(resp, true, false)
	if err != nil {
		panic(err)
	}
}

func isIpAddress(host string) bool {
	// Attempt to split the host and port, ignoring the error
	hostWithoutPort, _, err := net.SplitHostPort(host)
	if err != nil {
		// If an error occurs, it might be because there's no port
		// In that case, use the original host string
		hostWithoutPort = host
	}

	// Attempt to parse the host as an IP address (both IPv4 and IPv6)
	ip := net.ParseIP(hostWithoutPort)
	// if host is not nil is an IP address else is not an IP address
	return ip != nil
}

func getOriginFromHost(host string) string {
	protocol := "https://"
	if !strings.Contains(host, ".") {
		// "localhost:14000"
		protocol = "http://"
	} else if isIpAddress(host) {
		// "192.168.0.10"
		protocol = "http://"
	}

	return fmt.Sprintf("%s%s", protocol, host)
}

func removeHtmlTags(s string) string {
	re := regexp.MustCompile(`<[^>]+>`)
	return re.ReplaceAllString(s, "")
}

func getContentHash(content string) string {
	hasher := sha256.New()
	hasher.Write([]byte(content))

	res := hex.EncodeToString(hasher.Sum(nil))
	res = res[:8]
	return res
}

func (c *ApiController) getClientIp() string {
	res := strings.Replace(util.GetIPFromRequest(c.Ctx.Request), ": ", "", -1)
	return res
}

func (c *ApiController) getUserAgent() string {
	res := c.Ctx.Request.UserAgent()
	return res
}

func (c *ApiController) IsCurrentUser(usernameInput string) bool {
	username := c.GetSessionUsername()
	if !c.IsAdmin() && username != usernameInput {
		c.ResponseErrorWithCode(ErrCodeForbidden, c.T("auth:Unauthorized operation"))
		return false
	}
	return true
}

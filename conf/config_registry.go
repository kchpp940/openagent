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

package conf

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/beego/beego"
)

type ConfigType string

const (
	TypeString   ConfigType = "string"
	TypePassword ConfigType = "password"
	TypeBool     ConfigType = "bool"
	TypeInt      ConfigType = "int"
	TypeJSON     ConfigType = "json"
	TypeURL      ConfigType = "url"
	TypeColor    ConfigType = "color"
	TypeTextarea ConfigType = "textarea"
	TypeSelect   ConfigType = "select"
)

type ConfigCategory string

const (
	CategoryGeneral    ConfigCategory = "General"
	CategoryBranding   ConfigCategory = "Branding"
	CategoryContent    ConfigCategory = "Content"
	CategoryAuth       ConfigCategory = "Auth"
	CategoryDatabase   ConfigCategory = "Database"
	CategoryAdvanced   ConfigCategory = "Advanced"
	CategoryHidden     ConfigCategory = "Hidden"
)

type ConfigOption struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

type ConfigItem struct {
	Key             string         `json:"key"`
	DefaultValue    string         `json:"defaultValue"`
	Type            ConfigType     `json:"type"`
	Required        bool           `json:"required"`
	Sensitive       bool           `json:"sensitive"`
	Category        ConfigCategory `json:"category"`
	Order           int            `json:"order"`
	LabelEn         string         `json:"labelEn"`
	LabelZh         string         `json:"labelZh"`
	DescriptionEn   string         `json:"descriptionEn"`
	DescriptionZh   string         `json:"descriptionZh"`
	PlaceholderEn   string         `json:"placeholderEn"`
	PlaceholderZh   string         `json:"placeholderZh"`
	Options         []ConfigOption `json:"options,omitempty"`
	SiteField       string         `json:"-"`
	DeprecatedAlias []string       `json:"-"`
}

type ConfigMetadata struct {
	Key             string         `json:"key"`
	Type            ConfigType     `json:"type"`
	Required        bool           `json:"required"`
	Sensitive       bool           `json:"sensitive"`
	Category        ConfigCategory `json:"category"`
	Order           int            `json:"order"`
	LabelEn         string         `json:"labelEn"`
	LabelZh         string         `json:"labelZh"`
	DescriptionEn   string         `json:"descriptionEn"`
	DescriptionZh   string         `json:"descriptionZh"`
	PlaceholderEn   string         `json:"placeholderEn"`
	PlaceholderZh   string         `json:"placeholderZh"`
	Options         []ConfigOption `json:"options,omitempty"`
	CurrentValue    string         `json:"currentValue,omitempty"`
	HasSiteOverride bool           `json:"hasSiteOverride"`
}

var (
	registryMu  sync.RWMutex
	registry    = map[string]*ConfigItem{}
	orderedKeys []string
)

func RegisterConfig(item ConfigItem) {
	registryMu.Lock()
	defer registryMu.Unlock()

	if _, exists := registry[item.Key]; exists {
		panic(fmt.Sprintf("conf: duplicate config key registered: %s", item.Key))
	}

	registry[item.Key] = &item
	orderedKeys = append(orderedKeys, item.Key)
}

func init() {
	registerAllConfigs()
}

func registerAllConfigs() {
	RegisterConfig(ConfigItem{
		Key:          "appname",
		DefaultValue: "openagent",
		Type:         TypeString,
		Category:     CategoryHidden,
		Order:        1,
		LabelEn:      "App Name",
		LabelZh:      "应用名称",
		SiteField:    "",
	})

	RegisterConfig(ConfigItem{
		Key:          "httpport",
		DefaultValue: "14000",
		Type:         TypeInt,
		Category:     CategoryHidden,
		Order:        2,
		LabelEn:      "HTTP Port",
		LabelZh:      "HTTP 端口",
		SiteField:    "",
	})

	RegisterConfig(ConfigItem{
		Key:          "runmode",
		DefaultValue: "dev",
		Type:         TypeSelect,
		Category:     CategoryHidden,
		Order:        3,
		LabelEn:      "Run Mode",
		LabelZh:      "运行模式",
		Options: []ConfigOption{
			{Value: "dev", Label: "Development"},
			{Value: "prod", Label: "Production"},
		},
		SiteField: "",
	})

	RegisterConfig(ConfigItem{
		Key:          "driverName",
		DefaultValue: "mysql",
		Type:         TypeSelect,
		Category:     CategoryDatabase,
		Order:        10,
		LabelEn:      "Database Driver",
		LabelZh:      "数据库驱动",
		DescriptionEn: "Database driver: mysql, sqlite3, postgres, mssql",
		DescriptionZh: "数据库驱动：mysql、sqlite3、postgres、mssql",
		Options: []ConfigOption{
			{Value: "mysql", Label: "MySQL"},
			{Value: "sqlite3", Label: "SQLite"},
			{Value: "postgres", Label: "PostgreSQL"},
			{Value: "mssql", Label: "SQL Server"},
		},
		SiteField: "",
	})

	RegisterConfig(ConfigItem{
		Key:          "dataSourceName",
		DefaultValue: "root:123456@tcp(localhost:3306)/",
		Type:         TypeString,
		Category:     CategoryDatabase,
		Order:        11,
		LabelEn:      "Data Source Name",
		LabelZh:      "数据源名称",
		DescriptionEn: "Database connection string, format depends on driver",
		DescriptionZh: "数据库连接字符串，格式取决于驱动类型",
		SiteField:    "",
	})

	RegisterConfig(ConfigItem{
		Key:          "dbName",
		DefaultValue: "casibase",
		Type:         TypeString,
		Category:     CategoryDatabase,
		Order:        12,
		LabelEn:      "Database Name",
		LabelZh:      "数据库名称",
		SiteField:    "",
	})

	RegisterConfig(ConfigItem{
		Key:          "redisEndpoint",
		DefaultValue: "",
		Type:         TypeString,
		Category:     CategoryDatabase,
		Order:        13,
		LabelEn:      "Redis Endpoint",
		LabelZh:      "Redis 端点",
		DescriptionEn: "Optional Redis endpoint for session storage. If empty, file-based sessions are used.",
		DescriptionZh: "可选的 Redis 端点，用于会话存储。留空则使用文件存储会话。",
		SiteField:    "",
	})

	RegisterConfig(ConfigItem{
		Key:           "isDemoMode",
		DefaultValue:  "false",
		Type:          TypeBool,
		Category:      CategoryGeneral,
		Order:         100,
		LabelEn:       "Demo Mode",
		LabelZh:       "演示模式",
		DescriptionEn: "Enable demo mode with restricted operations",
		DescriptionZh: "启用演示模式，限制部分操作",
		SiteField:     "",
	})

	RegisterConfig(ConfigItem{
		Key:             "htmlTitle",
		DefaultValue:    "OpenAgent",
		Type:            TypeString,
		Category:        CategoryBranding,
		Order:           200,
		LabelEn:         "HTML Title",
		LabelZh:         "HTML 标题",
		DescriptionEn:   "Browser tab title",
		DescriptionZh:   "浏览器标签页标题",
		PlaceholderEn:   "e.g. My AI Assistant",
		PlaceholderZh:   "例如：我的 AI 助手",
		SiteField:       "HtmlTitle",
		DeprecatedAlias: []string{},
	})

	RegisterConfig(ConfigItem{
		Key:           "themeColor",
		DefaultValue:  "#404040",
		Type:          TypeColor,
		Category:      CategoryBranding,
		Order:         201,
		LabelEn:       "Theme Color",
		LabelZh:       "主题颜色",
		DescriptionEn: "Primary theme color (hex)",
		DescriptionZh: "主主题色（十六进制）",
		SiteField:     "ThemeColor",
	})

	RegisterConfig(ConfigItem{
		Key:           "faviconUrl",
		DefaultValue:  "https://cdn.openagentai.org/img/openagent.png",
		Type:          TypeURL,
		Category:      CategoryBranding,
		Order:         202,
		LabelEn:       "Favicon URL",
		LabelZh:       "Favicon 地址",
		DescriptionEn: "URL for the browser favicon (32x32, PNG/SVG)",
		DescriptionZh: "浏览器 favicon 的 URL（32x32，PNG/SVG）",
		PlaceholderEn: "https://.../favicon.png",
		PlaceholderZh: "https://.../favicon.png",
		SiteField:     "FaviconUrl",
	})

	RegisterConfig(ConfigItem{
		Key:           "logoUrl",
		DefaultValue:  "https://cdn.openagentai.org/img/openagent-logo_1900x450.png",
		Type:          TypeURL,
		Category:      CategoryBranding,
		Order:         203,
		LabelEn:       "Logo URL",
		LabelZh:       "Logo 地址",
		DescriptionEn: "URL for the header logo (transparent PNG, ~4:1)",
		DescriptionZh: "顶部 Logo 的 URL（透明 PNG，比例 ~4:1）",
		PlaceholderEn: "https://.../logo.png",
		PlaceholderZh: "https://.../logo.png",
		SiteField:     "LogoUrl",
	})

	RegisterConfig(ConfigItem{
		Key:           "staticBaseUrl",
		DefaultValue:  "https://cdn.openagentai.org",
		Type:          TypeURL,
		Category:      CategoryBranding,
		Order:         204,
		LabelEn:       "Static Base URL",
		LabelZh:       "静态资源基础地址",
		DescriptionEn: "Base URL for serving static assets (CDN)",
		DescriptionZh: "静态资源（CDN）的基础地址",
		PlaceholderEn: "https://cdn.example.com",
		PlaceholderZh: "https://cdn.example.com",
		SiteField:     "StaticBaseUrl",
	})

	RegisterConfig(ConfigItem{
		Key:           "endpoint",
		DefaultValue:  "",
		Type:          TypeURL,
		Category:      CategoryBranding,
		Order:         205,
		LabelEn:       "Endpoint URL",
		LabelZh:       "端点地址",
		DescriptionEn: "Public endpoint URL of this OpenAgent instance",
		DescriptionZh: "本 OpenAgent 实例的公网访问地址",
		PlaceholderEn: "https://openagent.example.com",
		PlaceholderZh: "https://openagent.example.com",
		SiteField:     "Endpoint",
	})

	RegisterConfig(ConfigItem{
		Key:           "navbarHtml",
		DefaultValue:  "",
		Type:          TypeTextarea,
		Category:      CategoryContent,
		Order:         300,
		LabelEn:       "Navbar HTML",
		LabelZh:       "导航栏 HTML",
		DescriptionEn: "Custom HTML injected into the top navigation bar",
		DescriptionZh: "注入顶部导航栏的自定义 HTML",
		SiteField:     "NavbarHtml",
	})

	RegisterConfig(ConfigItem{
		Key:           "footerHtml",
		DefaultValue:  `<a target="_blank" href="https://github.com/the-open-agent/openagent" rel="noreferrer"><img style="padding-bottom: 3px;" height="30" alt="OpenAgent" src="https://cdn.openagentai.org/img/openagent-logo_1900x450.png" /></a>`,
		Type:          TypeTextarea,
		Category:      CategoryContent,
		Order:         301,
		LabelEn:       "Footer HTML",
		LabelZh:       "页脚 HTML",
		DescriptionEn: "Custom HTML injected into the page footer",
		DescriptionZh: "注入页面底部的自定义 HTML",
		SiteField:     "FooterHtml",
	})

	RegisterConfig(ConfigItem{
		Key:           "hubDesc",
		DefaultValue:  "",
		Type:          TypeTextarea,
		Category:      CategoryContent,
		Order:         302,
		LabelEn:       "Hub Description",
		LabelZh:       "Hub 描述",
		DescriptionEn: "Description text for the Hub page",
		DescriptionZh: "Hub 页面的描述文字",
		SiteField:     "HubDesc",
	})

	RegisterConfig(ConfigItem{
		Key:             "issuer",
		DefaultValue:    "",
		Type:            TypeURL,
		Category:        CategoryAuth,
		Order:           400,
		LabelEn:         "OIDC Issuer",
		LabelZh:         "OIDC 发行者",
		DescriptionEn:   "OIDC issuer URL (e.g. https://casdoor.example.com). Fallback: casdoorEndpoint",
		DescriptionZh:   "OIDC 发行者 URL（如 https://casdoor.example.com）。回退：casdoorEndpoint",
		PlaceholderEn:   "https://casdoor.example.com",
		PlaceholderZh:   "https://casdoor.example.com",
		SiteField:       "Issuer",
		DeprecatedAlias: []string{"casdoorEndpoint"},
	})

	RegisterConfig(ConfigItem{
		Key:           "casdoorEndpoint",
		DefaultValue:  "",
		Type:          TypeURL,
		Category:      CategoryAuth,
		Order:         401,
		LabelEn:       "Casdoor Endpoint",
		LabelZh:       "Casdoor 端点",
		DescriptionEn: "Deprecated. Use issuer instead. Casdoor server endpoint URL.",
		DescriptionZh: "已弃用，请使用 issuer。Casdoor 服务器端点地址。",
		SiteField:     "CasdoorEndpoint",
	})

	RegisterConfig(ConfigItem{
		Key:           "clientId",
		DefaultValue:  "",
		Type:          TypeString,
		Category:      CategoryAuth,
		Order:         402,
		LabelEn:       "Client ID",
		LabelZh:       "客户端 ID",
		DescriptionEn: "OIDC / Casdoor application client ID",
		DescriptionZh: "OIDC / Casdoor 应用的客户端 ID",
		SiteField:     "ClientId",
	})

	RegisterConfig(ConfigItem{
		Key:           "clientSecret",
		DefaultValue:  "",
		Type:          TypePassword,
		Category:      CategoryAuth,
		Order:         403,
		LabelEn:       "Client Secret",
		LabelZh:       "客户端密钥",
		DescriptionEn: "OIDC / Casdoor application client secret (sensitive)",
		DescriptionZh: "OIDC / Casdoor 应用的客户端密钥（敏感字段）",
		Sensitive:     true,
		SiteField:     "ClientSecret",
	})

	RegisterConfig(ConfigItem{
		Key:           "casdoorOrganization",
		DefaultValue:  "",
		Type:          TypeString,
		Category:      CategoryAuth,
		Order:         404,
		LabelEn:       "Casdoor Organization",
		LabelZh:       "Casdoor 组织",
		DescriptionEn: "Casdoor organization name (backward compat)",
		DescriptionZh: "Casdoor 组织名称（向后兼容）",
		SiteField:     "CasdoorOrganization",
	})

	RegisterConfig(ConfigItem{
		Key:           "casdoorApplication",
		DefaultValue:  "",
		Type:          TypeString,
		Category:      CategoryAuth,
		Order:         405,
		LabelEn:       "Casdoor Application",
		LabelZh:       "Casdoor 应用",
		DescriptionEn: "Casdoor application name (backward compat)",
		DescriptionZh: "Casdoor 应用名称（向后兼容）",
		SiteField:     "CasdoorApplication",
	})

	RegisterConfig(ConfigItem{
		Key:           "checkUserBalance",
		DefaultValue:  "false",
		Type:          TypeBool,
		Category:      CategoryAuth,
		Order:         406,
		LabelEn:       "Check User Balance",
		LabelZh:       "检查用户余额",
		DescriptionEn: "Enable user balance checks before operations",
		DescriptionZh: "操作前检查用户余额",
		SiteField:     "CheckUserBalance",
	})

	RegisterConfig(ConfigItem{
		Key:           "ipParsingMode",
		DefaultValue:  "",
		Type:          TypeString,
		Category:      CategoryAdvanced,
		Order:         500,
		LabelEn:       "IP Parsing Mode",
		LabelZh:       "IP 解析模式",
		DescriptionEn: "Method for parsing client IP from headers (e.g. x-forwarded-for, x-real-ip)",
		DescriptionZh: "从请求头解析客户端 IP 的方法（如 x-forwarded-for、x-real-ip）",
		SiteField:     "IpParsingMode",
	})

	RegisterConfig(ConfigItem{
		Key:           "parentDbName",
		DefaultValue:  "",
		Type:          TypeString,
		Category:      CategoryAdvanced,
		Order:         501,
		LabelEn:       "Parent DB Name",
		LabelZh:       "父数据库名称",
		DescriptionEn: "For multi-tenant setups: name of the parent/master database",
		DescriptionZh: "多租户场景：父/主数据库的名称",
		SiteField:     "ParentDbName",
	})

	RegisterConfig(ConfigItem{
		Key:           "hubDbNames",
		DefaultValue:  "",
		Type:          TypeString,
		Category:      CategoryAdvanced,
		Order:         502,
		LabelEn:       "Hub DB Names",
		LabelZh:       "Hub 数据库名称",
		DescriptionEn: "Comma-separated list of hub database names",
		DescriptionZh: "逗号分隔的 Hub 数据库名称列表",
		PlaceholderEn: "openagent-db1, openagent-db2",
		PlaceholderZh: "openagent-db1, openagent-db2",
		SiteField:     "HubDbNames",
	})

	RegisterConfig(ConfigItem{
		Key:           "socks5Proxy",
		DefaultValue:  "127.0.0.1:10808",
		Type:          TypeString,
		Category:      CategoryAdvanced,
		Order:         503,
		LabelEn:       "SOCKS5 Proxy",
		LabelZh:       "SOCKS5 代理",
		DescriptionEn: "SOCKS5 proxy host:port for outbound network requests (e.g. web_search)",
		DescriptionZh: "出站网络请求（如 web_search）的 SOCKS5 代理 host:port",
		PlaceholderEn: "127.0.0.1:10808",
		PlaceholderZh: "127.0.0.1:10808",
		SiteField:     "Socks5Proxy",
	})

	RegisterConfig(ConfigItem{
		Key:           "logConfig",
		DefaultValue:  `{"adapter":"file", "filename": "logs/openagent.log", "maxdays":99999, "perm":"0770"}`,
		Type:          TypeJSON,
		Category:      CategoryAdvanced,
		Order:         504,
		LabelEn:       "Log Config",
		LabelZh:       "日志配置",
		DescriptionEn: "Beego logs adapter config as JSON string",
		DescriptionZh: "Beego 日志适配器配置，JSON 字符串格式",
		SiteField:     "LogConfig",
	})

	RegisterConfig(ConfigItem{
		Key:           "defaultColorPrimary",
		DefaultValue:  "",
		Type:          TypeColor,
		Category:      CategoryBranding,
		Order:         206,
		LabelEn:       "Default Primary Color",
		LabelZh:       "默认主色",
		DescriptionEn: "Default Ant Design primary color",
		DescriptionZh: "默认 Ant Design 主色",
		SiteField:     "",
	})

	RegisterConfig(ConfigItem{
		Key:           "batchSize",
		DefaultValue:  "100",
		Type:          TypeInt,
		Category:      CategoryAdvanced,
		Order:         505,
		LabelEn:       "Batch Size",
		LabelZh:       "批处理大小",
		DescriptionEn: "Default batch size for bulk operations",
		DescriptionZh: "批量操作的默认批大小",
		SiteField:     "",
	})
}

func lookupItem(key string) *ConfigItem {
	registryMu.RLock()
	defer registryMu.RUnlock()
	item, ok := registry[key]
	if ok {
		return item
	}
	for _, it := range registry {
		for _, alias := range it.DeprecatedAlias {
			if alias == key {
				return it
			}
		}
	}
	return nil
}

func resolveValue(item *ConfigItem, key string) string {
	if value, ok := os.LookupEnv(key); ok && value != "" {
		return value
	}

	for _, alias := range item.DeprecatedAlias {
		if value, ok := os.LookupEnv(alias); ok && value != "" {
			return value
		}
	}

	siteConfigMu.RLock()
	siteVal, hasSiteVal := siteConfigOverrides[key]
	siteConfigMu.RUnlock()
	if hasSiteVal && siteVal != "" {
		return siteVal
	}
	for _, alias := range item.DeprecatedAlias {
		siteConfigMu.RLock()
		v, ok := siteConfigOverrides[alias]
		siteConfigMu.RUnlock()
		if ok && v != "" {
			return v
		}
	}

	tokens := ReadGlobalConfigTokens()
	if len(tokens) > 0 {
		switch key {
		case "htmlTitle":
			if tokens[8] != "" {
				return tokens[8]
			}
		case "faviconUrl":
			if tokens[9] != "" {
				return tokens[9]
			}
		case "logoUrl":
			if tokens[10] != "" {
				return tokens[10]
			}
		case "footerHtml":
			if tokens[11] != "" {
				return tokens[11]
			}
		}
	}

	res := beego.AppConfig.String(key)
	if res == "" {
		for _, alias := range item.DeprecatedAlias {
			res = beego.AppConfig.String(alias)
			if res != "" {
				break
			}
		}
	}
	if res != "" {
		return res
	}

	return item.DefaultValue
}

func GetConfigValue(key string) (string, bool) {
	item := lookupItem(key)
	if item == nil {
		return "", false
	}
	return resolveValue(item, key), true
}

func GetConfigString2(key string) string {
	item := lookupItem(key)
	if item == nil {
		if value, ok := os.LookupEnv(key); ok {
			return value
		}
		siteConfigMu.RLock()
		siteVal, hasSiteVal := siteConfigOverrides[key]
		siteConfigMu.RUnlock()
		if hasSiteVal {
			return siteVal
		}
		return beego.AppConfig.String(key)
	}

	result := resolveValue(item, key)
	if key == "staticBaseUrl" {
		if strings.HasSuffix(beego.AppConfig.String("casdoorEndpoint"), ".casdoor.net") && result == "https://cdn.openagentai.org" {
			return "https://cdn.casibase.com"
		}
	}
	return result
}

func GetConfigBool2(key string) bool {
	raw := GetConfigString2(key)
	return raw == "true" || raw == "True" || raw == "TRUE" || raw == "1" || raw == "yes" || raw == "on"
}

func GetConfigInt2(key string) int {
	raw := GetConfigString2(key)
	num, err := strconv.Atoi(raw)
	if err != nil {
		item := lookupItem(key)
		if item != nil {
			if defNum, defErr := strconv.Atoi(item.DefaultValue); defErr == nil {
				return defNum
			}
		}
		return 0
	}
	return num
}

func HasSiteOverride(key string) bool {
	siteConfigMu.RLock()
	defer siteConfigMu.RUnlock()
	_, ok := siteConfigOverrides[key]
	if ok {
		return true
	}
	item := lookupItem(key)
	if item != nil {
		for _, alias := range item.DeprecatedAlias {
			if _, ok2 := siteConfigOverrides[alias]; ok2 {
				return true
			}
		}
	}
	return false
}

func GetAllConfigMetadata() []ConfigMetadata {
	registryMu.RLock()
	defer registryMu.RUnlock()

	result := make([]ConfigMetadata, 0, len(orderedKeys))
	for _, key := range orderedKeys {
		item := registry[key]
		if item.Category == CategoryHidden {
			continue
		}
		meta := ConfigMetadata{
			Key:             item.Key,
			Type:            item.Type,
			Required:        item.Required,
			Sensitive:       item.Sensitive,
			Category:        item.Category,
			Order:           item.Order,
			LabelEn:         item.LabelEn,
			LabelZh:         item.LabelZh,
			DescriptionEn:   item.DescriptionEn,
			DescriptionZh:   item.DescriptionZh,
			PlaceholderEn:   item.PlaceholderEn,
			PlaceholderZh:   item.PlaceholderZh,
			Options:         item.Options,
			HasSiteOverride: HasSiteOverride(item.Key),
		}
		if !item.Sensitive {
			meta.CurrentValue = resolveValue(item, item.Key)
		}
		result = append(result, meta)
	}
	return result
}

func ValidateConfigs() []string {
	var missing []string
	registryMu.RLock()
	defer registryMu.RUnlock()
	for _, key := range orderedKeys {
		item := registry[key]
		if item.Required {
			val := resolveValue(item, key)
			if val == "" {
				missing = append(missing, key)
			}
		}
	}
	return missing
}

type SiteConfigMap map[string]string

func BuildSiteOverridesFromConfigMap(cm SiteConfigMap) map[string]string {
	registryMu.RLock()
	defer registryMu.RUnlock()

	result := make(map[string]string)
	for key, value := range cm {
		item, ok := registry[key]
		if !ok {
			continue
		}
		if item.SiteField == "" {
			continue
		}
		result[key] = value
	}
	return result
}

func GetAllSiteFieldKeys() map[string]string {
	registryMu.RLock()
	defer registryMu.RUnlock()

	result := make(map[string]string)
	for _, key := range orderedKeys {
		item := registry[key]
		if item.SiteField != "" {
			result[key] = item.SiteField
		}
	}
	return result
}

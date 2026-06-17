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

import React from "react";
import {Col, Input, Row, Select, Switch, Tooltip} from "antd";
import {InfoCircleOutlined, LinkOutlined} from "@ant-design/icons";
import i18next from "i18next";
import * as Setting from "./Setting";

const {TextArea, Password} = Input;
const {Option} = Select;

export const CATEGORY_LABELS = {
  General: {en: "General Settings", zh: "通用设置"},
  Branding: {en: "Branding", zh: "品牌定制"},
  Content: {en: "Content", zh: "内容设置"},
  Auth: {en: "Authentication", zh: "身份认证"},
  Database: {en: "Database", zh: "数据库"},
  Advanced: {en: "Advanced", zh: "高级设置"},
};

export const CATEGORY_DESCRIPTIONS = {
  General: {en: "Core application behavior", zh: "核心应用行为配置"},
  Branding: {en: "Visual identity and CDN settings", zh: "视觉标识和 CDN 设置"},
  Content: {en: "Custom HTML content injection", zh: "自定义 HTML 内容注入"},
  Auth: {en: "OIDC / Casdoor identity provider", zh: "OIDC / Casdoor 身份提供商"},
  Database: {en: "Database and storage connections", zh: "数据库和存储连接"},
  Advanced: {en: "Networking, logging, and expert options", zh: "网络、日志及专家选项"},
};

function i18nText(field, langKey) {
  if (!field) {return "";}
  const key = `${langKey === "zh" ? "zh" : "en"}`;
  return field[key] || field["en"] || field["zh"] || "";
}

function getLangKey() {
  const lng = Setting.getLanguage();
  if (lng === "zh" || lng === "zh-CN" || lng === "cn") {
    return "zh";
  }
  return "en";
}

function parseBool(v) {
  if (typeof v === "boolean") {return v;}
  if (typeof v === "string") {
    const s = v.toLowerCase().trim();
    return s === "true" || s === "1" || s === "yes" || s === "on";
  }
  return false;
}

function toBoolString(v) {
  if (typeof v === "boolean") {return v ? "true" : "false";}
  return v;
}

function FieldTooltip({meta, langKey}) {
  const desc = i18nText({en: meta.descriptionEn, zh: meta.descriptionZh}, langKey);
  if (!desc) {return null;}
  return (
    <Tooltip title={desc} placement="top">
      <InfoCircleOutlined style={{color: "var(--ant-color-text-tertiary)", marginLeft: 4, cursor: "help"}} />
    </Tooltip>
  );
}

function FieldLabel({meta, langKey}) {
  const label = i18nText({en: meta.labelEn, zh: meta.labelZh}, langKey);
  return (
    <span style={{display: "inline-flex", alignItems: "center"}}>
      <span style={{fontWeight: 500}}>{label}</span>
      {meta.required ? <span style={{color: "#ff4d4f", marginLeft: 4}}>*</span> : null}
      <FieldTooltip meta={meta} langKey={langKey} />
    </span>
  );
}

function renderControl(meta, value, onChange, langKey, extraProps = {}) {
  const placeholder = i18nText({en: meta.placeholderEn, zh: meta.placeholderZh}, langKey);

  switch (meta.type) {
  case "bool":
    return (
      <Switch
        checked={parseBool(value)}
        onChange={(checked) => onChange(meta.key, toBoolString(checked))}
        {...extraProps}
      />
    );
  case "password":
    return (
      <Password
        value={value || ""}
        placeholder={placeholder}
        onChange={(e) => onChange(meta.key, e.target.value)}
        {...extraProps}
      />
    );
  case "int":
    return (
      <Input
        type="number"
        value={value || ""}
        placeholder={placeholder}
        onChange={(e) => onChange(meta.key, e.target.value)}
        {...extraProps}
      />
    );
  case "color":
    return (
      <input
        type="color"
        value={value || "#000000"}
        onChange={(e) => onChange(meta.key, e.target.value)}
        style={{
          height: 32,
          width: 64,
          cursor: "pointer",
          border: "1px solid #d9d9d9",
          borderRadius: 6,
          padding: 2,
          background: "transparent",
        }}
        {...extraProps}
      />
    );
  case "url":
    return (
      <Input
        prefix={<LinkOutlined />}
        value={value || ""}
        placeholder={placeholder}
        onChange={(e) => onChange(meta.key, e.target.value)}
        {...extraProps}
      />
    );
  case "textarea":
    return (
      <TextArea
        value={value || ""}
        placeholder={placeholder}
        autoSize={{minRows: 2, maxRows: 8}}
        onChange={(e) => onChange(meta.key, e.target.value)}
        {...extraProps}
      />
    );
  case "json":
    return (
      <TextArea
        value={value || ""}
        placeholder={placeholder || "{\"key\": \"value\"}"}
        autoSize={{minRows: 3, maxRows: 10}}
        style={{fontFamily: "ui-monospace, SFMono-Regular, Menlo, Consolas, monospace", fontSize: 13}}
        onChange={(e) => onChange(meta.key, e.target.value)}
        {...extraProps}
      />
    );
  case "select":
    return (
      <Select
        value={value || undefined}
        placeholder={placeholder || i18next.t("general:Please select")}
        style={{width: "100%"}}
        onChange={(v) => onChange(meta.key, v)}
        {...extraProps}
      >
        {(meta.options || []).map((opt) => (
          <Option key={opt.value} value={opt.value}>
            {opt.label}
          </Option>
        ))}
      </Select>
    );
  case "string":
  default:
    return (
      <Input
        value={value || ""}
        placeholder={placeholder}
        onChange={(e) => onChange(meta.key, e.target.value)}
        {...extraProps}
      />
    );
  }
}

function groupByCategory(items) {
  const groups = {};
  for (const item of items) {
    const cat = item.category || "General";
    if (!groups[cat]) {groups[cat] = [];}
    groups[cat].push(item);
  }
  for (const cat of Object.keys(groups)) {
    groups[cat].sort((a, b) => (a.order || 0) - (b.order || 0));
  }
  return groups;
}

function getFieldSpan(type) {
  if (type === "textarea" || type === "json") {return 24;}
  if (type === "color" || type === "bool") {return 6;}
  if (type === "url") {return 12;}
  return 8;
}

class ConfigForm extends React.Component {
  constructor(props) {
    super(props);
    this.state = {
      values: props.initialValues || {},
    };
  }

  componentDidUpdate(prevProps) {
    if (prevProps.initialValues !== this.props.initialValues) {
      this.setState({values: this.props.initialValues || {}});
    }
  }

  handleChange = (key, value) => {
    const next = {...this.state.values, [key]: value};
    this.setState({values: next});
    if (this.props.onChange) {
      this.props.onChange(next);
    }
  };

  getValues() {
    return this.state.values;
  }

  renderField(meta) {
    const langKey = getLangKey();
    const value = this.state.values[meta.key] !== undefined
      ? this.state.values[meta.key]
      : (meta.currentValue !== undefined ? meta.currentValue : "");
    const span = this.props.fieldSpan || getFieldSpan(meta.type);
    const mobileSpan = Setting.isMobile() ? 22 : span;

    return (
      <Col
        key={meta.key}
        style={{marginTop: 12}}
        span={this.props.span || mobileSpan}
      >
        <div style={{
          marginBottom: 6,
          color: "var(--ant-color-text-secondary)",
          fontWeight: 500,
          lineHeight: "22px",
          fontSize: 13,
        }}>
          <FieldLabel meta={meta} langKey={langKey} />
          {meta.hasSiteOverride ? (
            <span style={{
              marginLeft: 6,
              fontSize: 11,
              padding: "0 6px",
              borderRadius: 4,
              background: "rgba(22,119,255,0.08)",
              color: "#1677ff",
              fontWeight: 500,
            }}>
              DB
            </span>
          ) : null}
        </div>
        {renderControl(meta, value, this.handleChange, langKey, this.props.controlProps?.[meta.key] || {})}
      </Col>
    );
  }

  renderCategory(catName, items) {
    const langKey = getLangKey();
    const {hideCategoryHeader} = this.props;

    const fields = items.map((m) => this.renderField(m));
    const fieldsNode = (
      <Row gutter={[16, 8]}>
        {fields}
      </Row>
    );

    if (hideCategoryHeader) {
      return fields;
    }

    const catLabels = CATEGORY_LABELS[catName] || {en: catName, zh: catName};
    const catDescs = CATEGORY_DESCRIPTIONS[catName] || {en: "", zh: ""};
    const title = i18nText(catLabels, langKey);
    const desc = i18nText(catDescs, langKey);

    if (this.props.renderCategory) {
      return (
        <div key={catName}>
          {this.props.renderCategory(catName, title, desc, fieldsNode)}
        </div>
      );
    }

    return (
      <div key={catName} style={{marginBottom: 16}}>
        {this.props.renderCategoryHeader
          ? this.props.renderCategoryHeader(catName, title, desc)
          : (
            <div style={{
              display: "flex",
              justifyContent: "space-between",
              alignItems: "baseline",
              marginBottom: 4,
              marginTop: 8,
              padding: "0 4px",
            }}>
              <div>
                <div style={{fontWeight: 600, fontSize: 15, color: "var(--ant-color-text)"}}>
                  {title}
                </div>
                {desc ? (
                  <div style={{fontSize: 12, color: "var(--ant-color-text-tertiary)", marginTop: 2}}>
                    {desc}
                  </div>
                ) : null}
              </div>
            </div>
          )}
        {fieldsNode}
      </div>
    );
  }

  render() {
    const {metadata = [], categories, hideCategoryHeader} = this.props;
    let grouped = groupByCategory(metadata);
    if (categories && categories.length > 0) {
      const filtered = {};
      for (const c of categories) {
        if (grouped[c]) {filtered[c] = grouped[c];}
      }
      grouped = filtered;
    }

    const categoryOrder = [
      "General", "Branding", "Content", "Auth", "Database", "Advanced",
    ];
    const orderedCats = Object.keys(grouped).sort((a, b) => {
      const ai = categoryOrder.indexOf(a);
      const bi = categoryOrder.indexOf(b);
      if (ai === -1 && bi === -1) {return a.localeCompare(b);}
      if (ai === -1) {return 1;}
      if (bi === -1) {return -1;}
      return ai - bi;
    });

    return (
      <div>
        {orderedCats.map((cat) => this.renderCategory(cat, grouped[cat]))}
        {hideCategoryHeader ? (
          <Row gutter={[16, 8]}>
            {Object.keys(grouped)
              .filter((c) => !categoryOrder.includes(c))
              .flatMap((c) => grouped[c].map((m) => this.renderField(m)))}
          </Row>
        ) : null}
      </div>
    );
  }
}

export default ConfigForm;

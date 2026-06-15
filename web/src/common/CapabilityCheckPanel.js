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
import {Alert, Button, Collapse, Tag, Typography, Space, Tooltip, message} from "antd";
import {
  CheckCircleOutlined,
  CloseCircleOutlined,
  WarningOutlined,
  MinusCircleOutlined,
  ReloadOutlined,
  CopyOutlined,
} from "@ant-design/icons";
import * as Setting from "../Setting";
import i18next from "i18next";

const {Panel} = Collapse;
const {Text, Paragraph} = Typography;

const STATUS_CONFIG = {
  passed: {
    color: "green",
    icon: <CheckCircleOutlined style={{color: "#52c41a"}} />,
    labelKey: "capability:Passed",
  },
  failed: {
    color: "red",
    icon: <CloseCircleOutlined style={{color: "#ff4d4f"}} />,
    labelKey: "capability:Failed",
  },
  warning: {
    color: "orange",
    icon: <WarningOutlined style={{color: "#faad14"}} />,
    labelKey: "capability:Warning",
  },
  skipped: {
    color: "default",
    icon: <MinusCircleOutlined style={{color: "#bfbfbf"}} />,
    labelKey: "capability:Skipped",
  },
};

class CapabilityCheckPanel extends React.Component {
  constructor(props) {
    super(props);
    this.state = {
      loading: false,
      result: null,
      expandedKeys: [],
    };
  }

  componentDidMount() {
    if (this.props.result !== undefined) {
      return;
    }
    if (this.props.autoCheck !== false) {
      this.runCheck();
    }
  }

  componentDidUpdate(prevProps) {
    if (this.props.result !== undefined) {
      return;
    }
    if (this.props.checkTrigger !== undefined && prevProps.checkTrigger !== this.props.checkTrigger) {
      this.runCheck();
    }
  }

  async runCheck() {
    const {onCheck} = this.props;
    if (!onCheck) {
      return;
    }

    if (this.props.result !== undefined) {
      onCheck();
      return;
    }

    this.setState({loading: true});
    try {
      const result = await onCheck();
      this.setState({result});
      const failedOrWarning = result.checks?.filter(
        (c) => c.status === "failed" || c.status === "warning"
      ).map((c) => c.name);
      this.setState({expandedKeys: failedOrWarning || []});
    } catch (error) {
      message.error(`${i18next.t("general:Failed to check")}: ${error.message}`);
    } finally {
      this.setState({loading: false});
    }
  }

  copyToClipboard(text) {
    navigator.clipboard.writeText(text).then(
      () => {
        message.success(i18next.t("general:Copied to clipboard"));
      },
      () => {
        message.error(i18next.t("general:Failed to copy"));
      }
    );
  }

  getStatusConfig(status) {
    return STATUS_CONFIG[status] || STATUS_CONFIG.skipped;
  }

  getOverallAlertType(status) {
    switch (status) {
      case "passed":
        return "success";
      case "failed":
        return "error";
      case "warning":
        return "warning";
      default:
        return "info";
    }
  }

  renderSummary(result) {
    if (!result) {
      return null;
    }

    const config = this.getStatusConfig(result.overallStatus);
    const alertType = this.getOverallAlertType(result.overallStatus);

    return (
      <Alert
        type={alertType}
        showIcon
        icon={config.icon}
        message={
          <Space>
            <span style={{fontWeight: 600}}>
              {i18next.t("capability:Overall Status")}: {i18next.t(config.labelKey)}
            </span>
            <Tag color={config.color}>
              {result.passedChecks}/{result.totalChecks} {i18next.t("capability:passed")}
            </Tag>
            {result.failedChecks > 0 && (
              <Tag color="red">
                {result.failedChecks} {i18next.t("capability:failed")}
              </Tag>
            )}
            {result.warningChecks > 0 && (
              <Tag color="orange">
                {result.warningChecks} {i18next.t("capability:warnings")}
              </Tag>
            )}
          </Space>
        }
        description={
          result.overallStatus === "failed"
            ? i18next.t("capability:Some checks failed. Please review the items below and fix the issues.")
            : result.overallStatus === "warning"
            ? i18next.t("capability:Some checks have warnings. Consider reviewing them.")
            : i18next.t("capability:All checks passed! The configuration looks good.")
        }
        style={{marginBottom: "12px"}}
      />
    );
  }

  renderCheckItem(check) {
    const config = this.getStatusConfig(check.status);

    return (
      <Panel
        key={check.name}
        header={
          <Space>
            {config.icon}
            <span style={{fontWeight: 500}}>{check.description}</span>
            <Tag color={config.color} style={{marginLeft: "auto"}}>
              {i18next.t(config.labelKey)}
            </Tag>
          </Space>
        }
      >
        <div style={{padding: "8px 4px"}}>
          <Paragraph style={{marginBottom: "12px", color: "var(--ant-color-text-secondary)"}}>
            {check.message}
          </Paragraph>

          {check.fixSuggestion && (
            <div style={{marginBottom: "12px"}}>
              <Text strong style={{color: "#faad14"}}>
                {i18next.t("capability:Fix Suggestion")}:
              </Text>
              <Paragraph style={{marginTop: "4px", marginBottom: 0}}>
                {check.fixSuggestion}
              </Paragraph>
            </div>
          )}

          {check.fixCommand && (
            <div
              style={{
                background: "var(--ant-color-fill-tertiary)",
                padding: "12px",
                borderRadius: "6px",
                fontFamily: "monospace",
                fontSize: "13px",
                position: "relative",
              }}
            >
              <Text code style={{wordBreak: "break-all", whiteSpace: "pre-wrap"}}>
                {check.fixCommand}
              </Text>
              <Tooltip title={i18next.t("general:Copy")}>
                <Button
                  type="text"
                  size="small"
                  icon={<CopyOutlined />}
                  style={{position: "absolute", top: "4px", right: "4px"}}
                  onClick={() => this.copyToClipboard(check.fixCommand)}
                />
              </Tooltip>
            </div>
          )}
        </div>
      </Panel>
    );
  }

  render() {
    const {title, description, showRefresh = true} = this.props;
    const {expandedKeys} = this.state;

    const loading = this.props.loading !== undefined ? this.props.loading : this.state.loading;
    const result = this.props.result !== undefined ? this.props.result : this.state.result;

    return (
      <div>
        <div style={{display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: "12px"}}>
          <div>
            <div style={{fontWeight: 600, fontSize: "15px"}}>
              {title || i18next.t("capability:Capability Check")}
            </div>
            {description && (
              <div
                style={{
                  fontSize: "13px",
                  color: "var(--ant-color-text-tertiary)",
                  fontWeight: 400,
                  marginTop: "2px",
                }}
              >
                {description}
              </div>
            )}
          </div>
          {showRefresh && (
            <Button
              type="primary"
              icon={<ReloadOutlined />}
              loading={loading}
              onClick={() => this.runCheck()}
            >
              {i18next.t("capability:Check Availability")}
            </Button>
          )}
        </div>

        {this.renderSummary(result)}

        {result && result.checks && result.checks.length > 0 && (
          <Collapse
            size="small"
            ghost
            activeKey={expandedKeys}
            onChange={(keys) => this.setState({expandedKeys: keys})}
          >
            {result.checks.map((check) => this.renderCheckItem(check))}
          </Collapse>
        )}

        {!result && !loading && (
          <div
            style={{
              padding: "24px",
              textAlign: "center",
              color: "var(--ant-color-text-tertiary)",
              border: "1px dashed var(--ant-color-border)",
              borderRadius: "8px",
            }}
          >
            <MinusCircleOutlined style={{fontSize: "24px", marginBottom: "8px", opacity: 0.5}} />
            <div>{i18next.t("capability:Click \"Check Availability\" to run diagnostics")}</div>
          </div>
        )}
      </div>
    );
  }
}

export default CapabilityCheckPanel;

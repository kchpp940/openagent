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
import {Alert, Button, Card, Col, Collapse, Descriptions, List, Progress, Row, Tag, Tooltip, Typography} from "antd";
import {CheckCircleOutlined, CloseCircleOutlined, CopyOutlined, ExclamationCircleOutlined, ReloadOutlined, SafetyCertificateOutlined} from "@ant-design/icons";
import i18next from "i18next";
import * as Setting from "../Setting";

const {Panel} = Collapse;
const {Text, Paragraph} = Typography;

function StatusIcon({status}) {
  switch (status) {
  case "pass":
    return <CheckCircleOutlined style={{color: "#52c41a"}} />;
  case "fail":
    return <CloseCircleOutlined style={{color: "#ff4d4f"}} />;
  case "warning":
    return <ExclamationCircleOutlined style={{color: "#faad14"}} />;
  case "skipped":
    return <ExclamationCircleOutlined style={{color: "#8c8c8c"}} />;
  case "pending":
    return <SafetyCertificateOutlined style={{color: "#1890ff", animation: "pulse 2s infinite"}} />;
  default:
    return null;
  }
}

function StatusTag({status}) {
  let color = "default";
  let text = status;
  switch (status) {
    case "pass":
      color = "success";
      text = i18next.t("capability:Passed");
      break;
    case "fail":
      color = "error";
      text = i18next.t("capability:Failed");
      break;
    case "warning":
      color = "warning";
      text = i18next.t("capability:Warnings");
      break;
    case "skipped":
      color = "default";
      text = i18next.t("capability:Skipped");
      break;
    case "pending":
      color = "processing";
      text = i18next.t("capability:Checking");
      break;
  }
  return <Tag color={color} style={{margin: 0}}>{text}</Tag>;
}

function OverallStatus({result}) {
  if (!result) return null;

  const {overallStatus, totalChecks, passedChecks, failedChecks, warningChecks} = result;

  let type = "success";
  let message = i18next.t("capability:All checks passed");
  let description = i18next.t("capability:No issues detected");

  if (overallStatus === "fail") {
    type = "error";
    message = i18next.t("capability:Some checks failed");
    description = `${failedChecks} ${i18next.t("capability:Failed").toLowerCase()}, ${warningChecks} ${i18next.t("capability:Warnings").toLowerCase()}`;
  } else if (overallStatus === "warning") {
    type = "warning";
    message = i18next.t("capability:Some warnings detected");
    description = `${warningChecks} ${i18next.t("capability:Warnings").toLowerCase()}`;
  }

  const percent = totalChecks > 0 ? Math.round((passedChecks / totalChecks) * 100) : 0;
  const progressStatus = overallStatus === "fail" ? "exception" : overallStatus === "warning" ? "normal" : "success";

  return (
    <div style={{marginBottom: "16px"}}>
      <Alert
        type={type}
        showIcon
        icon={<SafetyCertificateOutlined />}
        message={
          <span style={{fontWeight: 600, fontSize: "15px"}}>{message}</span>
        }
        description={description}
        action={
          <div style={{display: "flex", alignItems: "center", gap: "16px"}}>
            <div style={{textAlign: "center", width: "120px"}}>
              <Progress
                type="circle"
                size={60}
                percent={percent}
                status={progressStatus}
                format={(p) => `${p}%`}
              />
            </div>
            <Descriptions size="small" column={1} colon={false}>
              <Descriptions.Item label={i18next.t("capability:Passed")}>
                <Text strong style={{color: "#52c41a"}}>{passedChecks}</Text>
              </Descriptions.Item>
              <Descriptions.Item label={i18next.t("capability:Failed")}>
                <Text strong style={{color: "#ff4d4f"}}>{failedChecks}</Text>
              </Descriptions.Item>
              <Descriptions.Item label={i18next.t("capability:Warnings")}>
                <Text strong style={{color: "#faad14"}}>{warningChecks}</Text>
              </Descriptions.Item>
            </Descriptions>
          </div>
        }
      />
    </div>
  );
}

function CheckItem({item}) {
  const [copied, setCopied] = React.useState(false);

  const handleCopy = () => {
    if (item.fixCommand) {
      navigator.clipboard.writeText(item.fixCommand);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    }
  };

  return (
    <List.Item
      style={{
        padding: "12px 16px",
        background: item.status === "fail" ? "rgba(255, 77, 79, 0.05)" :
                   item.status === "warning" ? "rgba(250, 173, 20, 0.05)" :
                   "transparent",
        borderLeft: `3px solid ${
          item.status === "fail" ? "#ff4d4f" :
          item.status === "warning" ? "#faad14" :
          item.status === "pass" ? "#52c41a" : "#d9d9d9"
        }`,
        marginBottom: "8px",
        borderRadius: "8px",
      }}
    >
      <List.Item.Meta
        avatar={<StatusIcon status={item.status} />}
        title={
          <div style={{display: "flex", alignItems: "center", gap: "8px", marginBottom: "4px"}}>
            <Text strong style={{fontSize: "14px"}}>{item.name}</Text>
            <StatusTag status={item.status} />
            <Text type="secondary" style={{fontSize: "12px", marginLeft: "auto"}}>
              {item.durationMs}{i18next.t("capability:ms")}
            </Text>
          </div>
        }
        description={
          <div>
            <div style={{fontSize: "12px", color: "var(--ant-color-text-secondary)", marginBottom: item.message ? "8px"}}>
              {item.description}
            </div>
            {item.message && (
              <Paragraph style={{margin: 0, fontSize: "13px"}}>
                {item.message}
              </Paragraph>
            )}
            {item.fixCommand && (
              <div style={{marginTop: "8px", display: "flex", alignItems: "flex-start", gap: "8px"}}>
                <div style={{
                  flex: 1,
                  background: "#f5f5f5",
                  padding: "8px 12px",
                  borderRadius: "6px",
                  fontFamily: "monospace",
                  fontSize: "12px",
                  overflowX: "auto",
                  whiteSpace: "pre-wrap",
                  wordBreak: "break-all",
                }}>
                  {item.fixCommand}
                </div>
                <Tooltip title={copied ? i18next.t("capability:Copied") : i18next.t("capability:Copy command")}>
                  <Button
                    type="text"
                    size="small"
                    icon={<CopyOutlined />}
                    onClick={handleCopy}
                  />
                </Tooltip>
              </div>
            )}
            {item.fixHint && (
                <div style={{marginTop: "8px", fontSize: "12px", color: "var(--ant-color-text-secondary)"}}>
                  <Text strong>{i18next.t("capability:Hint")}:</Text> {item.fixHint}
                </div>
              )}
          </div>
        }
      />
    </List.Item>
  );
}

class CapabilityCheckPanel extends React.Component {
  constructor(props) {
    super(props);
    this.state = {
      checking: false,
      result: null,
      availability: null,
      expandedKeys: ["checks"],
    };
  }

  componentDidMount() {
    if (this.props.recordFn) {
      this.loadRecord();
    } else if (this.props.autoRun !== false && this.props.config && this.props.checkFn) {
      this.runChecks();
    }
  }

  componentDidUpdate(prevProps) {
    if (this.props.triggerKey !== undefined &&
        this.props.triggerKey !== prevProps.triggerKey) {
      if (this.props.recordFn) {
        this.loadRecord();
      } else if (this.props.config && this.props.checkFn) {
        this.runChecks();
      }
    }
  }

  loadRecord = () => {
    const {recordFn, recordParams} = this.props;
    if (!recordFn) return;

    this.setState({checking: true});

    const args = recordParams || [];
    recordFn(...args)
      .then((res) => {
        if (res.status === "ok") {
          const avail = res.data;
          this.setState({
            availability: avail,
            result: avail && avail.record ? avail.record.result : null,
          });
        } else {
          Setting.showMessage("error", `${i18next.t("general:Failed to get")}: ${res.msg}`);
        }
      })
      .catch((error) => {
        Setting.showMessage("error", `${i18next.t("general:Failed to connect to server")}: ${error}`);
      })
      .finally(() => {
        this.setState({checking: false});
      });
  };

  runCheckAndLoad = () => {
    const {runCheckFn, runCheckParams, recordFn} = this.props;
    if (!runCheckFn) {
      this.loadRecord();
      return;
    }

    this.setState({checking: true});

    const args = runCheckParams || [];
    runCheckFn(...args)
      .then((res) => {
        if (res.status === "ok") {
          const avail = res.data;
          this.setState({
            availability: avail,
            result: avail && avail.record ? avail.record.result : null,
          });
        } else {
          Setting.showMessage("error", `${i18next.t("general:Failed to run")}: ${res.msg}`);
          if (recordFn) {
            this.loadRecord();
          }
        }
      })
      .catch((error) => {
        Setting.showMessage("error", `${i18next.t("general:Failed to connect to server")}: ${error}`);
      })
      .finally(() => {
        this.setState({checking: false});
      });
  };

  runChecks = () => {
    const {config, checkFn} = this.props;
    if (!config || !checkFn) return;

    this.setState({checking: true, result: null});

    checkFn(config)
      .then((res) => {
        if (res.status === "ok") {
          this.setState({result: res.data});
        } else {
          Setting.showMessage("error", `${i18next.t("general:Failed to get")}: ${res.msg}`);
        }
      })
      .catch((error) => {
        Setting.showMessage("error", `${i18next.t("general:Failed to connect to server")}: ${error}`);
      })
      .finally(() => {
        this.setState({checking: false});
      });
  };

  render() {
    const {title, description, toolNames, recordFn, checkFn} = this.props;
    const {checking, result, expandedKeys, availability} = this.state;

    const hasRecord = !!(availability && availability.hasRecord);
    const configStale = !!(availability && availability.configStale);
    const isPending = availability && availability.status === "pending";
    const neverChecked = availability && !availability.hasRecord;

    const cardHeadStyle = {background: "transparent", borderBottom: "none", fontWeight: 600, fontSize: "15px", fontFamily: "Inter, -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif"};
    const sectionCardStyle = {
      marginBottom: "16px",
      borderRadius: "14px",
      boxShadow: "0 1px 3px rgba(0,0,0,0.06), 0 1px 2px rgba(0,0,0,0.04)",
      padding: "18px",
    };

    const renderCardTitle = (title, desc) => (
      <div>
        <div style={{display: "flex", alignItems: "center", gap: "8px"}}>
          <SafetyCertificateOutlined style={{fontSize: "16px"}} />
          <div>
            <div style={{fontWeight: 600, fontSize: "15px"}}>{title}</div>
            <div style={{fontSize: "13px", color: "var(--ant-color-text-tertiary)", fontWeight: 400, marginTop: "2px"}}>{desc}</div>
          </div>
        </div>
      </div>
    );

    return (
      <Card
        size="small"
        title={renderCardTitle(
          title || i18next.t("capability:Check availability"),
          description || i18next.t("capability:Check availability desc")
        )}
        style={sectionCardStyle}
        headStyle={cardHeadStyle}
        extra={
          recordFn ? (
            <Button
              type="primary"
              icon={checking ? null : <ReloadOutlined />}
              loading={checking}
              onClick={this.props.runCheckFn ? this.runCheckAndLoad : this.loadRecord}
            >
              {hasRecord ? i18next.t("capability:Re-check") : i18next.t("capability:Run checks")}
            </Button>
          ) : (
            <Button
              type="primary"
              icon={checking ? null : <ReloadOutlined />}
              loading={checking}
              onClick={this.runChecks}
              disabled={!checkFn}
            >
              {result ? i18next.t("capability:Re-check") : i18next.t("capability:Run checks")}
            </Button>
          )
        }
      >
        {checking && (
          <Alert
            type="info"
            showIcon
            message={i18next.t("capability:Checking...")}
            style={{marginBottom: "16px"}}
          />
        )}

        {!checking && isPending && (
          <Alert
            type="info"
            showIcon
            message={i18next.t("capability:Check in progress")}
            description={i18next.t("capability:Check in progress desc")}
            style={{marginBottom: "16px"}}
          />
        )}

        {!checking && configStale && (
          <Alert
            type="warning"
            showIcon
            message={i18next.t("capability:Config changed")}
            description={i18next.t("capability:Config changed desc")}
            style={{marginBottom: "16px"}}
          />
        )}

        {!checking && neverChecked && (
          <Alert
            type="warning"
            showIcon
            message={i18next.t("capability:Not checked yet")}
            description={i18next.t("capability:Not checked yet desc")}
            style={{marginBottom: "16px"}}
          />
        )}

        {result && <OverallStatus result={result} />}

        {result && result.toolNames && result.toolNames.length > 0 && (
          <div style={{marginBottom: "16px"}}>
            <Text type="secondary" style={{fontSize: "12px"}}>
              {i18next.t("tool:Functions")}:{" "}
            </Text>
            {result.toolNames.map((name, idx) => (
              <Tag key={idx} style={{fontFamily: "monospace", margin: "0 4px 4px 0"}}>
                {name}
              </Tag>
            ))}
          </div>
        )}

        {result && (
          <Collapse
            activeKey={expandedKeys}
            onChange={(keys) => this.setState({expandedKeys: keys})}
            ghost
          >
            <Panel
              header={
                <span>
                  {i18next.t("capability:Overall status")}
                  <Text type="secondary" style={{marginLeft: "8px", fontSize: "12px"}}>
                    ({result.items.length} {i18next.t("general:Checks").toLowerCase()})
                  </Text>
                </span>
              }
              key="checks"
            >
              <List
                dataSource={result.items}
                renderItem={(item) => <CheckItem key={item.id} item={item} />}
                split={false}
              />
            </Panel>
          </Collapse>
        )}

        {!result && !checking && (
          <div style={{textAlign: "center", padding: "40px 0", color: "var(--ant-color-text-tertiary)"}}>
            <SafetyCertificateOutlined style={{fontSize: "48px", opacity: 0.3, marginBottom: "12px"}} />
            <Paragraph style={{margin: 0}}>
              {i18next.t("capability:Run checks")} {i18next.t("capability:Check availability desc").toLowerCase()}
            </Paragraph>
          </div>
        )}
      </Card>
    );
  }
}

export default CapabilityCheckPanel;

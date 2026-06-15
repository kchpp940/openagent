// Copyright 2025 The OpenAgent Authors. All Rights Reserved.
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
import {Button, Card, Col, Descriptions, Row, Statistic, Table, Tag, Typography} from "antd";
import {ArrowLeftOutlined, CheckCircleOutlined, DiffOutlined, MinusCircleOutlined, PlusCircleOutlined, SyncOutlined} from "@ant-design/icons";
import BaseListPage from "./BaseListPage";
import * as Setting from "./Setting";
import * as FileBackend from "./backend/FileBackend";
import i18next from "i18next";

const {Title, Paragraph, Text} = Typography;

class FileVersionDiffPage extends BaseListPage {
  constructor(props) {
    super(props);
    this.state = {
      ...this.state,
      file: null,
      diffs: [],
      latestDiff: null,
      parseVersions: [],
      selectedDiff: null,
    };
  }

  UNSAFE_componentWillMount() {
    const {owner, name} = this.props.match.params;
    this.fetchData(owner, name);
  }

  fetchData(owner, name) {
    this.setState({loading: true});
    Promise.all([
      FileBackend.getFile(owner, name),
      FileBackend.getFileVersionDiffs(owner, name),
      FileBackend.getLatestFileVersionDiff(owner, name),
      FileBackend.getFileParseVersions(owner, name),
    ]).then(([fileRes, diffsRes, latestRes, versionsRes]) => {
      this.setState({loading: false});
      if (fileRes.status === "ok") {
        this.setState({file: fileRes.data});
      }
      if (diffsRes.status === "ok") {
        this.setState({diffs: diffsRes.data});
      }
      if (latestRes.status === "ok") {
        this.setState({latestDiff: latestRes.data, selectedDiff: latestRes.data});
      }
      if (versionsRes.status === "ok") {
        this.setState({parseVersions: versionsRes.data});
      }
    }).catch(error => {
      this.setState({loading: false});
      Setting.showMessage("error", `${i18next.t("general:Failed to fetch")}: ${error}`);
    });
  }

  getStatusTag(status) {
    const statusMap = {
      "Pending": {color: "default", text: i18next.t("file:Pending")},
      "Parsing": {color: "processing", text: i18next.t("file:Parsing")},
      "Vectorizing": {color: "processing", text: i18next.t("file:Vectorizing")},
      "Processing": {color: "processing", text: i18next.t("file:Processing")},
      "Finished": {color: "success", text: i18next.t("file:Finished")},
      "PartialFailed": {color: "warning", text: i18next.t("file:Partial Failed")},
      "Error": {color: "error", text: i18next.t("file:Error")},
    };
    const config = statusMap[status] || {color: "default", text: status};
    return <Tag color={config.color}>{config.text}</Tag>;
  }

  getDiffTypeTag(type) {
    const typeMap = {
      "Added": {color: "success", icon: <PlusCircleOutlined />, text: i18next.t("file:Added")},
      "Deleted": {color: "error", icon: <MinusCircleOutlined />, text: i18next.t("file:Deleted")},
      "Modified": {color: "warning", icon: <DiffOutlined />, text: i18next.t("file:Modified")},
      "Unchanged": {color: "default", icon: <CheckCircleOutlined />, text: i18next.t("file:Unchanged")},
    };
    const config = typeMap[type] || {color: "default", text: type};
    return <Tag color={config.color}>{config.icon} {config.text}</Tag>;
  }

  renderDiffSummary(diff) {
    if (!diff) {
      return null;
    }

    return (
      <Row gutter={16} style={{marginBottom: 16}}>
        <Col span={6}>
          <Statistic
            title={i18next.t("file:Added Chunks")}
            value={diff.addedCount}
            valueStyle={{color: "#3f8600"}}
            prefix={<PlusCircleOutlined />}
          />
        </Col>
        <Col span={6}>
          <Statistic
            title={i18next.t("file:Deleted Chunks")}
            value={diff.deletedCount}
            valueStyle={{color: "#cf1322"}}
            prefix={<MinusCircleOutlined />}
          />
        </Col>
        <Col span={6}>
          <Statistic
            title={i18next.t("file:Modified Chunks")}
            value={diff.modifiedCount}
            valueStyle={{color: "#fa8c16"}}
            prefix={<DiffOutlined />}
          />
        </Col>
        <Col span={6}>
          <Statistic
            title={i18next.t("file:Unchanged Chunks")}
            value={diff.unchangedCount}
            valueStyle={{color: "#1890ff"}}
            prefix={<CheckCircleOutlined />}
          />
        </Col>
      </Row>
    );
  }

  renderDiffInfo(diff) {
    if (!diff) {
      return null;
    }

    return (
      <Card style={{marginBottom: 16}}>
        <Descriptions bordered size="small" column={2}>
          <Descriptions.Item label={i18next.t("file:Version")}>
            v{diff.fromParseVersion} → v{diff.toParseVersion}
          </Descriptions.Item>
          <Descriptions.Item label={i18next.t("file:Status")}>
            {this.getStatusTag(diff.status)}
          </Descriptions.Item>
          <Descriptions.Item label={i18next.t("file:Created Time")}>
            {Setting.getFormattedDate(diff.createdTime)}
          </Descriptions.Item>
          <Descriptions.Item label={i18next.t("file:Content Hash")}>
            <Text code copyable>{diff.toContentHash?.substring(0, 16)}...</Text>
          </Descriptions.Item>
          {diff.errorText && (
            <Descriptions.Item label={i18next.t("file:Error")} span={2}>
              <Text type="danger">{diff.errorText}</Text>
            </Descriptions.Item>
          )}
        </Descriptions>
      </Card>
    );
  }

  renderChunkList(chunks, type) {
    if (!chunks || chunks.length === 0) {
      return null;
    }

    const columns = [
      {
        title: i18next.t("vector:Index"),
        dataIndex: "index",
        key: "index",
        width: 80,
      },
      {
        title: i18next.t("general:Type"),
        dataIndex: "type",
        key: "type",
        width: 120,
        render: (text) => this.getDiffTypeTag(text),
      },
      {
        title: i18next.t("general:Content"),
        key: "content",
        render: (_, record) => {
          if (type === "Deleted") {
            return (
              <div>
                <Paragraph style={{marginBottom: 8}}>
                  <Text strong>{i18next.t("file:Old Content")}:</Text>
                </Paragraph>
                <Paragraph style={{background: "#fff1f0", padding: 12, borderRadius: 4}}>
                  <Text delete>{record.oldText}</Text>
                </Paragraph>
              </div>
            );
          } else if (type === "Added") {
            return (
              <div>
                <Paragraph style={{marginBottom: 8}}>
                  <Text strong>{i18next.t("file:New Content")}:</Text>
                </Paragraph>
                <Paragraph style={{background: "#f6ffed", padding: 12, borderRadius: 4}}>
                  <Text mark>{record.newText}</Text>
                </Paragraph>
              </div>
            );
          } else {
            return (
              <div>
                <Paragraph style={{marginBottom: 8}}>
                  <Text strong>{i18next.t("file:Old Content")}:</Text>
                </Paragraph>
                <Paragraph style={{background: "#fff1f0", padding: 12, borderRadius: 4, marginBottom: 12}}>
                  <Text delete>{record.oldText}</Text>
                </Paragraph>
                <Paragraph style={{marginBottom: 8}}>
                  <Text strong>{i18next.t("file:New Content")}:</Text>
                </Paragraph>
                <Paragraph style={{background: "#f6ffed", padding: 12, borderRadius: 4}}>
                  <Text mark>{record.newText}</Text>
                </Paragraph>
              </div>
            );
          }
        },
      },
    ];

    const titles = {
      "Added": i18next.t("file:Added Chunks"),
      "Deleted": i18next.t("file:Deleted Chunks"),
      "Modified": i18next.t("file:Modified Chunks"),
    };

    return (
      <Card
        title={titles[type]}
        style={{marginBottom: 16}}
        size="small"
      >
        <Table
          columns={columns}
          dataSource={chunks}
          rowKey={(record, index) => `${type}-${record.index}-${index}`}
          pagination={false}
          size="small"
        />
      </Card>
    );
  }

  renderVersionHistory() {
    const {diffs} = this.state;
    if (!diffs || diffs.length === 0) {
      return null;
    }

    const columns = [
      {
        title: i18next.t("file:Version"),
        dataIndex: "version",
        key: "version",
        width: 100,
        render: (text) => `v${text}`,
      },
      {
        title: i18next.t("file:From → To"),
        key: "range",
        width: 120,
        render: (_, record) => `v${record.fromParseVersion} → v${record.toParseVersion}`,
      },
      {
        title: i18next.t("file:Status"),
        dataIndex: "status",
        key: "status",
        width: 120,
        render: (text) => this.getStatusTag(text),
      },
      {
        title: i18next.t("file:Added"),
        dataIndex: "addedCount",
        key: "addedCount",
        width: 80,
        render: (text) => <Tag color="success">+{text}</Tag>,
      },
      {
        title: i18next.t("file:Deleted"),
        dataIndex: "deletedCount",
        key: "deletedCount",
        width: 80,
        render: (text) => <Tag color="error">-{text}</Tag>,
      },
      {
        title: i18next.t("file:Modified"),
        dataIndex: "modifiedCount",
        key: "modifiedCount",
        width: 80,
        render: (text) => <Tag color="warning">±{text}</Tag>,
      },
      {
        title: i18next.t("general:Created time"),
        dataIndex: "createdTime",
        key: "createdTime",
        render: (text) => Setting.getFormattedDate(text),
      },
    ];

    return (
      <Card title={i18next.t("file:Version History")} style={{marginBottom: 16}}>
        <Table
          columns={columns}
          dataSource={diffs}
          rowKey="version"
          pagination={false}
          size="small"
          onRow={(record) => ({
            onClick: () => this.setState({selectedDiff: record}),
            style: {cursor: "pointer", background: this.state.selectedDiff?.version === record.version ? "#e6f7ff" : undefined},
          })}
        />
      </Card>
    );
  }

  render() {
    const {file, selectedDiff, loading} = this.state;

    return (
      <div style={{padding: 24}}>
        <Button
          icon={<ArrowLeftOutlined />}
          onClick={() => this.props.history.push("/files")}
          style={{marginBottom: 16}}
        >
          {i18next.t("general:Back to File List")}
        </Button>

        <Card loading={loading} style={{marginBottom: 16}}>
          <Title level={3} style={{marginBottom: 16}}>
            <DiffOutlined style={{marginRight: 8}} />
            {i18next.t("file:Version Compare")}
            {file && <Text type="secondary" style={{fontSize: 16, marginLeft: 12}}>{file.filename}</Text>}
          </Title>

          {file && (
            <Descriptions bordered size="small" column={3} style={{marginBottom: 16}}>
              <Descriptions.Item label={i18next.t("general:Owner")}>{file.owner}</Descriptions.Item>
              <Descriptions.Item label={i18next.t("general:Store")}>{file.store}</Descriptions.Item>
              <Descriptions.Item label={i18next.t("file:Parse Version")}>v{file.parseVersion || 0}</Descriptions.Item>
              <Descriptions.Item label={i18next.t("file:Vector Version")}>v{file.vectorVersion || 0}</Descriptions.Item>
              <Descriptions.Item label={i18next.t("general:Size")}>{Setting.getFormattedSize(file.size)}</Descriptions.Item>
              <Descriptions.Item label={i18next.t("general:Status")}>{this.getStatusTag(file.status)}</Descriptions.Item>
            </Descriptions>
          )}
        </Card>

        {this.renderDiffSummary(selectedDiff)}
        {this.renderDiffInfo(selectedDiff)}
        {selectedDiff && selectedDiff.status === "Vectorizing" && (
          <Card style={{marginBottom: 16}}>
            <div style={{textAlign: "center", padding: 20}}>
              <SyncOutlined spin style={{fontSize: 32, color: "#1890ff", marginBottom: 12}} />
              <Paragraph>{i18next.t("file:Generating vectors, please wait...")}</Paragraph>
            </div>
          </Card>
        )}
        {this.renderChunkList(selectedDiff?.addedChunks, "Added")}
        {this.renderChunkList(selectedDiff?.deletedChunks, "Deleted")}
        {this.renderChunkList(selectedDiff?.modifiedChunks, "Modified")}
        {this.renderVersionHistory()}
      </div>
    );
  }
}

export default FileVersionDiffPage;

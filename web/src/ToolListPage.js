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
import {Link} from "react-router-dom";
import {Button, Popconfirm, Switch, Table, Tag, Tooltip, Badge, Modal} from "antd";
import moment from "moment";
import BaseListPage from "./BaseListPage";
import * as Setting from "./Setting";
import * as ToolBackend from "./backend/ToolBackend";
import * as StoreBackend from "./backend/StoreBackend";
import i18next from "i18next";
import {DeleteOutlined, EditOutlined, SafetyOutlined, CheckCircleOutlined, CloseCircleOutlined, WarningOutlined, ClockCircleOutlined, HistoryOutlined} from "@ant-design/icons";
import CapabilityCheckPanel from "./common/CapabilityCheckPanel";

class ToolListPage extends BaseListPage {
  constructor(props) {
    super(props);
    this.state = {
      ...this.state,
      capabilityResults: {},
      checkingTools: {},
      checkModalVisible: false,
      currentCheckTool: null,
      historyModalVisible: false,
      currentHistoryTool: null,
      checkRecords: [],
      loadingHistory: false,
      selectedRecord: null,
    };
  }

  newTool() {
    const randomName = Setting.getRandomName();
    return {
      owner: "admin",
      name: `tool_${randomName}`,
      createdTime: moment().format(),
      displayName: "",
      displayName2: "",
      type: "time",
      subType: "Default",
      clientId: "",
      clientSecret: "",
      providerUrl: "",
      enableProxy: false,
      testContent: "",
      modelProvider: "",
      resultSummary: "",
      state: "Active",
    };
  }

  addTool() {
    const newTool = this.newTool();
    ToolBackend.addTool(newTool)
      .then((res) => {
        if (res.status === "ok") {
          Setting.showMessage("success", i18next.t("general:Successfully added"));
          this.props.history.push({
            pathname: `/tools/${newTool.name}`,
            state: {isNewTool: true},
          });
        } else {
          Setting.showMessage("error", `${i18next.t("general:Failed to add")}: ${res.msg}`);
        }
      })
      .catch(error => {
        Setting.showMessage("error", `${i18next.t("general:Failed to add")}: ${error}`);
      });
  }

  deleteItem = async(i) => {
    return ToolBackend.deleteTool(this.state.data[i]);
  };

  deleteTool(record) {
    ToolBackend.deleteTool(record)
      .then((res) => {
        if (res.status === "ok") {
          Setting.showMessage("success", i18next.t("general:Successfully deleted"));
          this.setState({
            data: this.state.data.filter((item) => item.name !== record.name),
            pagination: {
              ...this.state.pagination,
              total: this.state.pagination.total - 1,
            },
          });
        } else {
          Setting.showMessage("error", `${i18next.t("general:Failed to delete")}: ${res.msg}`);
        }
      })
      .catch(error => {
        Setting.showMessage("error", `${i18next.t("general:Failed to delete")}: ${error}`);
      });
  }

  checkToolCapability = (record) => {
    const toolName = record.name;

    this.setState((prevState) => ({
      checkingTools: {
        ...prevState.checkingTools,
        [toolName]: true,
      },
    }));

    ToolBackend.checkToolCapability(record)
      .then((res) => {
        if (res.status === "ok") {
          this.setState((prevState) => ({
            capabilityResults: {
              ...prevState.capabilityResults,
              [toolName]: res.data,
            },
            checkingTools: {
              ...prevState.checkingTools,
              [toolName]: false,
            },
          }));
        } else {
          Setting.showMessage("error", `${i18next.t("general:Failed to check")}: ${res.msg}`);
          this.setState((prevState) => ({
            checkingTools: {
              ...prevState.checkingTools,
              [toolName]: false,
            },
          }));
        }
      })
      .catch((error) => {
        Setting.showMessage("error", `${i18next.t("general:Failed to check")}: ${error}`);
        this.setState((prevState) => ({
          checkingTools: {
            ...prevState.checkingTools,
            [toolName]: false,
          },
        }));
      });
  };

  openCheckModal = (record) => {
    const toolName = record.name;
    const result = this.state.capabilityResults[toolName];

    this.setState({
      checkModalVisible: true,
      currentCheckTool: record,
    });

    if (!result) {
      this.checkToolCapability(record);
    }
  };

  closeCheckModal = () => {
    this.setState({
      checkModalVisible: false,
      currentCheckTool: null,
    });
  };

  openHistoryModal = (record) => {
    this.setState({
      historyModalVisible: true,
      currentHistoryTool: record,
      checkRecords: [],
      loadingHistory: true,
      selectedRecord: null,
    });

    const entityId = `${Setting.getOwner()}/${record.name}`;
    StoreBackend.getCapabilityCheckRecords("tool", entityId, 20)
      .then((res) => {
        if (res.status === "ok") {
          this.setState({
            checkRecords: res.data,
            loadingHistory: false,
          });
        } else {
          Setting.showMessage("error", `${i18next.t("general:Failed to get")}: ${res.msg}`);
          this.setState({loadingHistory: false});
        }
      })
      .catch((error) => {
        Setting.showMessage("error", `${i18next.t("general:Failed to get")}: ${error}`);
        this.setState({loadingHistory: false});
      });
  };

  closeHistoryModal = () => {
    this.setState({
      historyModalVisible: false,
      currentHistoryTool: null,
      checkRecords: [],
      selectedRecord: null,
    });
  };

  selectRecord = (record) => {
    this.setState({
      selectedRecord: record,
    });
  };

  getCapabilityStatusBadge = (tool) => {
    const toolName = tool.name;
    const isChecking = this.state.checkingTools[toolName];
    const frontendResult = this.state.capabilityResults[toolName];

    if (isChecking) {
      return <Badge status="processing" text={i18next.t("capability:Checking...")} />;
    }

    let status;
    if (frontendResult) {
      status = frontendResult.overallStatus;
    } else if (tool.latestCapabilityStatus) {
      status = tool.latestCapabilityStatus;
    } else {
      return (
        <span style={{cursor: "pointer"}} onClick={(e) => {e.stopPropagation(); this.openHistoryModal(tool);}}>
          <Tooltip title={i18next.t("capability:Click to view history")}>
            <Badge status="default" text={<span style={{display: "flex", alignItems: "center", gap: "4px"}}><HistoryOutlined />{i18next.t("capability:Not checked")}</span>} />
          </Tooltip>
        </span>
      );
    }

    let badge;
    switch (status) {
      case "passed":
        badge = <Badge status="success" text={i18next.t("capability:Passed")} />;
        break;
      case "failed":
        badge = <Badge status="error" text={i18next.t("capability:Failed")} />;
        break;
      case "warning":
        badge = <Badge status="warning" text={i18next.t("capability:Warning")} />;
        break;
      case "pending":
        badge = <Badge status="processing" text={i18next.t("capability:Checking...")} />;
        break;
      default:
        badge = <Badge status="default" text={status} />;
    }

    return (
      <span style={{cursor: "pointer"}} onClick={(e) => {e.stopPropagation(); this.openHistoryModal(tool);}}>
        <Tooltip title={i18next.t("capability:Click to view history")}>
          {badge}
        </Tooltip>
      </span>
    );
  };

  renderTable(tools) {
    const columns = [
      {
        title: i18next.t("general:Name"),
        dataIndex: "name",
        key: "name",
        width: "180px",
        sorter: (a, b) => a.name.localeCompare(b.name),
        ...this.getColumnSearchProps("name"),
        render: (text) => (
          <Link to={`/tools/${text}`}>{text}</Link>
        ),
      },
      {
        title: i18next.t("general:Type"),
        dataIndex: "type",
        key: "type",
        width: "180px",
        filterMultiple: false,
        filters: Setting.getProviderTypeOptions("Tool").map((o) => ({text: o.name, value: o.name})),
        onFilter: (value, record) => record.type === value,
        sorter: (a, b) => a.type.localeCompare(b.type),
        render: (text, record) => (
          <span>
            <img width={20} height={20} style={{marginBottom: "3px", marginRight: "6px"}}
              src={Setting.getProviderLogoURL({category: "Tool", type: text})} alt={text} />
            {text}
          </span>
        ),
      },
      {
        title: i18next.t("provider:Sub type"),
        dataIndex: "subType",
        key: "subType",
        width: "150px",
        sorter: (a, b) => (a.subType || "").localeCompare(b.subType || ""),
        ...this.getColumnSearchProps("subType"),
      },
      {
        title: i18next.t("tool:Functions"),
        key: "functions",
        render: (_, record) => (
          <div style={{display: "flex", flexDirection: "column", gap: "4px"}}>
            {Setting.getToolFunctions(record).map((f) => (
              <Tag key={f.name} style={{fontFamily: "monospace", margin: 0}}>{f.name}</Tag>
            ))}
          </div>
        ),
      },
      {
        title: i18next.t("provider:Enable proxy"),
        dataIndex: "enableProxy",
        key: "enableProxy",
        width: "130px",
        render: (text, record) => {
          if (!["web_search", "web_fetch", "web_browser", "browser_use"].includes(record.type)) {
            return null;
          }
          return <Switch disabled checked={text} />;
        },
      },
      {
        title: i18next.t("general:State"),
        dataIndex: "state",
        key: "state",
        width: "110px",
        sorter: (a, b) => (a.state || "").localeCompare(b.state || ""),
      },
      {
        title: i18next.t("capability:Availability"),
        dataIndex: "capability",
        key: "capability",
        width: "130px",
        render: (_, record) => this.getCapabilityStatusBadge(record),
      },
      {
        title: i18next.t("general:Action"),
        dataIndex: "action",
        key: "action",
        width: "180px",
        fixed: "right",
        render: (text, record) => (
          <div style={{display: "flex", alignItems: "center", gap: "2px", flexWrap: "nowrap"}}>
            <Tooltip title={i18next.t("capability:Check Availability")}>
              <Button
                type="text"
                size="small"
                icon={<SafetyOutlined />}
                loading={this.state.checkingTools[record.name]}
                style={{minWidth: "28px", width: "28px", height: "28px", padding: 0, borderRadius: "6px"}}
                onClick={() => this.openCheckModal(record)}
              />
            </Tooltip>
            <Tooltip title={i18next.t("general:Edit")}>
              <Button type="text" size="small" icon={<EditOutlined />} style={{minWidth: "28px", width: "28px", height: "28px", padding: 0, borderRadius: "6px"}} onClick={() => this.props.history.push(`/tools/${record.name}`)} />
            </Tooltip>
            <Popconfirm
              title={`${i18next.t("general:Sure to delete")}: ${record.name}?`}
              onConfirm={() => this.deleteTool(record)}
              okText={i18next.t("general:OK")}
              cancelText={i18next.t("general:Cancel")}
            >
              <Tooltip title={i18next.t("general:Delete")}>
                <Button type="text" size="small" danger icon={<DeleteOutlined />} style={{minWidth: "28px", width: "28px", height: "28px", padding: 0, borderRadius: "6px"}} />
              </Tooltip>
            </Popconfirm>
          </div>
        ),
      },
    ];

    const paginationProps = {
      total: this.state.pagination.total,
      showQuickJumper: true,
      showSizeChanger: true,
      pageSizeOptions: ["10", "20", "50", "100"],
      showTotal: (total) => i18next.t("general:{total} in total").replace("{total}", total),
    };

    return (
      <div>
        <Table
          scroll={{x: "max-content"}}
          columns={columns}
          dataSource={tools}
          rowKey="name"
          size="middle"
          bordered
          pagination={paginationProps}
          title={() => (
            <div>
              {i18next.t("general:Tools")}&nbsp;&nbsp;&nbsp;&nbsp;
              <Button type="primary" size="small" onClick={() => this.addTool()}>
                {i18next.t("general:Add")}
              </Button>
            </div>
          )}
          loading={this.state.loading}
          onChange={this.handleTableChange}
        />
        <Modal
          title={i18next.t("capability:Tool Capability Check") + " - " + (this.state.currentCheckTool?.name || "")}
          open={this.state.checkModalVisible}
          onCancel={this.closeCheckModal}
          width={720}
          footer={[
            <Button key="close" onClick={this.closeCheckModal}>
              {i18next.t("general:Close")}
            </Button>,
          ]}
        >
          {this.state.currentCheckTool && (
            <CapabilityCheckPanel
              result={this.state.capabilityResults[this.state.currentCheckTool.name]}
              loading={this.state.checkingTools[this.state.currentCheckTool.name]}
              title={i18next.t("capability:Verify tool desc")}
              description={i18next.t("capability:Check if the tool is properly configured and functional")}
              checkType="tool"
              entityId={this.state.currentCheckTool.name}
              entity={this.state.currentCheckTool}
              onCheck={() => this.checkToolCapability(this.state.currentCheckTool)}
            />
          )}
        </Modal>
        <Modal
          title={
            <span style={{display: "flex", alignItems: "center", gap: "8px"}}>
              <HistoryOutlined />
              {i18next.t("capability:Check History") + " - " + (this.state.currentHistoryTool?.name || "")}
            </span>
          }
          open={this.state.historyModalVisible}
          onCancel={this.closeHistoryModal}
          width={900}
          footer={[
            <Button key="close" onClick={this.closeHistoryModal}>
              {i18next.t("general:Close")}
            </Button>,
          ]}
        >
          <div style={{display: "flex", gap: "16px", minHeight: "400px"}}>
            <div style={{width: "300px", borderRight: "1px solid #f0f0f0", paddingRight: "16px"}}>
              <div style={{fontWeight: "bold", marginBottom: "12px"}}>{i18next.t("capability:Check Records")}</div>
              {this.state.loadingHistory ? (
                <div style={{textAlign: "center", padding: "20px"}}>{i18next.t("general:Loading")}...</div>
              ) : this.state.checkRecords.length === 0 ? (
                <div style={{textAlign: "center", padding: "20px", color: "#999"}}>{i18next.t("capability:No check records")}</div>
              ) : (
                this.state.checkRecords.map((record, index) => (
                  <div
                    key={record.id}
                    onClick={() => this.selectRecord(record)}
                    style={{
                      padding: "10px",
                      marginBottom: "8px",
                      borderRadius: "6px",
                      cursor: "pointer",
                      backgroundColor: this.state.selectedRecord?.id === record.id ? "#e6f7ff" : "transparent",
                      border: this.state.selectedRecord?.id === record.id ? "1px solid #91d5ff" : "1px solid transparent",
                    }}
                  >
                    <div style={{display: "flex", alignItems: "center", gap: "8px", marginBottom: "4px"}}>
                      {record.status === "passed" && <CheckCircleOutlined style={{color: "#52c41a"}} />
                      {record.status === "failed" && <CloseCircleOutlined style={{color: "#ff4d4f"}} />
                      {record.status === "warning" && <WarningOutlined style={{color: "#faad14"}} />
                      {record.status === "pending" && <ClockCircleOutlined style={{color: "#1890ff"}} />
                      <span style={{fontWeight: "500"}}>{i18next.t(`capability:${record.status}`)}</span>
                    </div>
                    <div style={{fontSize: "12px", color: "#666"}}>
                      {record.checkedAt ? moment(record.checkedAt).format("YYYY-MM-DD HH:mm:ss") : "-"}
                    </div>
                    {record.configHash && (
                      <div style={{fontSize: "11px", color: "#999", marginTop: "4px", fontFamily: "monospace"}}>
                        hash: {record.configHash.substring(0, 12)}...
                      </div>
                    )}
                  </div>
                ))
              )}
            </div>
            <div style={{flex: 1, paddingLeft: "16px"}}>
              {this.state.selectedRecord ? (
                <div>
                <div style={{fontWeight: "bold", marginBottom: "12px"}}>{i18next.t("capability:Check Details")}</div>
                <CapabilityCheckPanel
                  result={this.state.selectedRecord.checkResult}
                  loading={false}
                  title={i18next.t("capability:Check Result")}
                  description={`${i18next.t("capability:Checked at")}: ${this.state.selectedRecord.checkedAt ? moment(this.state.selectedRecord.checkedAt).format("YYYY-MM-DD HH:mm:ss") : "-"}`}
                  checkType="tool"
                  entityId={this.state.currentHistoryTool?.name || ""}
                  entity={this.state.currentHistoryTool}
                  showRefresh={false}
                />
                </div>
              ) : (
                <div style={{textAlign: "center", padding: "40px", color: "#999"}}>
                  {this.state.checkRecords.length > 0 ? i18next.t("capability:Select a record to view details") : i18next.t("capability:No check records available")}
                </div>
              )}
            </div>
          </div>
        </Modal>
      </div>
    );
  }

  fetch = (params = {}) => {
    const {pagination} = params;
    this.setState({loading: true});
    ToolBackend.getTools("admin", pagination.current, pagination.pageSize, this.state.searchField, this.state.searchValue, params.sortField, params.sortOrder)
      .then((res) => {
        if (res.status === "ok") {
          this.setState({
            loading: false,
            data: res.data,
            pagination: {
              ...pagination,
              total: res.data2,
            },
          });
        } else {
          if (res.status === "error" && res.msg === "Unauthorized") {
            this.setState({isAuthorized: false, loading: false});
          } else {
            Setting.showMessage("error", `${i18next.t("general:Failed to get")}: ${res.msg}`);
          }
        }
      });
  };
}

export default ToolListPage;

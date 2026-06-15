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
import {Button, Popconfirm, Table, Tag, Tooltip, Badge, Modal} from "antd";
import moment from "moment";
import BaseListPage from "./BaseListPage";
import * as Setting from "./Setting";
import * as SkillBackend from "./backend/SkillBackend";
import * as StoreBackend from "./backend/StoreBackend";
import i18next from "i18next";
import {DeleteOutlined, DownloadOutlined, EditOutlined, ShopOutlined, SafetyOutlined, HistoryOutlined, CheckCircleOutlined, CloseCircleOutlined, WarningOutlined, ClockCircleOutlined} from "@ant-design/icons";
import LoadSkillModal from "./LoadSkillModal";
import SkillMarketplaceModal from "./SkillMarketplaceModal";
import CapabilityCheckPanel from "./common/CapabilityCheckPanel";

const SKILL_TYPES = ["writing", "coding", "analysis", "translation", "reasoning", "search", "custom"];

class SkillListPage extends BaseListPage {
  constructor(props) {
    super(props);
    this.state = {
      ...this.state,
      loadModalVisible: false,
      marketplaceVisible: false,
      capabilityResults: {},
      checkingSkills: {},
      checkModalVisible: false,
      currentCheckSkill: null,
      historyModalVisible: false,
      currentHistorySkill: null,
      checkRecords: [],
      loadingHistory: false,
      selectedRecord: null,
    };
  }

  newSkill() {
    const randomName = Setting.getRandomName();
    return {
      owner: "admin",
      name: `skill_${randomName}`,
      createdTime: moment().format(),
      displayName: "",
      type: "custom",
      description: "",
      homepage: "",
      emoji: "",
      metadata: "",
      content: "",
      skillMd: "",
      references: [],
      state: "Active",
    };
  }

  addSkill() {
    const newSkill = this.newSkill();
    SkillBackend.addSkill(newSkill)
      .then((res) => {
        if (res.status === "ok") {
          Setting.showMessage("success", i18next.t("general:Successfully added"));
          this.props.history.push({
            pathname: `/skills/${newSkill.name}`,
            state: {isNewSkill: true},
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
    return SkillBackend.deleteSkill(this.state.data[i]);
  };

  deleteSkill(record) {
    SkillBackend.deleteSkill(record)
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

  checkSkillCapability = (record) => {
    const skillName = record.name;

    this.setState((prevState) => ({
      checkingSkills: {
        ...prevState.checkingSkills,
        [skillName]: true,
      },
    }));

    SkillBackend.checkSkillCapability(record)
      .then((res) => {
        if (res.status === "ok") {
          this.setState((prevState) => ({
            capabilityResults: {
              ...prevState.capabilityResults,
              [skillName]: res.data,
            },
            checkingSkills: {
              ...prevState.checkingSkills,
              [skillName]: false,
            },
          }));
        } else {
          Setting.showMessage("error", `${i18next.t("general:Failed to check")}: ${res.msg}`);
          this.setState((prevState) => ({
            checkingSkills: {
              ...prevState.checkingSkills,
              [skillName]: false,
            },
          }));
        }
      })
      .catch((error) => {
        Setting.showMessage("error", `${i18next.t("general:Failed to check")}: ${error}`);
        this.setState((prevState) => ({
          checkingSkills: {
            ...prevState.checkingSkills,
            [skillName]: false,
          },
        }));
      });
  };

  openCheckModal = (record) => {
    const skillName = record.name;
    const result = this.state.capabilityResults[skillName];

    this.setState({
      checkModalVisible: true,
      currentCheckSkill: record,
    });

    if (!result) {
      this.checkSkillCapability(record);
    }
  };

  closeCheckModal = () => {
    this.setState({
      checkModalVisible: false,
      currentCheckSkill: null,
    });
  };

  openHistoryModal = (record) => {
    this.setState({
      historyModalVisible: true,
      currentHistorySkill: record,
      checkRecords: [],
      loadingHistory: true,
      selectedRecord: null,
    });

    const entityId = `${Setting.getOwner()}/${record.name}`;
    StoreBackend.getCapabilityCheckRecords("skill", entityId, 20)
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
      currentHistorySkill: null,
      checkRecords: [],
      selectedRecord: null,
    });
  };

  selectRecord = (record) => {
    this.setState({
      selectedRecord: record,
    });
  };

  getSkillCapabilityStatusBadge = (skill) => {
    const skillName = skill.name;
    const isChecking = this.state.checkingSkills[skillName];
    const frontendResult = this.state.capabilityResults[skillName];

    if (isChecking) {
      return <Badge status="processing" text={i18next.t("capability:Checking...")} />;
    }

    let status;
    if (frontendResult) {
      status = frontendResult.overallStatus;
    } else if (skill.latestCapabilityStatus) {
      status = skill.latestCapabilityStatus;
    } else {
      return (
        <span style={{cursor: "pointer"}} onClick={(e) => {e.stopPropagation(); this.openHistoryModal(skill);}}>
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
      <span style={{cursor: "pointer"}} onClick={(e) => {e.stopPropagation(); this.openHistoryModal(skill);}}>
        <Tooltip title={i18next.t("capability:Click to view history")}>
          {badge}
        </Tooltip>
      </span>
    );
  };

  renderTable(skills) {
    const columns = [
      {
        title: i18next.t("general:Name"),
        dataIndex: "name",
        key: "name",
        width: "200px",
        sorter: (a, b) => a.name.localeCompare(b.name),
        ...this.getColumnSearchProps("name"),
        render: (text, record) => (
          <span>
            {record.emoji && <span style={{marginRight: 6}}>{record.emoji}</span>}
            <Link to={`/skills/${text}`}>{text}</Link>
          </span>
        ),
      },
      {
        title: i18next.t("general:Display name"),
        dataIndex: "displayName",
        key: "displayName",
        width: "160px",
        sorter: (a, b) => (a.displayName || "").localeCompare(b.displayName || ""),
        ...this.getColumnSearchProps("displayName"),
      },
      {
        title: i18next.t("general:Type"),
        dataIndex: "type",
        key: "type",
        width: "120px",
        filterMultiple: false,
        filters: SKILL_TYPES.map((t) => ({text: t, value: t})),
        onFilter: (value, record) => record.type === value,
        sorter: (a, b) => (a.type || "").localeCompare(b.type || ""),
        render: (text) => <Tag color="blue">{text}</Tag>,
      },
      {
        title: i18next.t("general:Description"),
        dataIndex: "description",
        key: "description",
        width: "200px",
        render: (text) => (text ? Setting.getShortText(text, 20) : null),
        ...this.getColumnSearchProps("description"),
      },
      {
        title: i18next.t("skill:References"),
        key: "references",
        width: "160px",
        render: (_, record) => {
          const refs = record.references || [];
          if (refs.length === 0) {
            return null;
          }
          return (
            <div style={{display: "flex", flexDirection: "column", gap: "3px"}}>
              {refs.map(r => (
                <Tag key={r.name} style={{fontFamily: "monospace", margin: 0}}>{r.name}</Tag>
              ))}
            </div>
          );
        },
      },
      {
        title: i18next.t("general:State"),
        dataIndex: "state",
        key: "state",
        width: "100px",
        sorter: (a, b) => (a.state || "").localeCompare(b.state || ""),
      },
      {
        title: i18next.t("capability:Availability"),
        dataIndex: "capability",
        key: "capability",
        width: "130px",
        render: (_, record) => this.getSkillCapabilityStatusBadge(record),
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
                loading={this.state.checkingSkills[record.name]}
                style={{minWidth: "28px", width: "28px", height: "28px", padding: 0, borderRadius: "6px"}}
                onClick={() => this.openCheckModal(record)}
              />
            </Tooltip>
            <Tooltip title={i18next.t("general:Edit")}>
              <Button type="text" size="small" icon={<EditOutlined />} style={{minWidth: "28px", width: "28px", height: "28px", padding: 0, borderRadius: "6px"}} onClick={() => this.props.history.push(`/skills/${record.name}`)} />
            </Tooltip>
            <Popconfirm
              title={`${i18next.t("general:Sure to delete")}: ${record.name}?`}
              onConfirm={() => this.deleteSkill(record)}
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
        <LoadSkillModal
          open={this.state.loadModalVisible}
          onClose={() => this.setState({loadModalVisible: false})}
          onImported={(skillName) => this.props.history.push(`/skills/${skillName}`)}
        />
        <SkillMarketplaceModal
          open={this.state.marketplaceVisible}
          onClose={() => this.setState({marketplaceVisible: false})}
          onInstalled={(skillName) => {
            this.setState({marketplaceVisible: false});
            this.props.history.push(`/skills/${skillName}`);
          }}
          installedNames={(this.state.data || []).map((s) => s.name)}
        />
        <Table
          scroll={{x: "max-content"}}
          columns={columns}
          dataSource={skills}
          rowKey="name"
          size="middle"
          bordered
          pagination={paginationProps}
          title={() => (
            <div>
              {i18next.t("general:Skills")}&nbsp;&nbsp;&nbsp;&nbsp;
              <Button type="primary" size="small" onClick={() => this.addSkill()}>
                {i18next.t("general:Add")}
              </Button>
              &nbsp;&nbsp;
              <Button
                size="small"
                icon={<DownloadOutlined />}
                onClick={() => this.setState({loadModalVisible: true})}
              >
                {i18next.t("skill:Load Existing Skill")}
              </Button>
              &nbsp;&nbsp;
              <Button
                size="small"
                type="default"
                icon={<ShopOutlined />}
                onClick={() => this.setState({marketplaceVisible: true})}
              >
                {i18next.t("skill:Marketplace")}
              </Button>
            </div>
          )}
          loading={this.state.loading}
          onChange={this.handleTableChange}
        />
        <Modal
          title={i18next.t("capability:Skill Capability Check") + " - " + (this.state.currentCheckSkill?.name || "")}
          open={this.state.checkModalVisible}
          onCancel={this.closeCheckModal}
          width={720}
          footer={[
            <Button key="close" onClick={this.closeCheckModal}>
              {i18next.t("general:Close")}
            </Button>,
          ]}
        >
          {this.state.currentCheckSkill && (
            <CapabilityCheckPanel
              result={this.state.capabilityResults[this.state.currentCheckSkill.name]}
              loading={this.state.checkingSkills[this.state.currentCheckSkill.name]}
              title={i18next.t("capability:Verify skill desc")}
              description={i18next.t("capability:Check if the skill is properly configured and ready to use")}
              checkType="skill"
              entityId={this.state.currentCheckSkill.name}
              entity={this.state.currentCheckSkill}
              onCheck={() => this.checkSkillCapability(this.state.currentCheckSkill)}
            />
          )}
        </Modal>
        <Modal
          title={
            <span style={{display: "flex", alignItems: "center", gap: "8px"}}>
              <HistoryOutlined />
              {i18next.t("capability:Check History") + " - " + (this.state.currentHistorySkill?.name || "")}
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
                      {record.status === "passed" && <CheckCircleOutlined style={{color: "#52c41a"}} />}
                      {record.status === "failed" && <CloseCircleOutlined style={{color: "#ff4d4f"}} />}
                      {record.status === "warning" && <WarningOutlined style={{color: "#faad14"}} />}
                      {record.status === "pending" && <ClockCircleOutlined style={{color: "#1890ff"}} />}
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
                  checkType="skill"
                  entityId={this.state.currentHistorySkill?.name || ""}
                  entity={this.state.currentHistorySkill}
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
    SkillBackend.getSkills("admin", pagination.current, pagination.pageSize, this.state.searchField, this.state.searchValue, params.sortField, params.sortOrder)
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

export default SkillListPage;

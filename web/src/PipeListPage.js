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
import {Button, Popconfirm, Switch, Table, Tooltip} from "antd";
import moment from "moment";
import BaseListPage from "./BaseListPage";
import * as Setting from "./Setting";
import * as PipeBackend from "./backend/PipeBackend";
import i18next from "i18next";
import {DeleteOutlined, EditOutlined} from "@ant-design/icons";

class PipeListPage extends BaseListPage {
  constructor(props) {
    super(props);
  }

  newPipe() {
    const randomName = Setting.getRandomName();
    return {
      owner: "admin",
      name: `pipe_${randomName}`,
      createdTime: moment().format(),
      displayName: `New Pipe - ${randomName}`,
      type: "Telegram",
      token: "",
      secretKey: "",
      store: "store-built-in",
      domain: "",
      isDefault: false,
      state: "Active",
    };
  }

  addPipe() {
    const newPipe = this.newPipe();
    PipeBackend.addPipe(newPipe)
      .then(() => {
        Setting.ResponseAdapter.showSuccessMessage(i18next.t("general:Successfully added"));
        this.props.history.push({
          pathname: `/pipes/${newPipe.name}`,
          state: {isNewPipe: true},
        });
      })
      .catch(error => {
        Setting.ResponseAdapter.showErrorMessage(error, i18next.t("general:Failed to add"));
      });
  }

  deleteItem = async(i) => {
    return PipeBackend.deletePipe(this.state.data[i]);
  };

  deletePipe(record) {
    PipeBackend.deletePipe(record)
      .then(() => {
        Setting.ResponseAdapter.showSuccessMessage(i18next.t("general:Successfully deleted"));
        this.setState({
          data: this.state.data.filter((item) => item.name !== record.name),
          pagination: {
            ...this.state.pagination,
            total: this.state.pagination.total - 1,
          },
        });
      })
      .catch(error => {
        Setting.ResponseAdapter.showErrorMessage(error, i18next.t("general:Failed to delete"));
      });
  }

  renderTable(pipes) {
    const columns = [
      {
        title: i18next.t("general:Name"),
        dataIndex: "name",
        key: "name",
        width: "180px",
        sorter: (a, b) => a.name.localeCompare(b.name),
        ...this.getColumnSearchProps("name"),
        render: (text) => (
          <Link to={`/pipes/${text}`}>{text}</Link>
        ),
      },
      {
        title: i18next.t("general:Display name"),
        dataIndex: "displayName",
        key: "displayName",
        width: "220px",
        sorter: (a, b) => (a.displayName || "").localeCompare(b.displayName || ""),
        ...this.getColumnSearchProps("displayName"),
      },
      {
        title: i18next.t("general:Type"),
        dataIndex: "type",
        key: "type",
        width: "150px",
        align: "center",
        filterMultiple: false,
        filters: [
          {text: "Telegram", value: "Telegram"},
          {text: "Discord", value: "Discord"},
          {text: "WhatsApp", value: "WhatsApp"},
          {text: "Slack", value: "Slack"},
        ],
        sorter: (a, b) => a.type.localeCompare(b.type),
        render: (text, record) => (
          <span>
            <img width={20} height={20} style={{marginBottom: "3px", marginRight: "6px"}}
              src={Setting.getProviderLogoURL({category: "Chat", type: text})}
              alt={text} />
            {text}
          </span>
        ),
      },
      {
        title: i18next.t("provider:Domain"),
        dataIndex: "domain",
        key: "domain",
        width: "220px",
        sorter: (a, b) => (a.domain || "").localeCompare(b.domain || ""),
      },
      {
        title: i18next.t("store:Is default"),
        dataIndex: "isDefault",
        key: "isDefault",
        width: "120px",
        render: (text) => (
          <Switch disabled checkedChildren={i18next.t("general:ON")} unCheckedChildren={i18next.t("general:OFF")} checked={text} />
        ),
      },
      {
        title: i18next.t("general:State"),
        dataIndex: "state",
        key: "state",
        width: "90px",
        sorter: (a, b) => a.state.localeCompare(b.state),
      },
      {
        title: i18next.t("general:Action"),
        dataIndex: "action",
        key: "action",
        width: "120px",
        fixed: "right",
        render: (text, record) => (
          <div style={{display: "flex", alignItems: "center", gap: "2px", flexWrap: "nowrap"}}>
            <Tooltip title={i18next.t("general:Edit")}>
              <Button
                type="text"
                size="small"
                icon={<EditOutlined />}
                onClick={() => this.props.history.push(`/pipes/${record.name}`)}
                style={{minWidth: "28px", width: "28px", height: "28px", padding: 0, borderRadius: "6px"}}
              />
            </Tooltip>
            <Popconfirm
              title={`${i18next.t("general:Sure to delete")}: ${record.name} ?`}
              onConfirm={() => this.deletePipe(record)}
              okText={i18next.t("general:OK")}
              cancelText={i18next.t("general:Cancel")}
            >
              <Tooltip title={i18next.t("general:Delete")}>
                <Button
                  type="text"
                  size="small"
                  danger
                  icon={<DeleteOutlined />}
                  style={{minWidth: "28px", width: "28px", height: "28px", padding: 0, borderRadius: "6px"}}
                />
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
      showTotal: () => i18next.t("general:{total} in total").replace("{total}", this.state.pagination.total),
    };

    return (
      <div>
        <Table
          scroll={{x: "max-content"}}
          columns={columns}
          dataSource={pipes}
          rowKey="name"
          rowSelection={this.getRowSelection()}
          size="middle"
          bordered
          pagination={paginationProps}
          title={() => (
            <div>
              {i18next.t("general:Pipes")}&nbsp;&nbsp;&nbsp;&nbsp;
              <Button type="primary" size="small" onClick={() => this.addPipe()}>{i18next.t("general:Add")}</Button>
              {this.state.selectedRowKeys.length > 0 && (
                <Popconfirm
                  title={`${i18next.t("general:Sure to delete")}: ${this.state.selectedRowKeys.length} ${i18next.t("general:items")} ?`}
                  onConfirm={() => this.performBulkDelete(this.state.selectedRows, this.state.selectedRowKeys)}
                  okText={i18next.t("general:OK")}
                  cancelText={i18next.t("general:Cancel")}
                >
                  <Button type="primary" danger size="small" icon={<DeleteOutlined />} style={{marginLeft: 8}}>
                    {i18next.t("general:Delete")} ({this.state.selectedRowKeys.length})
                  </Button>
                </Popconfirm>
              )}
            </div>
          )}
          loading={this.getTableLoading()}
          onChange={this.handleTableChange}
        />
      </div>
    );
  }

  fetch = (params = {}) => {
    this.setState({loading: true});
    PipeBackend.getPipes("admin")
      .then((res) => {
        const data = res.data || [];
        this.setState({
          loading: false,
          data,
          pagination: {
            ...params.pagination,
            total: data.length,
          },
          searchText: params.searchText,
          searchedColumn: params.searchedColumn,
        });
      })
      .catch(error => {
        if (error instanceof Setting.ApiError && error.isPermissionDenied()) {
          this.setState({isAuthorized: false, loading: false});
        } else {
          this.setState({loading: false});
          Setting.ResponseAdapter.showErrorMessage(error, i18next.t("general:Failed to get"));
        }
      });
  };
}

export default PipeListPage;

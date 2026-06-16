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

import React from "react";
import {Link} from "react-router-dom";
import {Button, Card, Popconfirm, Popover, Progress, Table, Tag, Tooltip} from "antd";
import moment from "moment";
import BaseListPage from "./BaseListPage";
import * as Setting from "./Setting";
import * as VectorBackend from "./backend/VectorBackend";
import * as FileBackend from "./backend/FileBackend";
import i18next from "i18next";
import {DeleteOutlined, EditOutlined, ReloadOutlined} from "@ant-design/icons";
import Editor from "./common/Editor";

class VectorListPage extends BaseListPage {
  constructor(props) {
    super(props);
    const fileFilter = new URLSearchParams(props.location?.search).get("file");
    if (fileFilter) {
      this.state = {
        ...this.state,
        fileFilter,
        searchText: fileFilter,
        searchedColumn: "file",
        fileInfo: null,
        loadingFileInfo: false,
      };
    }
  }

  UNSAFE_componentWillMount() {
    const {pagination} = this.state;
    if (this.state.fileFilter) {
      this.fetch({pagination, searchText: this.state.fileFilter, searchedColumn: "file"});
      this.loadFileInfo(this.state.fileFilter);
    } else {
      this.fetch({pagination});
    }
    this.getForm();
  }

  loadFileInfo(filePath) {
    const storeName = this.getApiStoreName();
    const owner = this.props.account?.name || "admin";
    this.setState({loadingFileInfo: true});
    FileBackend.getFiles(owner, storeName)
      .then((res) => {
        if (res.status === "ok" && res.data) {
          const matchedFile = res.data.find(f => {
            const name = f.name || "";
            const prefix = `${storeName}_`;
            const objectKey = name.startsWith(prefix) ? name.slice(prefix.length) : name;
            return objectKey === filePath || name === filePath;
          });
          this.setState({
            fileInfo: matchedFile || null,
            loadingFileInfo: false,
          });
        } else {
          this.setState({loadingFileInfo: false});
        }
      })
      .catch(() => {
        this.setState({loadingFileInfo: false});
      });
  }

  refreshFileVectors() {
    if (!this.state.fileInfo) {
      return;
    }
    FileBackend.refreshFileVectors(this.state.fileInfo)
      .then((res) => {
        if (res.status === "ok") {
          Setting.showMessage("success", i18next.t("general:Vectors generated successfully"));
          setTimeout(() => this.loadFileInfo(this.state.fileFilter), 1000);
        } else {
          Setting.showMessage("error", `${i18next.t("general:Vectors failed to generate")}: ${res.msg}`);
        }
      })
      .catch(error => {
        Setting.showMessage("error", `${i18next.t("general:Vectors failed to generate")}: ${error}`);
      });
  }

  newVector() {
    const randomName = Setting.getRandomName();
    let storeName = this.getApiStoreName();
    if (!storeName && Setting.isDefaultStoreSelected(this.props.account)) {
      storeName = "store-built-in";
    }
    return {
      owner: "admin",
      name: `vector_${randomName}`,
      createdTime: moment().format(),
      displayName: `New Vector - ${randomName}`,
      store: storeName,
      file: "/aaa/openagent.txt",
      text: "The text of vector",
      data: [0.1, 0.2, 0.3],
    };
  }

  addVector() {
    const newVector = this.newVector();
    VectorBackend.addVector(newVector)
      .then((res) => {
        if (res.status === "ok") {
          Setting.showMessage("success", i18next.t("general:Successfully added"));
          this.props.history.push({
            pathname: `/vectors/${newVector.name}`,
            state: {isNewVector: true},
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
    return VectorBackend.deleteVector(this.state.data[i]);
  };

  deleteVector(record) {
    VectorBackend.deleteVector(record)
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

  deleteAllVectors() {
    VectorBackend.deleteAllVectors()
      .then((res) => {
        if (res.status === "ok") {
          Setting.showMessage("success", i18next.t("general:Successfully deleted"));
          this.setState({
            data: [],
            pagination: {
              ...this.state.pagination,
              current: 1,
              total: 0,
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

  renderTable(vectors) {
    const columns = [
      {
        title: i18next.t("general:Store"),
        dataIndex: "store",
        key: "store",
        width: "130px",
        sorter: (a, b) => a.store.localeCompare(b.store),
        ...this.getColumnSearchProps("store"),
        render: (text, record, index) => {
          return (
            <Link to={`/stores/${record.owner}/${text}`}>
              {text}
            </Link>
          );
        },
      },
      {
        title: i18next.t("general:Name"),
        dataIndex: "name",
        key: "name",
        width: "140px",
        sorter: (a, b) => a.name.localeCompare(b.name),
        ...this.getColumnSearchProps("name"),
        render: (text, record, index) => {
          return (
            <Link to={`/vectors/${text}`}>
              {text}
            </Link>
          );
        },
      },
      // {
      //   title: i18next.t("general:Display name"),
      //   dataIndex: "displayName",
      //   key: "displayName",
      //   width: "200px",
      //   sorter: (a, b) => a.displayName.localeCompare(b.displayName),
      // },
      {
        title: i18next.t("general:Provider"),
        dataIndex: "provider",
        key: "provider",
        width: "200px",
        sorter: (a, b) => a.provider.localeCompare(b.provider),
        ...this.getColumnSearchProps("provider"),
        render: (text, record, index) => {
          return (
            <Link to={`/providers/${text}`}>
              {text}
            </Link>
          );
        },
      },
      {
        title: i18next.t("store:File"),
        dataIndex: "file",
        key: "file",
        width: "200px",
        sorter: (a, b) => a.file.localeCompare(b.file),
        ...this.getColumnSearchProps("file"),
      },
      {
        title: i18next.t("vector:Index"),
        dataIndex: "index",
        key: "index",
        width: "80px",
        sorter: (a, b) => a.index - b.index,
      },
      {
        title: i18next.t("general:Text"),
        dataIndex: "text",
        key: "text",
        width: "200px",
        sorter: (a, b) => a.text.localeCompare(b.text),
        ...this.getColumnSearchProps("text"),
        render: (text, record, index) => {
          return (
            <Popover placement="left" content={
              <Editor value={text} lang="text" dark readOnly height="300px" width="800px" />
            } title="" trigger="hover">
              <div style={{maxWidth: "200px"}}>
                {Setting.getShortText(text, 60)}
              </div>
            </Popover>
          );
        },
      },
      {
        title: i18next.t("general:Size"),
        dataIndex: "size",
        key: "size",
        width: "80px",
        sorter: (a, b) => a.size - b.size,
      },
      {
        title: i18next.t("general:Data"),
        dataIndex: "data",
        key: "data",
        width: "200px",
        sorter: (a, b) => a.data.localeCompare(b.data),
        render: (text, record, index) => {
          return (
            <Tooltip placement="left" title={Setting.getShortText(JSON.stringify(text), 1000)}>
              <div style={{maxWidth: "200px"}}>
                {Setting.getShortText(JSON.stringify(text), 50)}
              </div>
            </Tooltip>
          );
        },
      },
      {
        title: i18next.t("vector:Dimension"),
        dataIndex: "dimension",
        key: "dimension",
        width: "80px",
        sorter: (a, b) => a.dimension - b.dimension,
      },
      {
        title: i18next.t("general:Action"),
        dataIndex: "action",
        key: "action",
        width: "130px",
        fixed: "right",
        render: (text, record, index) => {
          return (
            <div style={{display: "flex", alignItems: "center", gap: "2px", flexWrap: "nowrap"}}>
              <Tooltip title={i18next.t("general:Edit")}>
                <Button type="text" size="small" icon={<EditOutlined />} style={{minWidth: "28px", width: "28px", height: "28px", padding: 0, borderRadius: "6px"}} onClick={() => this.props.history.push(`/vectors/${record.name}`)} />
              </Tooltip>
              <Popconfirm
                title={`${i18next.t("general:Sure to delete")}: ${record.name} ?`}
                onConfirm={() => this.deleteVector(record)}
                okText={i18next.t("general:OK")}
                cancelText={i18next.t("general:Cancel")}
              >
                <Tooltip title={i18next.t("general:Delete")}>
                  <Button type="text" size="small" danger icon={<DeleteOutlined />} style={{minWidth: "28px", width: "28px", height: "28px", padding: 0, borderRadius: "6px"}} />
                </Tooltip>
              </Popconfirm>
            </div>
          );
        },
      },
    ];
    const filteredColumns = Setting.filterTableColumns(columns, this.props.formItems ?? this.state.formItems);
    const paginationProps = {
      total: this.state.pagination.total,
      showQuickJumper: true,
      showSizeChanger: true,
      pageSizeOptions: ["10", "20", "50", "100", "1000", "10000", "100000"],
      showTotal: () => i18next.t("general:{total} in total").replace("{total}", this.state.pagination.total),
    };

    return (
      <div>
        <Table scroll={{x: "max-content"}} columns={filteredColumns} dataSource={vectors} rowKey="name" rowSelection={this.getRowSelection()} size="middle" bordered pagination={paginationProps}
          title={() => (
            <div>
              {i18next.t("general:Vectors")}
              {this.state.fileFilter ? <span style={{marginLeft: "8px", color: "#888", fontWeight: "normal", fontSize: "13px"}}>({this.state.fileFilter})</span> : null}
              &nbsp;&nbsp;&nbsp;&nbsp;
              <Button type="primary" size="small" onClick={this.addVector.bind(this)}>{i18next.t("general:Add")}</Button>
              {this.state.selectedRowKeys.length > 0 && (
                <Popconfirm title={`${i18next.t("general:Sure to delete")}: ${this.state.selectedRowKeys.length} ${i18next.t("general:items")} ?`} onConfirm={() => this.performBulkDelete(this.state.selectedRows, this.state.selectedRowKeys)} okText={i18next.t("general:OK")} cancelText={i18next.t("general:Cancel")}>
                  <Button type="primary" danger size="small" icon={<DeleteOutlined />} style={{marginLeft: 8}}>
                    {i18next.t("general:Delete")} ({this.state.selectedRowKeys.length})
                  </Button>
                </Popconfirm>
              )}
              <Popconfirm
                title={`${i18next.t("general:Sure to delete all")} ${i18next.t("general:Vectors")}`}
                onConfirm={() => this.deleteAllVectors()}
                okText={i18next.t("general:OK")}
                cancelText={i18next.t("general:Cancel")}
              >
                <Button style={{marginLeft: "10px"}} type="primary" size="small" danger>{i18next.t("general:Delete All")}</Button>
              </Popconfirm>
            </div>
          )}
          loading={this.getTableLoading()}
          onChange={this.handleTableChange}
        />
      </div>
    );
  }

  fetch = (params = {}) => {
    const field = "searchedColumn" in params ? params.searchedColumn : this.state.searchedColumn;
    const value = "searchText" in params ? params.searchText : this.state.searchText;
    const sortField = params.sortField, sortOrder = params.sortOrder;
    this.setState({loading: true});
    VectorBackend.getVectors("admin", this.getApiStoreName(), params.pagination.current, params.pagination.pageSize, field, value, sortField, sortOrder)
      .then((res) => {
        this.setState({
          loading: false,
        });
        if (res.status === "ok") {
          this.setState({
            data: res.data,
            pagination: {
              ...params.pagination,
              total: res.data2,
            },
            searchText: params.searchText,
            searchedColumn: params.searchedColumn,
          });
          if (this.state.fileFilter) {
            this.loadFileInfo(this.state.fileFilter);
          }
        } else {
          if (Setting.isResponseDenied(res)) {
            this.setState({
              isAuthorized: false,
            });
          } else {
            Setting.showMessage("error", res.msg);
          }
        }
      });
  };

  renderFileStatusCard() {
    if (!this.state.fileFilter) {
      return null;
    }
    const fileInfo = this.state.fileInfo;
    const detail = fileInfo?.stateDetail || {};
    const isProcessing = detail.state === "Parsing" || detail.state === "Vectorizing";

    return (
      <Card
        size="small"
        style={{marginBottom: "16px", borderRadius: "10px"}}
        title={
          <span>
            {i18next.t("store:File")}: <code style={{color: "var(--ant-color-text-secondary)"}}>{this.state.fileFilter}</code>
            &nbsp;&nbsp;
            <Tag color={detail.labelColor || "default"}>{detail.label || "-"}</Tag>
          </span>
        }
        extra={
          Setting.isLocalAdminUser(this.props.account) ? (
            <Tooltip title={detail.canRetry ? i18next.t("general:Refresh Vectors") : detail.label || ""}>
              <Button
                type="primary"
                size="small"
                icon={<ReloadOutlined />}
                disabled={!detail.canRetry}
                loading={isProcessing}
                onClick={() => this.refreshFileVectors()}
              >
                {i18next.t("general:Refresh Vectors")}
              </Button>
            </Tooltip>
          ) : null
        }
      >
        {isProcessing && detail.totalSections > 0 && (
          <Progress percent={Math.round((detail.progress / detail.totalSections) * 100)} size="small" />
        )}
        {detail.errorText && (
          <div style={{color: "var(--ant-color-error)", fontSize: "13px", marginTop: "8px"}}>
            {i18next.t("general:Error")}: {detail.errorText}
          </div>
        )}
        {detail.vectorError && !detail.errorText && (
          <div style={{color: "var(--ant-color-warning)", fontSize: "13px", marginTop: "8px"}}>
            {i18next.t("vector:Vector error")}: {detail.vectorError}
          </div>
        )}
        {fileInfo?.tokenCount > 0 && (
          <div style={{color: "var(--ant-color-text-secondary)", fontSize: "13px", marginTop: "8px"}}>
            {i18next.t("general:Tokens")}: {fileInfo.tokenCount}
          </div>
        )}
      </Card>
    );
  }

  render() {
    if (!this.state.isAuthorized) {
      return super.render();
    }
    if (this.state.loading && this.state.data === null) {
      return super.render();
    }
    return (
      <div>
        {this.renderFileStatusCard()}
        {this.renderTable(this.state.data)}
      </div>
    );
  }
}

export default VectorListPage;

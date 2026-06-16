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
import {Alert, Tag, Tooltip} from "antd";
import {CheckCircleOutlined, ClockCircleOutlined, CloseCircleOutlined, ReloadOutlined, WarningOutlined} from "@ant-design/icons";

const statusConfig = {
  Pass: {
    color: "success",
    text: "通过",
    icon: <CheckCircleOutlined />,
  },
  Warning: {
    color: "warning",
    text: "警告",
    icon: <WarningOutlined />,
  },
  Fail: {
    color: "error",
    text: "失败",
    icon: <CloseCircleOutlined />,
  },
  Pending: {
    color: "default",
    text: "待检测",
    icon: <ClockCircleOutlined />,
  },
  Stale: {
    color: "processing",
    text: "需刷新",
    icon: <ReloadOutlined />,
  },
};

const StatusTag = ({status, size = "default"}) => {
  const config = statusConfig[status] || statusConfig.Pending;
  return (
    <Tag color={config.color} icon={config.icon} style={{fontSize: size === "small" ? "12px" : "14px", margin: 0}}>
      {config.text}
    </Tag>
  );
};

const StatusBadge = ({status}) => {
  const config = statusConfig[status] || statusConfig.Pending;
  return (
    <span style={{display: "inline-flex", alignItems: "center", gap: "4px"}}>
      <span style={{color: config.color === "success" ? "#52c41a" : config.color === "warning" ? "#faad14" : config.color === "error" ? "#ff4d4f" : "#8c8c8c"}}>
        {config.icon}
      </span>
      <span>{config.text}</span>
    </span>
  );
};

const CapabilityIssues = ({issues, summary}) => {
  if (!issues || issues.length === 0) {
    return null;
  }

  const getType = (level) => {
    switch (level) {
    case "Fail": return "error";
    case "Warning": return "warning";
    default: return "info";
    }
  };

  const grouped = issues.reduce((acc, issue) => {
    const type = getType(issue.level);
    if (!acc[type]) {acc[type] = [];}
    acc[type].push(issue);
    return acc;
  }, {});

  return (
    <div style={{marginTop: "12px"}}>
      {summary && <div style={{marginBottom: "8px", fontWeight: 500}}>{summary}</div>}
      {Object.entries(grouped).map(([type, items]) => (
        <Alert
          key={type}
          type={type}
          showIcon
          message={items.length === 1 ? items[0].message : `${items.length} 个${type === "error" ? "错误" : "警告"}`}
          description={items.length > 1 ? (
            <ul style={{margin: 0, paddingLeft: "20px"}}>
              {items.map((item, idx) => (
                <li key={idx}>{item.message}</li>
              ))}
            </ul>
          ) : null}
          style={{marginBottom: "8px"}}
        />
      ))}
    </div>
  );
};

const CapabilityInfo = ({capability, showBadge = true, showIssues = true}) => {
  if (!capability) {return null;}

  return (
    <div>
      <div style={{display: "flex", alignItems: "center", gap: "8px", flexWrap: "wrap"}}>
        {showBadge && <StatusBadge status={capability.status} />}
        {capability.needReview && (
          <Tooltip title="需要用户重新检查配置">
            <Tag color="orange" style={{margin: 0}}>
              需检查
            </Tag>
          </Tooltip>
        )}
        {!capability.canMount && (
          <Tooltip title="不可挂载到 Agent">
            <Tag color="red" style={{margin: 0}}>
              不可挂载
            </Tag>
          </Tooltip>
        )}
        {capability.summary && !showIssues && (
          <span style={{color: "#666", fontSize: "13px"}}>{capability.summary}</span>
        )}
      </div>
      {showIssues && <CapabilityIssues issues={capability.issues} summary={capability.summary} />}
    </div>
  );
};

const getStatusColor = (status) => {
  const config = statusConfig[status] || statusConfig.Pending;
  return config.color;
};

const getStatusText = (status) => {
  const config = statusConfig[status] || statusConfig.Pending;
  return config.text;
};

const getStatusIcon = (status) => {
  const config = statusConfig[status] || statusConfig.Pending;
  return config.icon;
};

export {StatusTag, StatusBadge, CapabilityIssues, CapabilityInfo, getStatusColor, getStatusText, getStatusIcon, statusConfig};

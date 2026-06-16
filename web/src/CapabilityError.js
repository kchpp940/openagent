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

import React, {useState} from "react";
import {Alert, Button, Collapse, Space, Tag, Tooltip, Typography} from "antd";
import {
  CheckCircleOutlined,
  CloseCircleOutlined,
  DatabaseOutlined,
  ExclamationCircleOutlined,
  ReloadOutlined,
  SyncOutlined,
  WarningOutlined
} from "@ant-design/icons";
import * as ToolBackend from "./backend/ToolBackend";

const {Text} = Typography;

const ERR_CODE_CAPABILITY_CHECK_FAILED = "CAPABILITY_CHECK_FAILED";

export function extractCapabilityDecision(res) {
  if (!res) {return null;}
  if (res.decision) {return res.decision;}
  if (res.data && res.data.decision) {return res.data.decision;}
  if (res.code === ERR_CODE_CAPABILITY_CHECK_FAILED && res.data && res.data.decision) {
    return res.data.decision;
  }
  if (res.canMount !== undefined || res.canSaveStore !== undefined) {return res;}
  return null;
}

export function isCapabilityError(res) {
  return res && res.code === ERR_CODE_CAPABILITY_CHECK_FAILED;
}

const statusMap = {
  canMount: {
    true: {icon: <CheckCircleOutlined />, color: "#52c41a", text: "可挂载"},
    false: {icon: <CloseCircleOutlined />, color: "#ff4d4f", text: "不可挂载"},
  },
  canSaveStore: {
    true: {icon: <CheckCircleOutlined />, color: "#52c41a", text: "可保存"},
    false: {icon: <CloseCircleOutlined />, color: "#ff4d4f", text: "不可保存"},
  },
};

const CapabilityStatusTag = ({decision, type = "canMount"}) => {
  if (!decision) {return null;}
  const status = statusMap[type][decision[type]];
  return (
    <Tag icon={status.icon} color={decision[type] ? "success" : "error"} style={{margin: 0}}>
      {status.text}
    </Tag>
  );
};

const ErrorList = ({errors, type = "error"}) => {
  if (!errors || errors.length === 0) {return null;}

  const alertType = type === "error" ? "error" : "warning";
  const icon = type === "error" ? <CloseCircleOutlined /> : <WarningOutlined />;

  if (errors.length === 1) {
    return (
      <Alert
        type={alertType}
        showIcon
        icon={icon}
        message={errors[0].message}
        description={errors[0].field ? `字段: ${errors[0].field}` : null}
        style={{marginBottom: "8px"}}
      />
    );
  }

  return (
    <Alert
      type={alertType}
      showIcon
      icon={icon}
      message={`${errors.length} 个${type === "error" ? "错误" : "警告"}`}
      description={
        <ul style={{margin: "8px 0 0 0", paddingLeft: "20px"}}>
          {errors.map((err, idx) => (
            <li key={idx}>
              {err.message}
              {err.field && <Text type="secondary"> ({err.field})</Text>}
            </li>
          ))}
        </ul>
      }
      style={{marginBottom: "8px"}}
    />
  );
};

const FailedResources = ({resources}) => {
  if (!resources || resources.length === 0) {return null;}

  const items = resources.map((res, idx) => ({
    key: idx,
    label: (
      <Space>
        <DatabaseOutlined style={{color: "#ff4d4f"}} />
        <span>{res.name}</span>
        <Tag color="default">{res.entityType}</Tag>
      </Space>
    ),
    children: (
      <div style={{paddingLeft: "32px"}}>
        {res.errors && res.errors.map((err, errIdx) => (
          <div key={errIdx} style={{marginBottom: "4px"}}>
            <Text type="danger">• {err.message}</Text>
            {err.field && <Text type="secondary"> ({err.field})</Text>}
          </div>
        ))}
      </div>
    ),
  }));

  return (
    <Alert
      type="error"
      showIcon
      icon={<ExclamationCircleOutlined />}
      message={`${resources.length} 个依赖资源不可用`}
      description={
        <Collapse
          ghost
          items={items}
          defaultActiveKey={resources.length <= 3 ? resources.map((_, i) => i) : []}
          style={{marginTop: "8px"}}
        />
      }
      style={{marginBottom: "8px"}}
    />
  );
};

const CapabilityRecheckButton = ({decision, entityType, entityId, onRecheck}) => {
  const [loading, setLoading] = useState(false);

  if (!decision || !decision.needsRecheck) {return null;}

  const handleRecheck = async() => {
    setLoading(true);
    try {
      const resp = await ToolBackend.checkCapability(entityType, entityId);
      if (resp && resp.decision && onRecheck) {
        onRecheck(resp.decision);
      }
    } finally {
      setLoading(false);
    }
  };

  const actionText = decision.recheckAction === "sync" ? "同步" : "重新检查";

  return (
    <Tooltip title={decision.recheckAction === "sync" ? "点击同步 MCP 工具列表" : "点击重新校验配置"}>
      <Button
        type="primary"
        size="small"
        icon={<ReloadOutlined />}
        onClick={handleRecheck}
        loading={loading}
      >
        {actionText}
      </Button>
    </Tooltip>
  );
};

const CapabilityError = ({
  decision,
  entityType,
  entityId,
  onRecheck,
  showStatus = true,
  showActions = true,
  compact = false,
}) => {
  if (!decision) {return null;}

  const hasErrors = decision.errors && decision.errors.length > 0;
  const hasWarnings = decision.warnings && decision.warnings.length > 0;
  const hasFailedResources = decision.failedResources && decision.failedResources.length > 0;
  const hasIssues = hasErrors || hasWarnings || hasFailedResources;

  if (!hasIssues && !decision.blockReason && !decision.needsRecheck) {
    if (!showStatus) {return null;}
    return (
      <div style={compact ? {padding: "8px 0"} : {}}>
        <Space>
          <Tag icon={<CheckCircleOutlined />} color="success" style={{margin: 0}}>
            配置正常
          </Tag>
          {showStatus && (
            <>
              <CapabilityStatusTag decision={decision} type="canMount" />
              <CapabilityStatusTag decision={decision} type="canSaveStore" />
            </>
          )}
          {decision.lastCheckedAt && (
            <Text type="secondary" style={{fontSize: "12px"}}>
              上次检查: {new Date(decision.lastCheckedAt).toLocaleString()}
            </Text>
          )}
        </Space>
      </div>
    );
  }

  return (
    <div>
      <Space direction="vertical" size="middle" style={{width: "100%"}}>
        <Space wrap>
          {decision.blockReason && (
            <Alert
              type="error"
              showIcon
              icon={<ExclamationCircleOutlined />}
              message={decision.blockReason}
              style={{marginBottom: 0}}
            />
          )}
          {showStatus && (
            <Space size="small">
              <CapabilityStatusTag decision={decision} type="canMount" />
              <CapabilityStatusTag decision={decision} type="canSaveStore" />
            </Space>
          )}
          {decision.needsRecheck && showActions && (
            <CapabilityRecheckButton
              decision={decision}
              entityType={entityType}
              entityId={entityId}
              onRecheck={onRecheck}
            />
          )}
          {decision.lastCheckedAt && (
            <Text type="secondary" style={{fontSize: "12px"}}>
              上次检查: {new Date(decision.lastCheckedAt).toLocaleString()}
            </Text>
          )}
        </Space>

        {hasErrors && <ErrorList errors={decision.errors} type="error" />}
        {hasWarnings && <ErrorList errors={decision.warnings} type="warning" />}
        {hasFailedResources && <FailedResources resources={decision.failedResources} />}
      </Space>
    </div>
  );
};

const CapabilityStatusBadge = ({decision}) => {
  if (!decision) {return null;}

  const hasErrors = decision.errors && decision.errors.length > 0;
  const hasWarnings = decision.warnings && decision.warnings.length > 0;
  const hasFailedResources = decision.failedResources && decision.failedResources.length > 0;

  if (!decision.canMount || hasErrors || hasFailedResources) {
    return (
      <span style={{display: "inline-flex", alignItems: "center", gap: "4px", color: "#ff4d4f"}}>
        <CloseCircleOutlined />
        <span>错误</span>
      </span>
    );
  }

  if (hasWarnings || decision.needsRecheck) {
    return (
      <span style={{display: "inline-flex", alignItems: "center", gap: "4px", color: "#faad14"}}>
        <WarningOutlined />
        <span>警告</span>
      </span>
    );
  }

  return (
    <span style={{display: "inline-flex", alignItems: "center", gap: "4px", color: "#52c41a"}}>
      <CheckCircleOutlined />
      <span>正常</span>
    </span>
  );
};

const CapabilityListStatus = ({decision, compact = false}) => {
  if (!decision) {return null;}

  const hasErrors = decision.errors && decision.errors.length > 0;
  const hasWarnings = decision.warnings && decision.warnings.length > 0;
  const hasFailedResources = decision.failedResources && decision.failedResources.length > 0;

  let status = "normal";
  let icon = <CheckCircleOutlined />;
  let color = "#52c41a";

  if (!decision.canMount || hasErrors || hasFailedResources) {
    status = "error";
    icon = <CloseCircleOutlined />;
    color = "#ff4d4f";
  } else if (hasWarnings || decision.needsRecheck) {
    status = "warning";
    icon = <WarningOutlined />;
    color = "#faad14";
  }

  const statusText = {
    normal: "正常",
    warning: "警告",
    error: "错误",
  }[status];

  const summary = decision.blockReason || (hasErrors
    ? `${decision.errors.length} 个错误`
    : hasWarnings
      ? `${decision.warnings.length} 个警告`
      : hasFailedResources
        ? `${decision.failedResources.length} 个依赖问题`
        : "配置正常");

  return (
    <div style={{minWidth: compact ? "200px" : "260px"}}>
      <Space direction="vertical" size="small" style={{width: "100%"}}>
        <Space size="small">
          <span style={{color}}>{icon}</span>
          <span style={{fontWeight: 500}}>{statusText}</span>
          {decision.needsRecheck && (
            <Tag icon={<SyncOutlined spin />} color="processing" style={{margin: 0}}>
              需检查
            </Tag>
          )}
          {!decision.canMount && (
            <Tag color="error" style={{margin: 0}}>不可挂载</Tag>
          )}
        </Space>
        <Text type="secondary" style={{fontSize: "12px", display: "block"}}>
          {summary}
        </Text>
      </Space>
    </div>
  );
};

export {
  CapabilityError,
  CapabilityStatusTag,
  CapabilityStatusBadge,
  CapabilityListStatus,
  ErrorList,
  FailedResources,
  CapabilityRecheckButton
};

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
import {Tooltip, Empty} from "antd";
import {
  BulbOutlined,
  CheckCircleFilled,
  ClockCircleOutlined,
  CloseCircleFilled,
  DownOutlined,
  PlayCircleOutlined,
  SearchOutlined,
  SyncOutlined,
  ToolOutlined,
  WarningOutlined,
  InfoCircleOutlined,
} from "@ant-design/icons";
import i18next from "i18next";

const stepTypeConfig = {
  model_start: {icon: PlayCircleOutlined, color: "#1677ff", label: "Model"},
  model_end: {icon: CheckCircleFilled, color: "#52c41a", label: "Model"},
  reasoning: {icon: BulbOutlined, color: "#722ed1", label: "Reasoning"},
  knowledge_retrieval: {icon: SearchOutlined, color: "#13c2c2", label: "Knowledge"},
  tool_call_start: {icon: ToolOutlined, color: "#fa8c16", label: "Tool"},
  tool_call_end: {icon: ToolOutlined, color: "#52c41a", label: "Tool"},
  tool_error: {icon: WarningOutlined, color: "#ff4d4f", label: "Tool Error"},
  retry: {icon: SyncOutlined, color: "#faad14", label: "Retry"},
  final_output: {icon: CheckCircleFilled, color: "#52c41a", label: "Final"},
  info: {icon: InfoCircleOutlined, color: "#1677ff", label: "Info"},
  error: {icon: CloseCircleFilled, color: "#ff4d4f", label: "Error"},
};

const statusColorMap = {
  pending: "#d9d9d9",
  running: "#1677ff",
  completed: "#52c41a",
  failed: "#ff4d4f",
  skipped: "#8c8c8c",
};

const formatDuration = (ms) => {
  if (!ms) return "";
  if (ms < 1000) return `${ms}ms`;
  if (ms < 60000) return `${(ms / 1000).toFixed(1)}s`;
  const seconds = Math.floor(ms / 1000);
  const minutes = Math.floor(seconds / 60);
  const remainingSeconds = seconds % 60;
  return `${minutes}m ${remainingSeconds}s`;
};

const StepItem = ({step, isDark, themeColor, index}) => {
  const [expanded, setExpanded] = useState(false);
  const config = stepTypeConfig[step.type] || stepTypeConfig.info;
  const IconComponent = config.icon;
  const statusColor = statusColorMap[step.status] || statusColorMap.pending;

  const hasDetails = step.input || step.output || step.error || (step.metadata && Object.keys(step.metadata).length > 0);

  const labelColor = isDark ? "#565e78" : "#9aa3b8";
  const nameColor = isDark ? "#dde3f5" : "#1a2340";
  const bodyBg = isDark ? "#13151d" : "#f0f3fa";
  const border = isDark ? "1px solid #2a2e3d" : "1px solid #e6eaf4";
  const bodyBorder = isDark ? "1px solid #22263a" : "1px solid #eaeef8";

  const renderMetadata = (metadata) => {
    if (!metadata || Object.keys(metadata).length === 0) return null;
    return (
      <div style={{marginTop: "8px"}}>
        <div style={{fontSize: "11px", color: labelColor, marginBottom: "4px", fontWeight: 600}}>
          Metadata
        </div>
        <pre style={{
          fontSize: "11px",
          padding: "8px",
          background: bodyBg,
          borderRadius: "6px",
          margin: 0,
          overflowX: "auto",
          color: isDark ? "#a0a8c0" : "#4b5572",
          fontFamily: "monospace",
        }}>
          {JSON.stringify(metadata, null, 2)}
        </pre>
      </div>
    );
  };

  const renderTextBlock = (label, text) => {
    if (!text) return null;
    return (
      <div style={{marginTop: "8px"}}>
        <div style={{fontSize: "11px", color: labelColor, marginBottom: "4px", fontWeight: 600}}>
          {label}
        </div>
        <div style={{
          fontSize: "12px",
          padding: "8px",
          background: bodyBg,
          borderRadius: "6px",
          color: step.error ? "#ff4d4f" : (isDark ? "#a0a8c0" : "#4b5572"),
          wordBreak: "break-all",
          fontFamily: step.error ? "monospace" : "inherit",
          maxHeight: "200px",
          overflowY: "auto",
        }}>
          {text}
        </div>
      </div>
    );
  };

  return (
    <div style={{position: "relative", paddingLeft: "28px"}}>
      {/* Timeline dot */}
      <div style={{
        position: "absolute",
        left: 0,
        top: "10px",
        width: "16px",
        height: "16px",
        borderRadius: "50%",
        background: statusColor,
        display: "flex",
        alignItems: "center",
        justifyContent: "center",
        flexShrink: 0,
        zIndex: 1,
      }}>
        <IconComponent style={{fontSize: "9px", color: "#fff"}} />
      </div>

      {/* Timeline line */}
      {index < (step.totalCount || 0) - 1 && (
        <div style={{
          position: "absolute",
          left: "7px",
          top: "28px",
          bottom: "-8px",
          width: "2px",
          background: isDark ? "#2a2e3d" : "#e6eaf4",
        }} />
      )}

      {/* Step card */}
      <div style={{
        borderRadius: "8px",
        border,
        background: isDark ? "#191c26" : "#fff",
        marginBottom: "12px",
        overflow: "hidden",
      }}>
        {/* Header */}
        <div
          onClick={() => hasDetails && setExpanded(v => !v)}
          style={{
            display: "flex",
            alignItems: "center",
            gap: "8px",
            padding: "8px 12px",
            cursor: hasDetails ? "pointer" : "default",
            userSelect: "none",
          }}
        >
          <span style={{
            fontSize: "13px",
            fontWeight: 600,
            color: nameColor,
            flex: 1,
          }}>
            {step.title}
          </span>

          {step.durationMs > 0 && (
            <Tooltip title={`Duration: ${formatDuration(step.durationMs)}`}>
              <div style={{
                display: "flex",
                alignItems: "center",
                gap: "4px",
                fontSize: "11px",
                color: labelColor,
                flexShrink: 0,
              }}>
                <ClockCircleOutlined style={{fontSize: "10px"}} />
                <span>{formatDuration(step.durationMs)}</span>
              </div>
            </Tooltip>
          )}

          {step.round !== undefined && step.round !== null && (
            <div style={{
              fontSize: "10px",
              padding: "2px 6px",
              borderRadius: "10px",
              background: isDark ? "#22273a" : "#eff2fa",
              color: labelColor,
              flexShrink: 0,
              fontWeight: 500,
            }}>
              Round {step.round}
            </div>
          )}

          {hasDetails && (
            <DownOutlined style={{
              fontSize: "10px",
              color: labelColor,
              transform: expanded ? "rotate(180deg)" : "rotate(0deg)",
              transition: "transform 0.2s ease",
              flexShrink: 0,
            }} />
          )}
        </div>

        {/* Description */}
        {step.description && (
          <div style={{
            padding: "0 12px 8px",
            fontSize: "12px",
            color: isDark ? "#8b95b0" : "#6b7280",
          }}>
            {step.description}
          </div>
        )}

        {/* Body */}
        {expanded && hasDetails && (
          <div style={{
            borderTop: bodyBorder,
            padding: "10px 12px 12px",
          }}>
            {renderTextBlock("Input", step.input)}
            {renderTextBlock("Output", step.output)}
            {renderTextBlock("Error", step.error)}
            {renderMetadata(step.metadata)}
          </div>
        )}
      </div>
    </div>
  );
};

const ExecutionTimeline = ({steps, isDark, themeColor, loading}) => {
  const [expanded, setExpanded] = useState(false);

  if (!steps || steps.length === 0) {
    if (loading) {
      return (
        <div style={{marginBottom: "14px"}}>
          <div style={{
            display: "flex",
            alignItems: "center",
            gap: "5px",
            marginBottom: "8px",
            color: isDark ? "#565e78" : "#9aa3b8",
            fontSize: "11px",
            fontWeight: 600,
            textTransform: "uppercase",
            letterSpacing: "0.5px",
          }}>
            <ClockCircleOutlined style={{fontSize: "11px"}} />
            <span>Execution Trace</span>
          </div>
          <div style={{
            borderRadius: "10px",
            border: isDark ? "1px solid #2a2e3d" : "1px solid #e6eaf4",
            background: isDark ? "#191c26" : "#f7f9fd",
            padding: "20px",
            textAlign: "center",
          }}>
            <Empty description="Loading..." image={Empty.PRESENTED_IMAGE_SIMPLE} />
          </div>
        </div>
      );
    }
    return null;
  }

  const totalDuration = steps.reduce((sum, step) => sum + (step.durationMs || 0), 0);
  const stepCount = steps.length;
  const errorCount = steps.filter(s => s.status === "failed" || s.type === "error" || s.type === "tool_error").length;

  const labelColor = isDark ? "#565e78" : "#9aa3b8";
  const nameColor = isDark ? "#dde3f5" : "#1a2340";
  const border = isDark ? "1px solid #2a2e3d" : "1px solid #e6eaf4";
  const bg = isDark ? "#191c26" : "#f7f9fd";
  const bodyBorderTop = isDark ? "1px solid #22263a" : "1px solid #eaeef8";

  const stepsWithCount = steps.map((step, index) => ({...step, totalCount: stepCount}));

  return (
    <div style={{marginBottom: "14px"}}>
      {/* Section label */}
      <div style={{
        display: "flex",
        alignItems: "center",
        gap: "5px",
        marginBottom: "8px",
        color: labelColor,
        fontSize: "11px",
        fontWeight: 600,
        textTransform: "uppercase",
        letterSpacing: "0.5px",
      }}>
        <ClockCircleOutlined style={{fontSize: "11px"}} />
        <span>{i18next.t("chat:Execution trace")}</span>
      </div>

      {/* Card */}
      <div style={{
        borderRadius: "10px",
        border,
        background: bg,
        overflow: "hidden",
        transition: "box-shadow 0.15s",
      }}>
        {/* Header */}
        <div
          onClick={() => setExpanded(v => !v)}
          style={{
            display: "flex",
            alignItems: "center",
            gap: "10px",
            padding: "9px 13px",
            cursor: "pointer",
            userSelect: "none",
          }}
        >
          {/* Icon */}
          <div style={{
            width: "28px",
            height: "28px",
            borderRadius: "7px",
            background: `${themeColor}1a`,
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            flexShrink: 0,
          }}>
            <ClockCircleOutlined style={{color: themeColor, fontSize: "13px"}} />
          </div>

          {/* Title */}
          <span style={{
            fontSize: "13px",
            fontWeight: 600,
            color: nameColor,
            flex: 1,
          }}>
            {i18next.t("chat:Execution trace")}
          </span>

          {/* Stats */}
          <div style={{
            display: "flex",
            alignItems: "center",
            gap: "12px",
            flexShrink: 0,
          }}>
            <div style={{
              display: "flex",
              alignItems: "center",
              gap: "4px",
              fontSize: "11px",
              color: labelColor,
            }}>
              <span style={{fontWeight: 600, color: nameColor}}>{stepCount}</span>
              <span>steps</span>
            </div>

            {errorCount > 0 && (
              <div style={{
                display: "flex",
                alignItems: "center",
                gap: "4px",
                fontSize: "11px",
                color: "#ff4d4f",
              }}>
                <CloseCircleFilled style={{fontSize: "10px"}} />
                <span>{errorCount} errors</span>
              </div>
            )}

            {totalDuration > 0 && (
              <div style={{
                display: "flex",
                alignItems: "center",
                gap: "4px",
                fontSize: "11px",
                color: labelColor,
              }}>
                <ClockCircleOutlined style={{fontSize: "10px"}} />
                <span>{formatDuration(totalDuration)}</span>
              </div>
            )}
          </div>

          {/* Chevron */}
          <DownOutlined style={{
            fontSize: "10px",
            color: labelColor,
            transform: expanded ? "rotate(180deg)" : "rotate(0deg)",
            transition: "transform 0.2s ease",
            flexShrink: 0,
            marginLeft: "4px",
          }} />
        </div>

        {/* Body - Timeline */}
        {expanded && (
          <div style={{
            borderTop: bodyBorderTop,
            padding: "14px 13px 6px",
            maxHeight: "500px",
            overflowY: "auto",
          }}>
            {stepsWithCount.map((step, index) => (
              <StepItem
                key={step.id || index}
                step={step}
                isDark={isDark}
                themeColor={themeColor}
                index={index}
              />
            ))}
          </div>
        )}
      </div>
    </div>
  );
};

export default ExecutionTimeline;

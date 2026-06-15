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

import React, {useEffect, useMemo, useRef, useState} from "react";
import {Avatar, Badge, Button, Drawer, Dropdown, Input, List, Modal, Space, Table, Tag, Tooltip, Typography} from "antd";
import {
  CheckCircleOutlined,
  CommentOutlined,
  DeleteOutlined,
  DownloadOutlined,
  EditOutlined,
  ExclamationCircleOutlined,
  FullscreenOutlined,
  MessageOutlined,
  ReloadOutlined,
  SaveOutlined
} from "@ant-design/icons";
import i18next from "i18next";
import TaskAnalysisRadarChart from "./TaskAnalysisRadarChart";
import TaskAnalysisBarChart from "./TaskAnalysisBarChart";
import TaskAnalysisPieChart from "./TaskAnalysisPieChart";
import {downloadTaskAnalysisReportDocx} from "./taskReportDocx";
import {formatCategoryHeading} from "./taskResultFormat";
import {getScoreBandColor} from "./taskAnalysisScoreBands";
import * as Setting from "./Setting";
import * as TaskBackend from "./backend/TaskBackend";

const {TextArea} = Input;
const {Text} = Typography;

const taskChartCardStyle = {
  minWidth: 0,
  minHeight: 0,
  display: "flex",
  flexDirection: "column",
  border: "1px solid #ebeef2",
  borderRadius: "10px",
  background: "#fff",
  boxShadow: "0 1px 2px rgba(0, 0, 0, 0.04), 0 4px 16px rgba(0, 0, 0, 0.06)",
  padding: "12px 14px 10px",
  overflow: "hidden",
};

const taskChartCaptionStyle = {
  fontSize: "16px",
  fontWeight: 600,
  marginBottom: "10px",
  paddingBottom: "10px",
  borderBottom: "1px solid #f0f0f0",
  color: "rgba(0, 0, 0, 0.85)",
  lineHeight: 1.45,
  display: "flex",
  alignItems: "center",
  justifyContent: "center",
  position: "relative",
};

const STATUS_CONFIG = {
  open: {color: "blue", label: i18next.t("task:Open"), icon: <MessageOutlined />},
  resolved: {color: "green", label: i18next.t("task:Resolved"), icon: <CheckCircleOutlined />},
  disputed: {color: "orange", label: i18next.t("task:Disputed"), icon: <ExclamationCircleOutlined />},
};

function getCommentBadgeColor(comments) {
  if (!comments || comments.length === 0) {return "default";}
  const hasOpen = comments.some((c) => c.status === "open");
  const hasDisputed = comments.some((c) => c.status === "disputed");
  if (hasDisputed) {return "orange";}
  if (hasOpen) {return "blue";}
  return "green";
}

function countUnresolved(comments) {
  if (!comments) {return 0;}
  return comments.filter((c) => c.status !== "resolved").length;
}

export default function TaskAnalysisReport({result, downloadFileName, taskId, onCommentChange}) {
  const radarRef = useRef(null);
  const barRef = useRef(null);
  const pieRef = useRef(null);
  const lowScoreBarRef = useRef(null);
  const [downloading, setDownloading] = useState(false);
  const [fullscreenChart, setFullscreenChart] = useState(null);
  const [commentsDrawerOpen, setCommentsDrawerOpen] = useState(false);
  const [commentsLoading, setCommentsLoading] = useState(false);
  const [allComments, setAllComments] = useState([]);
  const [groupedComments, setGroupedComments] = useState({});
  const [backendAnchors, setBackendAnchors] = useState([]);
  const [commentCounts, setCommentCounts] = useState({total: 0, open: 0, resolved: 0, disputed: 0, unresolved: 0});
  const [activeTarget, setActiveTarget] = useState(null);
  const [newCommentText, setNewCommentText] = useState("");
  const [editingComment, setEditingComment] = useState(null);
  const [editingText, setEditingText] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const commentsLoadedRef = useRef(false);

  useEffect(() => {
    if (taskId && !commentsLoadedRef.current) {
      loadAllComments();
    }
  }, [taskId]);

  const loadAllComments = async() => {
    if (!taskId) {return;}
    setCommentsLoading(true);
    try {
      const res = await TaskBackend.getReportComments(taskId);
      if (res.status === "ok") {
        const data = res.data || {};
        const comments = data.comments || [];
        const grouped = data.grouped || {};
        const anchors = (data.anchors && data.anchors.anchors) ? data.anchors.anchors : [];
        const counts = data.counts || {total: comments.length, open: 0, resolved: 0, disputed: 0, unresolved: 0};
        setAllComments(comments);
        setGroupedComments(grouped);
        setBackendAnchors(anchors);
        setCommentCounts(counts);
        commentsLoadedRef.current = true;
        notifyParent(counts, comments);
      }
    } catch (err) {
      Setting.showMessage("error", err?.message || String(err));
    } finally {
      setCommentsLoading(false);
    }
  };

  const notifyParent = (counts, comments) => {
    if (onCommentChange) {
      onCommentChange({counts: counts || commentCounts, comments: comments || allComments});
    }
  };

  const anchorsByAnchorId = useMemo(() => {
    const m = {};
    (backendAnchors || []).forEach((a) => {
      if (a && a.anchorId) {
        m[a.anchorId] = a;
      }
    });
    return m;
  }, [backendAnchors]);

  const fieldAnchorsByPosition = useMemo(() => {
    const m = {};
    (backendAnchors || []).forEach((a) => {
      if (a && a.anchorType === "field" && typeof a.categoryIndex === "number" && typeof a.itemIndex === "number" && a.referencedField) {
        const k = `${a.categoryIndex}||${a.itemIndex}::${a.referencedField}`;
        m[k] = a;
      }
    });
    return m;
  }, [backendAnchors]);

  const itemAnchorsByPosition = useMemo(() => {
    const m = {};
    (backendAnchors || []).forEach((a) => {
      if (a && a.anchorType === "item" && typeof a.categoryIndex === "number" && typeof a.itemIndex === "number") {
        const k = `${a.categoryIndex}||${a.itemIndex}`;
        if (!m[k]) {m[k] = a;}
      }
    });
    (backendAnchors || []).forEach((a) => {
      if (a && a.itemAnchorId && typeof a.categoryIndex === "number" && typeof a.itemIndex === "number") {
        const k = `${a.categoryIndex}||${a.itemIndex}`;
        if (!m[k] && anchorsByAnchorId[a.itemAnchorId]) {
          m[k] = anchorsByAnchorId[a.itemAnchorId];
        }
      }
    });
    return m;
  }, [backendAnchors, anchorsByAnchorId]);

  const anchorsByItemAnchorId = useMemo(() => {
    const m = {};
    (backendAnchors || []).forEach((a) => {
      if (!a) {return;}
      const k = a.itemAnchorId;
      if (!k) {return;}
      if (!m[k]) {m[k] = [];}
      m[k].push(a);
    });
    return m;
  }, [backendAnchors]);

  if (!result) {
    return null;
  }
  const categories = result.categories || [];

  const metaItems = [
    {label: i18next.t("task:Unit Name"), value: result.title},
    {label: i18next.t("task:Designer"), value: result.designer},
    {label: i18next.t("video:Stage"), value: result.stage},
    {label: i18next.t("task:Participants"), value: result.participants},
    {label: i18next.t("store:Grade"), value: result.grade},
    {label: i18next.t("task:Instructor"), value: result.instructor},
    {label: i18next.t("store:Subject"), value: result.subject},
    {label: i18next.t("video:School"), value: result.school},
    {label: i18next.t("task:Other Subjects"), value: result.otherSubjects},
    {label: i18next.t("task:Textbook"), value: result.textbook},
  ];

  const getChartDataUrl = (ref) => {
    const comp = ref?.current;
    if (!comp || typeof comp.getEchartsInstance !== "function") {
      return null;
    }
    const inst = comp.getEchartsInstance();
    if (!inst) {
      return null;
    }
    const opts = {type: "png", pixelRatio: 2, backgroundColor: "#fff"};
    try {
      return inst.getDataURL(opts);
    } catch {
      return null;
    }
  };

  const handleDownloadReport = async() => {
    setDownloading(true);
    try {
      const chartImages = {};
      const r = getChartDataUrl(radarRef);
      if (r) {
        chartImages.radar = r;
      }
      const b = getChartDataUrl(barRef);
      if (b) {
        chartImages.bar = b;
      }
      const p = getChartDataUrl(pieRef);
      if (p) {
        chartImages.pie = p;
      }
      const lb = getChartDataUrl(lowScoreBarRef);
      if (lb) {
        chartImages.lowScoreBar = lb;
      }
      await downloadTaskAnalysisReportDocx(result, {
        fileName: downloadFileName || "task_report.docx",
        chartImages,
      });
      Setting.showMessage("success", i18next.t("general:Successfully downloaded"));
    } catch (err) {
      Setting.showMessage("error", err?.message || String(err));
    } finally {
      setDownloading(false);
    }
  };

  const buildTargetFromAnchor = (anchor) => {
    if (!anchor) {
      return null;
    }
    return {
      anchorId: anchor.anchorId,
      itemAnchorId: anchor.itemAnchorId,
      anchorCategory: anchor.anchorCategory,
      anchorItem: anchor.anchorItem,
      anchorField: anchor.anchorField,
      contentHash: anchor.contentHash,
      stableCategoryKey: anchor.stableCategoryKey,
      stableItemKey: anchor.stableItemKey,
      categoryName: anchor.categoryName,
      itemName: anchor.itemName,
      referencedField: anchor.referencedField,
      referencedContent: anchor.referencedContent,
      categoryIndex: anchor.categoryIndex,
      itemIndex: anchor.itemIndex,
    };
  };

  const openCommentsDrawer = (target) => {
    if (!target) {return;}
    setActiveTarget(target);
    setNewCommentText("");
    setEditingComment(null);
    setEditingText("");
    setCommentsDrawerOpen(true);
  };

  const getItemComments = (itemAnchorId, stableItemKeyFallback) => {
    if (!itemAnchorId && !stableItemKeyFallback) {return [];}
    if (itemAnchorId && groupedComments[itemAnchorId]) {
      return groupedComments[itemAnchorId];
    }
    if (stableItemKeyFallback && groupedComments[stableItemKeyFallback]) {
      return groupedComments[stableItemKeyFallback];
    }
    if (itemAnchorId) {
      return allComments.filter((c) => c.itemAnchorId === itemAnchorId);
    }
    return [];
  };

  const getTargetComments = () => {
    if (!activeTarget) {return [];}
    if (activeTarget.itemAnchorId && groupedComments[activeTarget.itemAnchorId]) {
      return groupedComments[activeTarget.itemAnchorId];
    }
    if (activeTarget.stableItemKey && groupedComments[activeTarget.stableItemKey]) {
      return groupedComments[activeTarget.stableItemKey];
    }
    return allComments.filter((c) => {
      if (activeTarget.anchorId && c.anchorId === activeTarget.anchorId) {return true;}
      if (activeTarget.itemAnchorId && c.itemAnchorId === activeTarget.itemAnchorId) {return true;}
      if (activeTarget.stableItemKey && c.stableItemKey === activeTarget.stableItemKey) {return true;}
      return false;
    });
  };

  const handleAddComment = async() => {
    if (!newCommentText.trim() || !activeTarget || !activeTarget.anchorId || !taskId) {return;}
    setSubmitting(true);
    try {
      const [taskOwner, ...rest] = taskId.split("/");
      const taskName = rest.join("/");
      const res = await TaskBackend.addReportComment(taskOwner, taskName, activeTarget.anchorId, newCommentText.trim());
      if (res.status === "ok") {
        Setting.showMessage("success", i18next.t("general:Successfully saved"));
        setNewCommentText("");
        commentsLoadedRef.current = false;
        await loadAllComments();
      } else {
        Setting.showMessage("error", res.msg || i18next.t("general:Failed to save"));
      }
    } catch (err) {
      Setting.showMessage("error", err?.message || String(err));
    } finally {
      setSubmitting(false);
    }
  };

  const startEditComment = (comment) => {
    setEditingComment(comment);
    setEditingText(comment.content);
  };

  const cancelEditComment = () => {
    setEditingComment(null);
    setEditingText("");
  };

  const saveEditComment = async() => {
    if (!editingComment || !editingText.trim()) {return;}
    setSubmitting(true);
    try {
      const payload = {
        content: editingText.trim(),
        referencedContent: editingComment.referencedContent || "",
      };
      const res = await TaskBackend.updateReportComment(editingComment.id, payload);
      if (res.status === "ok") {
        Setting.showMessage("success", i18next.t("general:Successfully saved"));
        cancelEditComment();
        commentsLoadedRef.current = false;
        await loadAllComments();
      } else {
        Setting.showMessage("error", res.msg || i18next.t("general:Failed to save"));
      }
    } catch (err) {
      Setting.showMessage("error", err?.message || String(err));
    } finally {
      setSubmitting(false);
    }
  };

  const handleStatusChange = async(commentId, action) => {
    setSubmitting(true);
    try {
      let res;
      if (action === "resolve") {
        res = await TaskBackend.resolveReportComment(commentId);
      } else if (action === "reopen") {
        res = await TaskBackend.reopenReportComment(commentId);
      } else if (action === "dispute") {
        res = await TaskBackend.disputeReportComment(commentId);
      }
      if (res && res.status === "ok") {
        Setting.showMessage("success", i18next.t("general:Successfully saved"));
        commentsLoadedRef.current = false;
        await loadAllComments();
      } else if (res) {
        Setting.showMessage("error", res.msg || i18next.t("general:Failed to save"));
      }
    } catch (err) {
      Setting.showMessage("error", err?.message || String(err));
    } finally {
      setSubmitting(false);
    }
  };

  const handleDeleteComment = (commentId) => {
    Modal.confirm({
      title: i18next.t("general:Confirm"),
      content: i18next.t("task:Are you sure you want to delete this comment?"),
      okText: i18next.t("general:Delete"),
      okType: "danger",
      cancelText: i18next.t("general:Cancel"),
      onOk: async() => {
        try {
          const res = await TaskBackend.deleteReportComment(commentId);
          if (res.status === "ok") {
            Setting.showMessage("success", i18next.t("general:Successfully deleted"));
            commentsLoadedRef.current = false;
            await loadAllComments();
          } else {
            Setting.showMessage("error", res.msg || i18next.t("general:Failed to delete"));
          }
        } catch (err) {
          Setting.showMessage("error", err?.message || String(err));
        }
      },
    });
  };

  const renderCommentActions = (comment) => {
    if (editingComment?.id === comment.id) {
      return (
        <Space>
          <Button type="primary" size="small" icon={<SaveOutlined />} loading={submitting} onClick={saveEditComment}>
            {i18next.t("general:Save")}
          </Button>
          <Button size="small" onClick={cancelEditComment}>
            {i18next.t("general:Cancel")}
          </Button>
        </Space>
      );
    }
    const actions = [];
    actions.push({
      key: "edit",
      label: i18next.t("general:Edit"),
      icon: <EditOutlined />,
      onClick: () => startEditComment(comment),
    });
    actions.push({
      key: "delete",
      label: i18next.t("general:Delete"),
      icon: <DeleteOutlined />,
      danger: true,
      onClick: () => handleDeleteComment(comment.id),
    });
    if (comment.status === "open") {
      actions.push({
        key: "resolve",
        label: i18next.t("task:Resolve"),
        icon: <CheckCircleOutlined />,
        onClick: () => handleStatusChange(comment.id, "resolve"),
      });
      actions.push({
        key: "dispute",
        label: i18next.t("task:Mark as disputed"),
        icon: <ExclamationCircleOutlined />,
        onClick: () => handleStatusChange(comment.id, "dispute"),
      });
    } else if (comment.status === "resolved") {
      actions.push({
        key: "reopen",
        label: i18next.t("task:Reopen"),
        icon: <ReloadOutlined />,
        onClick: () => handleStatusChange(comment.id, "reopen"),
      });
    } else if (comment.status === "disputed") {
      actions.push({
        key: "resolve",
        label: i18next.t("task:Resolve"),
        icon: <CheckCircleOutlined />,
        onClick: () => handleStatusChange(comment.id, "resolve"),
      });
      actions.push({
        key: "reopen",
        label: i18next.t("task:Reopen as open"),
        icon: <ReloadOutlined />,
        onClick: () => handleStatusChange(comment.id, "reopen"),
      });
    }
    return (
      <Dropdown
        menu={{items: actions}}
        trigger={["click"]}
        disabled={submitting}
      >
        <Button type="text" size="small">
          {i18next.t("general:Actions")}
        </Button>
      </Dropdown>
    );
  };

  const radarAxis = (() => {
    if (categories.length === 0) {
      return {min: 0, max: 5};
    }
    const scores = [];
    (categories || []).forEach((c) => {
      (c.items || []).forEach((item) => {
        scores.push(Number(item.score) || 0);
      });
    });
    if (scores.length === 0) {
      (categories || []).forEach((c) => {
        scores.push(Number(c.score) || 0);
      });
    }
    if (scores.length === 0) {
      return {min: 0, max: 5};
    }
    const maxScore = Math.max(...scores);
    if (maxScore <= 5) {
      return {min: 0, max: 5};
    }
    if (maxScore <= 10) {
      return {min: 0, max: 10};
    }
    return {min: 50, max: 100};
  })();

  const hasCharts = categories.length > 0;
  const totalUnresolved = Number(commentCounts.unresolved) || 0;

  const CommentBadgeButton = ({target, itemComments, size = "small"}) => {
    if (!target) {return null;}
    const unresolved = countUnresolved(itemComments);
    const badgeColor = getCommentBadgeColor(itemComments);
    return (
      <Tooltip title={`${itemComments.length} ${i18next.t("task:comments")}, ${unresolved} ${i18next.t("task:unresolved")}`}>
        <Badge
          count={itemComments.length > 0 ? itemComments.length : 0}
          color={itemComments.length > 0 ? (unresolved > 0 ? (badgeColor === "orange" ? "#fa8c16" : "#1677ff") : "#52c41a") : undefined}
          showZero={false}
          size="small"
        >
          <Button
            type="text"
            size={size}
            icon={<CommentOutlined style={{color: unresolved > 0 ? (badgeColor === "orange" ? "#fa8c16" : "#1677ff") : (itemComments.length > 0 ? "#52c41a" : "#8c8c8c")}} />}
            onClick={() => openCommentsDrawer(target)}
            style={{padding: "0 4px", marginLeft: "4px"}}
          />
        </Badge>
      </Tooltip>
    );
  };

  const reportColumns = (cat, catIdx) => [
    {
      title: (
        <div style={{display: "flex", alignItems: "center", justifyContent: "space-between"}}>
          <span>{i18next.t("task:Sub-criteria")}</span>
        </div>
      ),
      dataIndex: "name",
      key: "name",
      width: "12%",
      render: (text, record, idx) => {
        const posKey = `${catIdx}||${idx}`;
        const itemAnchor = itemAnchorsByPosition[posKey];
        const target = buildTargetFromAnchor(itemAnchor);
        const itemComments = itemAnchor ? getItemComments(itemAnchor.itemAnchorId || itemAnchor.anchorId, itemAnchor.stableItemKey) : [];
        return (
          <div style={{display: "flex", alignItems: "center", gap: "4px"}}>
            <span>{text}</span>
            {target && <CommentBadgeButton target={target} itemComments={itemComments} />}
          </div>
        );
      },
    },
    {
      title: i18next.t("task:Score"),
      dataIndex: "score",
      key: "score",
      width: "8%",
      render: (score, record, idx) => {
        const text = `${score}${i18next.t("task:Score Unit")}`;
        const hex = getScoreBandColor(score, categories);
        const posKey = `${catIdx}||${idx}`;
        const itemAnchor = itemAnchorsByPosition[posKey];
        const itemComments = itemAnchor ? getItemComments(itemAnchor.itemAnchorId || itemAnchor.anchorId, itemAnchor.stableItemKey) : [];
        const badgeColor = getCommentBadgeColor(itemComments);
        const unresolved = countUnresolved(itemComments);
        return (
          <div style={{display: "flex", alignItems: "center", gap: "4px"}}>
            <Tag color={hex} style={{margin: 0}}>
              {text}
            </Tag>
            {itemComments.length > 0 && (
              <Badge
                dot
                color={unresolved > 0 ? (badgeColor === "orange" ? "#fa8c16" : "#1677ff") : "#52c41a"}
                offset={[-4, 4]}
              >
                <span />
              </Badge>
            )}
          </div>
        );
      },
    },
    {
      title: (
        <div style={{display: "flex", alignItems: "center"}}>
          <span>{i18next.t("task:Advantages")}</span>
        </div>
      ),
      dataIndex: "advantage",
      key: "advantage",
      width: "27%",
      render: (text, record, idx) => {
        const posKey = `${catIdx}||${idx}::advantage`;
        const fieldAnchor = fieldAnchorsByPosition[posKey];
        const target = buildTargetFromAnchor(fieldAnchor);
        return (
          <div style={{position: "relative"}}>
            {text}
            {target && (
              <div style={{position: "absolute", top: 0, right: 0}}>
                <Tooltip title={i18next.t("task:Comment on this field")}>
                  <Button
                    type="text"
                    size="small"
                    icon={<MessageOutlined style={{fontSize: "12px", color: "#bfbfbf"}} />}
                    onClick={() => openCommentsDrawer(target)}
                    style={{padding: "0 2px", minWidth: "auto", height: "16px"}}
                  />
                </Tooltip>
              </div>
            )}
          </div>
        );
      },
    },
    {
      title: i18next.t("task:Disadvantages"),
      dataIndex: "disadvantage",
      key: "disadvantage",
      width: "27%",
      render: (text, record, idx) => {
        const posKey = `${catIdx}||${idx}::disadvantage`;
        const fieldAnchor = fieldAnchorsByPosition[posKey];
        const target = buildTargetFromAnchor(fieldAnchor);
        return (
          <div style={{position: "relative"}}>
            {text}
            {target && (
              <div style={{position: "absolute", top: 0, right: 0}}>
                <Tooltip title={i18next.t("task:Comment on this field")}>
                  <Button
                    type="text"
                    size="small"
                    icon={<MessageOutlined style={{fontSize: "12px", color: "#bfbfbf"}} />}
                    onClick={() => openCommentsDrawer(target)}
                    style={{padding: "0 2px", minWidth: "auto", height: "16px"}}
                  />
                </Tooltip>
              </div>
            )}
          </div>
        );
      },
    },
    {
      title: i18next.t("task:Suggestion"),
      dataIndex: "suggestion",
      key: "suggestion",
      width: "26%",
      render: (text, record, idx) => {
        const posKey = `${catIdx}||${idx}::suggestion`;
        const fieldAnchor = fieldAnchorsByPosition[posKey];
        const target = buildTargetFromAnchor(fieldAnchor);
        return (
          <div style={{position: "relative"}}>
            {text}
            {target && (
              <div style={{position: "absolute", top: 0, right: 0}}>
                <Tooltip title={i18next.t("task:Comment on this field")}>
                  <Button
                    type="text"
                    size="small"
                    icon={<MessageOutlined style={{fontSize: "12px", color: "#bfbfbf"}} />}
                    onClick={() => openCommentsDrawer(target)}
                    style={{padding: "0 2px", minWidth: "auto", height: "16px"}}
                  />
                </Tooltip>
              </div>
            )}
          </div>
        );
      },
    },
  ];

  const renderDrawerTitle = () => {
    if (!activeTarget) {return i18next.t("task:Comments");}
    return (
      <div>
        <div style={{fontWeight: 600}}>
          {activeTarget.categoryName} → {activeTarget.itemName}
        </div>
        {activeTarget.referencedField && (
          <div style={{fontSize: "12px", color: "#8c8c8c", marginTop: "4px"}}>
            {i18next.t(`task:${activeTarget.referencedField.charAt(0).toUpperCase() + activeTarget.referencedField.slice(1)}`)}
            {activeTarget.referencedField === "score" && activeTarget.referencedContent ? `: ${activeTarget.referencedContent}` : ""}
          </div>
        )}
      </div>
    );
  };

  return (
    <div style={{marginTop: "16px"}}>
      <div style={{display: "grid", gridTemplateColumns: "1fr 1fr", gap: "8px 24px", marginBottom: "16px", padding: "12px", border: "1px solid #f0f0f0", borderRadius: "6px", background: "#fafafa"}}>
        {metaItems.map((item, idx) => (
          <div key={idx} style={{display: "flex", gap: "8px"}}>
            <span style={{fontWeight: 600, whiteSpace: "nowrap"}}>{item.label}：</span>
            <span>{item.value || "-"}</span>
          </div>
        ))}
      </div>
      <div style={{marginBottom: "12px", display: "flex", alignItems: "center", gap: "16px", flexWrap: "wrap"}}>
        <div style={{fontSize: "16px", fontWeight: 600}}>
          {i18next.t("task:Overall Score")}：<span style={{color: "#1677ff", fontSize: "20px"}}>{result.score}</span>
        </div>
        {taskId && totalUnresolved > 0 && (
          <Tooltip title={`${totalUnresolved} ${i18next.t("task:unresolved comments")} (${Number(commentCounts.open) || 0} ${i18next.t("task:Open")}, ${Number(commentCounts.disputed) || 0} ${i18next.t("task:Disputed")})`}>
            <Badge count={totalUnresolved} color="#1677ff">
              <Button type="text" icon={<CommentOutlined />} onClick={() => {
                const firstGroupKey = Object.keys(groupedComments || {}).find((k) => {
                  const arr = groupedComments[k] || [];
                  return arr.some((c) => c.status !== "resolved");
                });
                if (firstGroupKey) {
                  const arr = groupedComments[firstGroupKey] || [];
                  const first = arr.find((c) => c.status !== "resolved") || arr[0];
                  const matched = anchorsByAnchorId[first.anchorId] || (first.itemAnchorId && anchorsByAnchorId[first.itemAnchorId]) || null;
                  const target = matched
                    ? buildTargetFromAnchor(matched)
                    : {
                      anchorId: first.anchorId,
                      itemAnchorId: first.itemAnchorId,
                      categoryName: first.categoryName,
                      itemName: first.itemName,
                      referencedField: first.referencedField,
                      referencedContent: first.referencedContent,
                    };
                  setActiveTarget(target);
                  setCommentsDrawerOpen(true);
                }
              }}>
                {i18next.t("task:View comments")}
              </Button>
            </Badge>
          </Tooltip>
        )}
        <Button type="primary" icon={<DownloadOutlined />} loading={downloading} onClick={handleDownloadReport}>
          {i18next.t("task:Download report")}
        </Button>
      </div>
      {hasCharts && (
        <div
          style={{
            marginBottom: "24px",
            display: "grid",
            gridTemplateColumns: "1fr 1fr",
            gridTemplateRows: "repeat(2, minmax(400px, auto))",
            gap: "24px",
          }}
        >
          <div style={taskChartCardStyle}>
            <div style={taskChartCaptionStyle}>
              {i18next.t("task:Chart caption radar")}
              <Button type="text" size="small" icon={<FullscreenOutlined />} style={{position: "absolute", right: 0, top: "50%", transform: "translateY(-50%)", color: "#8c8c8c"}} onClick={() => setFullscreenChart("radar")} />
            </div>
            <div style={{flex: 1, minHeight: 0}}>
              <TaskAnalysisRadarChart categories={categories} radarMin={radarAxis.min} radarMax={radarAxis.max} chartRef={radarRef} />
            </div>
          </div>
          <div style={taskChartCardStyle}>
            <div style={taskChartCaptionStyle}>
              {i18next.t("task:Chart caption bar ranked")}
              <Button type="text" size="small" icon={<FullscreenOutlined />} style={{position: "absolute", right: 0, top: "50%", transform: "translateY(-50%)", color: "#8c8c8c"}} onClick={() => setFullscreenChart("bar")} />
            </div>
            <div style={{flex: 1, minHeight: 0}}>
              <TaskAnalysisBarChart categories={categories} chartRef={barRef} />
            </div>
          </div>
          <div style={taskChartCardStyle}>
            <div style={taskChartCaptionStyle}>
              {i18next.t("task:Chart caption pie")}
              <Button type="text" size="small" icon={<FullscreenOutlined />} style={{position: "absolute", right: 0, top: "50%", transform: "translateY(-50%)", color: "#8c8c8c"}} onClick={() => setFullscreenChart("pie")} />
            </div>
            <div style={{flex: 1, minHeight: 0}}>
              <TaskAnalysisPieChart categories={categories} chartRef={pieRef} />
            </div>
          </div>
          <div style={taskChartCardStyle}>
            <div style={taskChartCaptionStyle}>
              {i18next.t("task:Chart caption priority dimensions")}
              <Button type="text" size="small" icon={<FullscreenOutlined />} style={{position: "absolute", right: 0, top: "50%", transform: "translateY(-50%)", color: "#8c8c8c"}} onClick={() => setFullscreenChart("lowscore")} />
            </div>
            <div style={{flex: 1, minHeight: 0}}>
              <TaskAnalysisBarChart categories={categories} chartRef={lowScoreBarRef} orientation="vertical" maxScoreExclusive={70} />
            </div>
          </div>
        </div>
      )}
      <Modal
        open={fullscreenChart !== null}
        footer={null}
        onCancel={() => setFullscreenChart(null)}
        width="95vw"
        style={{top: 20, maxWidth: "95vw"}}
        styles={{body: {height: "80vh", padding: "16px", display: "flex", flexDirection: "column"}}}
        title={
          fullscreenChart === "radar" ? i18next.t("task:Chart caption radar") :
            fullscreenChart === "bar" ? i18next.t("task:Chart caption bar ranked") :
              fullscreenChart === "pie" ? i18next.t("task:Chart caption pie") :
                fullscreenChart === "lowscore" ? i18next.t("task:Chart caption priority dimensions") : ""
        }
      >
        <div style={{flex: 1, minHeight: 0}}>
          {fullscreenChart === "radar" && <TaskAnalysisRadarChart categories={categories} radarMin={radarAxis.min} radarMax={radarAxis.max} />}
          {fullscreenChart === "bar" && <TaskAnalysisBarChart categories={categories} />}
          {fullscreenChart === "pie" && <TaskAnalysisPieChart categories={categories} />}
          {fullscreenChart === "lowscore" && <TaskAnalysisBarChart categories={categories} orientation="vertical" maxScoreExclusive={70} />}
        </div>
      </Modal>
      {categories.map((cat, idx) => {
        const itemAnchorIdsForCat = Object.keys(anchorsByItemAnchorId).filter((k) => {
          const arr = anchorsByItemAnchorId[k] || [];
          return arr.some((a) => a.categoryIndex === idx);
        });
        const catComments = [];
        itemAnchorIdsForCat.forEach((k) => {
          const arr = groupedComments[k] || [];
          arr.forEach((c) => catComments.push(c));
        });
        if (groupedComments["__orphan__"]) {
          groupedComments["__orphan__"].forEach((c) => {
            if (c.categoryName === cat.name) {catComments.push(c);}
          });
        }
        const catUnresolved = countUnresolved(catComments);
        return (
          <div key={idx} style={{marginBottom: "24px"}}>
            <div style={{fontWeight: 600, marginBottom: "8px", fontSize: "14px", display: "flex", alignItems: "center", gap: "8px"}}>
              <span>{formatCategoryHeading(cat, idx)}（{i18next.t("task:Score")}：{cat.score}{i18next.t("task:Score Unit")}）</span>
              {catComments.length > 0 && (
                <Tooltip title={`${catComments.length} ${i18next.t("task:comments")}, ${catUnresolved} ${i18next.t("task:unresolved")}`}>
                  <Badge count={catComments.length} size="small" color={catUnresolved > 0 ? "#1677ff" : "#52c41a"} />
                </Tooltip>
              )}
            </div>
            <Table
              size="small"
              bordered
              pagination={false}
              columns={reportColumns(cat, idx)}
              dataSource={(cat.items || []).map((item, i) => ({...item, key: i}))}
            />
          </div>
        );
      })}

      <Drawer
        title={renderDrawerTitle()}
        placement="right"
        width={480}
        open={commentsDrawerOpen}
        onClose={() => setCommentsDrawerOpen(false)}
        extra={
          <Space>
            <Button size="small" icon={<ReloadOutlined />} onClick={() => {commentsLoadedRef.current = false; loadAllComments();}}>
              {i18next.t("general:Refresh")}
            </Button>
          </Space>
        }
      >
        {activeTarget?.referencedContent && activeTarget.referencedField !== "score" && (
          <div style={{
            padding: "10px 12px",
            background: "#f9f9f9",
            border: "1px solid #f0f0f0",
            borderRadius: "6px",
            marginBottom: "16px",
            fontSize: "13px",
            color: "#595959",
          }}>
            <Text type="secondary" style={{fontSize: "12px"}}>
              {i18next.t("task:Original content")}:
            </Text>
            <div style={{marginTop: "6px", whiteSpace: "pre-wrap", wordBreak: "break-word"}}>
              {activeTarget.referencedContent}
            </div>
          </div>
        )}

        <div style={{marginBottom: "16px"}}>
          <TextArea
            rows={4}
            placeholder={i18next.t("task:Add a comment, suggestion or correction...")}
            value={newCommentText}
            onChange={(e) => setNewCommentText(e.target.value)}
            disabled={!taskId || submitting || !activeTarget || !activeTarget.anchorId}
          />
          {!activeTarget?.anchorId && taskId && (
            <div style={{fontSize: "12px", color: "#fa8c16", marginTop: "6px"}}>
              {i18next.t("task:This item has no server anchor; comments cannot be added until the task analysis is regenerated.")}
            </div>
          )}
          <div style={{marginTop: "8px", textAlign: "right"}}>
            <Button
              type="primary"
              icon={<CommentOutlined />}
              loading={submitting}
              disabled={!newCommentText.trim() || !taskId || !activeTarget?.anchorId}
              onClick={handleAddComment}
            >
              {i18next.t("task:Add comment")}
            </Button>
          </div>
        </div>

        <List
          loading={commentsLoading}
          dataSource={getTargetComments().sort((a, b) => new Date(a.createdTime) - new Date(b.createdTime))}
          locale={{emptyText: i18next.t("task:No comments yet. Be the first to comment!")}}
          renderItem={(comment) => {
            const cfg = STATUS_CONFIG[comment.status] || STATUS_CONFIG.open;
            const isEditing = editingComment?.id === comment.id;
            return (
              <List.Item
                key={comment.id}
                style={{
                  alignItems: "flex-start",
                  padding: "12px 0",
                  borderBottom: "1px solid #f0f0f0",
                }}
              >
                <List.Item.Meta
                  avatar={<Avatar style={{backgroundColor: "#1677ff", verticalAlign: "middle"}} size="small">
                    {(comment.author || "U").charAt(0).toUpperCase()}
                  </Avatar>}
                  title={
                    <div style={{display: "flex", alignItems: "center", justifyContent: "space-between", gap: "8px"}}>
                      <Space size="small">
                        <Text strong>{comment.author}</Text>
                        <Tag color={cfg.color} icon={cfg.icon} style={{margin: 0}}>
                          {cfg.label}
                        </Tag>
                      </Space>
                      {renderCommentActions(comment)}
                    </div>
                  }
                  description={
                    <div style={{fontSize: "12px", color: "#8c8c8c", marginBottom: "4px"}}>
                      {comment.createdTime}
                      {comment.updatedTime && comment.updatedTime !== comment.createdTime && (
                        <span style={{marginLeft: "8px"}}>
                          ({i18next.t("task:Updated")}: {comment.updatedTime})
                        </span>
                      )}
                      {comment.status === "resolved" && comment.resolver && (
                        <span style={{marginLeft: "8px"}}>
                          {i18next.t("task:Resolved by")} {comment.resolver}
                          {comment.resolvedTime ? ` @ ${comment.resolvedTime}` : ""}
                        </span>
                      )}
                      {comment.referencedField && (
                        <Tag style={{marginLeft: "8px"}} color="default">
                          {i18next.t(`task:${comment.referencedField.charAt(0).toUpperCase() + comment.referencedField.slice(1)}`)}
                        </Tag>
                      )}
                    </div>
                  }
                />
                {isEditing ? (
                  <div style={{paddingLeft: "44px", width: "100%"}}>
                    <TextArea
                      rows={3}
                      value={editingText}
                      onChange={(e) => setEditingText(e.target.value)}
                      disabled={submitting}
                    />
                  </div>
                ) : (
                  <div style={{
                    paddingLeft: "44px",
                    width: "100%",
                    whiteSpace: "pre-wrap",
                    wordBreak: "break-word",
                    lineHeight: 1.6,
                  }}>
                    {comment.content}
                  </div>
                )}
              </List.Item>
            );
          }}
        />
      </Drawer>
    </div>
  );
}

import React from "react";
import {Alert, Tag} from "antd";
import i18next from "i18next";

export function extractCapabilityError(res) {
  if (!res || res.status !== "error" || res.code !== "CAPABILITY_CHECK_FAILED") {
    return null;
  }
  const d = res.data;
  if (!d || typeof d !== "object") {
    return null;
  }
  return {
    blockReason: d.blockReason || res.msg || "",
    warnings: Array.isArray(d.warnings) ? d.warnings : [],
    failedResources: Array.isArray(d.failedResources) ? d.failedResources : [],
    state: d.state || "",
    needsRecheck: !!d.needsRecheck,
  };
}

export function CapabilityErrorAlert({error, style}) {
  if (!error) {
    return null;
  }

  return (
    <Alert
      type="error"
      showIcon
      style={{marginBottom: "16px", borderRadius: "12px", ...style}}
      message={i18next.t("general:Save blocked by capability check")}
      description={
        <div>
          {error.blockReason && (
            <div style={{marginBottom: "8px"}}>{error.blockReason}</div>
          )}
          {error.failedResources.length > 0 && (
            <div style={{marginTop: "8px"}}>
              <div style={{fontWeight: 600, marginBottom: "4px"}}>{i18next.t("general:Failed resources")}:</div>
              <ul style={{margin: 0, paddingLeft: "20px"}}>
                {error.failedResources.map((r, i) => (
                  <li key={i}>
                    <Tag color={r.state === "Active" ? "green" : r.state === "Error" ? "red" : "orange"} style={{marginRight: "8px"}}>
                      {r.kind}
                    </Tag>
                    <span style={{fontWeight: 500}}>{r.name}</span>
                    {r.reason && <span style={{color: "var(--ant-color-text-secondary)", marginLeft: "8px"}}>&mdash; {r.reason}</span>}
                  </li>
                ))}
              </ul>
            </div>
          )}
          {error.warnings.length > 0 && (
            <div style={{marginTop: "8px"}}>
              <div style={{fontWeight: 600, marginBottom: "4px", color: "#faad14"}}>{i18next.t("general:Warnings")}:</div>
              <ul style={{margin: 0, paddingLeft: "20px", color: "#faad14"}}>
                {error.warnings.map((w, i) => (
                  <li key={i}>{w}</li>
                ))}
              </ul>
            </div>
          )}
        </div>
      }
    />
  );
}

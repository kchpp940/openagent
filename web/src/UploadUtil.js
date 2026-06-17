export function isUploadSuccess(res) {
  if (!res) {return false;}
  if (res.status !== "ok") {return false;}
  const data = res.data;
  if (data === true) {return true;}
  if (data === false) {return false;}
  if (typeof data === "string") {return data.length > 0;}
  if (data && typeof data === "object") {
    if ("success" in data) {return !!data.success;}
    return true;
  }
  return !!data;
}

export function getUploadErrorMsg(res, fallback = "") {
  if (!res) {return fallback || "Unknown error";}
  if (res.status === "ok") {return "";}
  return res.msg || fallback || "Upload failed";
}

export function parseUploadResult(res) {
  const ok = isUploadSuccess(res);
  const msg = getUploadErrorMsg(res);
  if (!ok) {
    return {
      ok: false,
      msg: msg,
      url: "",
      fileName: "",
      fileSize: 0,
      fileType: "",
      fileFormat: "",
      mimeType: "",
      storageKey: "",
      fileUrl: "",
      fullFilePath: "",
      isLeaf: false,
      success: false,
    };
  }

  const data = res.data;
  if (typeof data === "string") {
    return {
      ok: true,
      msg: "",
      url: data,
      fileName: "",
      fileSize: 0,
      fileType: "",
      fileFormat: "",
      mimeType: "",
      storageKey: res.data2 || "",
      fileUrl: data,
      fullFilePath: res.data2 || "",
      isLeaf: false,
      success: true,
    };
  }

  if (data === true) {
    return {
      ok: true,
      msg: "",
      url: "",
      fileName: "",
      fileSize: 0,
      fileType: "",
      fileFormat: "",
      mimeType: "",
      storageKey: "",
      fileUrl: "",
      fullFilePath: "",
      isLeaf: false,
      success: true,
    };
  }

  const base = {
    ok: true,
    msg: "",
    url: (data && (data.url || data.fileUrl)) || "",
    fileName: (data && (data.fileName || data.filename)) || "",
    fileSize: (data && (data.fileSize || data.size)) || 0,
    fileType: (data && data.fileType) || "",
    fileFormat: (data && data.fileFormat) || "",
    mimeType: (data && data.mimeType) || "",
    storageKey: (data && (data.storageKey || data.fullFilePath)) || "",
    fileUrl: (data && (data.fileUrl || data.url)) || "",
    fullFilePath: (data && (data.fullFilePath || data.storageKey)) || "",
    isLeaf: (data && data.isLeaf) || false,
    success: true,
  };
  if (data && typeof data === "object") {
    Object.assign(base, data);
  }
  return base;
}

export function parseUploadResult(res) {
  if (!res || res.status !== "ok") {
    return null;
  }

  const data = res.data;
  if (!data) {
    return null;
  }

  if (typeof data === "string") {
    return {
      url: data,
      fileName: "",
      fileSize: 0,
      fileType: "",
      fileFormat: "",
      mimeType: "",
      storageKey: res.data2 || "",
      fileUrl: data,
      fullFilePath: res.data2 || "",
    };
  }

  return {
    url: data.url || data.fileUrl || "",
    fileName: data.fileName || data.filename || "",
    fileSize: data.fileSize || data.size || 0,
    fileType: data.fileType || "",
    fileFormat: data.fileFormat || "",
    mimeType: data.mimeType || "",
    storageKey: data.storageKey || data.fullFilePath || "",
    fileUrl: data.fileUrl || data.url || "",
    fullFilePath: data.fullFilePath || data.storageKey || "",
  };
}

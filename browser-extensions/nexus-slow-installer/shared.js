const DEFAULT_CONFIG = {
  panelBaseUrl: "http://127.0.0.1:5173",
  instanceId: "stardew",
  autoStartOnNexusFilePage: true,
  autoClickSlowDownload: true,
  cancelBrowserDownload: true
};

const CONFIG_KEY = "anxiNexusInstallerConfig";
const STATE_KEY = "anxiNexusInstallerState";

function normalizePanelBaseUrl(value) {
  const raw = String(value || "").trim().replace(/\/+$/, "");
  if (!raw) {
    return "";
  }
  try {
    const parsed = new URL(raw);
    if (parsed.protocol !== "http:" && parsed.protocol !== "https:") {
      return "";
    }
    return parsed.toString().replace(/\/+$/, "");
  } catch {
    return "";
  }
}

function parseNexusPageUrl(rawUrl) {
  try {
    const url = new URL(rawUrl);
    const match = url.pathname.match(/^\/([^/]+)\/mods\/(\d+)/i);
    if (!match) {
      return null;
    }
    const fileId = url.searchParams.get("file_id") || url.searchParams.get("file");
    return {
      gameDomain: match[1],
      modId: Number(match[2]),
      fileId: fileId ? Number(fileId) : 0,
      pageUrl: url.toString()
    };
  } catch {
    return null;
  }
}

function isNexusArchiveDownloadUrl(rawUrl) {
  try {
    const url = new URL(rawUrl);
    if (url.protocol !== "https:") {
      return false;
    }
    const host = url.hostname.toLowerCase();
    const isNexusHost = host === "supporter-files.nexus-cdn.com" || host.endsWith(".nexus-cdn.com");
    return isNexusHost && url.pathname.toLowerCase().endsWith(".zip");
  } catch {
    return false;
  }
}

function redactDownloadUrl(rawUrl) {
  try {
    const url = new URL(rawUrl);
    for (const key of Array.from(url.searchParams.keys())) {
      if (["md5", "expires", "user_id", "key"].includes(key.toLowerCase())) {
        url.searchParams.set(key, "[redacted]");
      }
    }
    return url.toString();
  } catch {
    return "[invalid-url]";
  }
}

function panelJobUrl(config, jobId) {
  const base = normalizePanelBaseUrl(config && config.panelBaseUrl);
  const instanceId = encodeURIComponent((config && config.instanceId) || DEFAULT_CONFIG.instanceId);
  const suffix = `/instances/${instanceId}/jobs${jobId ? `?jobId=${encodeURIComponent(jobId)}` : ""}`;
  return base ? `${base}${suffix}` : suffix;
}

function statusTextFromError(error) {
  if (!error) {
    return "未知错误";
  }
  if (typeof error === "string") {
    return error;
  }
  return error.message || String(error);
}

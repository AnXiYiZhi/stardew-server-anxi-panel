const DEFAULT_CONFIG = {
  panelBaseUrl: "http://127.0.0.1:5173",
  instanceId: "stardew",
  autoStartOnNexusFilePage: true,
  autoClickSlowDownload: true,
  cancelBrowserDownload: true
};

const CONFIG_KEY = "anxiNexusInstallerConfig";
const STATE_KEY = "anxiNexusInstallerState";

function createInstallRequestId() {
  if (typeof globalThis.crypto?.randomUUID === "function") {
    return globalThis.crypto.randomUUID();
  }
  if (typeof globalThis.crypto?.getRandomValues === "function") {
    const bytes = globalThis.crypto.getRandomValues(new Uint8Array(16));
    return Array.from(bytes, (value) => value.toString(16).padStart(2, "0")).join("");
  }
  return `compat-${Date.now().toString(36)}-${Math.random().toString(36).slice(2)}`;
}

function createSingleflightRegistry() {
  const flights = new Map();
  return {
    run(key, operation) {
      const normalizedKey = String(key || "").trim();
      if (!normalizedKey) {
        throw new Error("singleflight key is required");
      }
      const existing = flights.get(normalizedKey);
      if (existing) {
        return { promise: existing, shared: true };
      }

      // Defer the operation until after the Promise has been published. This
      // makes the check-and-register step atomic within the JavaScript event
      // loop even when operation starts with asynchronous storage or fetch work.
      const promise = Promise.resolve().then(operation);
      flights.set(normalizedKey, promise);
      const cleanup = () => {
        if (flights.get(normalizedKey) === promise) {
          flights.delete(normalizedKey);
        }
      };
      promise.then(cleanup, cleanup);
      return { promise, shared: false };
    }
  };
}

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

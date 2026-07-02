importScripts("shared.js");

async function getConfig() {
  const stored = await chrome.storage.local.get(CONFIG_KEY);
  return { ...DEFAULT_CONFIG, ...(stored[CONFIG_KEY] || {}) };
}

async function setConfig(config) {
  const normalized = {
    ...DEFAULT_CONFIG,
    ...config,
    panelBaseUrl: normalizePanelBaseUrl(config.panelBaseUrl),
    instanceId: String(config.instanceId || DEFAULT_CONFIG.instanceId).trim() || DEFAULT_CONFIG.instanceId
  };
  await chrome.storage.local.set({ [CONFIG_KEY]: normalized });
  return normalized;
}

async function getState() {
  const stored = await chrome.storage.local.get(STATE_KEY);
  return stored[STATE_KEY] || { capture: null, lastInstall: null };
}

async function setState(patch) {
  const state = await getState();
  const next = { ...state, ...patch };
  await chrome.storage.local.set({ [STATE_KEY]: next });
  return next;
}

async function notify(title, message) {
  try {
    await chrome.notifications.create({
      type: "basic",
      iconUrl: "icon.svg",
      title,
      message
    });
  } catch {
    // Notifications can be unavailable in some unpacked-extension contexts.
  }
}

function remoteInstallEndpoint(config) {
  return `${config.panelBaseUrl}/api/instances/${encodeURIComponent(config.instanceId)}/mods/remote/install`;
}

async function postRemoteInstall(downloadUrl, capture) {
  const config = await getConfig();
  if (!config.panelBaseUrl) {
    throw new Error("请先在扩展设置里填写面板地址");
  }

  const mod = capture && capture.modId
    ? {
        modId: capture.modId,
        name: capture.modName || "",
        nexusUrl: capture.pageUrl || `https://www.nexusmods.com/stardewvalley/mods/${capture.modId}`
      }
    : undefined;

  const response = await fetch(remoteInstallEndpoint(config), {
    method: "POST",
    credentials: "include",
    headers: {
      "Content-Type": "application/json",
      "X-Anxi-Nexus-Installer": "0.1.0"
    },
    body: JSON.stringify({ url: downloadUrl, mod })
  });

  let body = null;
  const text = await response.text();
  if (text) {
    try {
      body = JSON.parse(text);
    } catch {
      body = { raw: text };
    }
  }

  if (!response.ok) {
    const message = body && body.error && body.error.message
      ? body.error.message
      : `面板返回 HTTP ${response.status}`;
    const code = body && body.error && body.error.code ? ` (${body.error.code})` : "";
    throw new Error(`${message}${code}`);
  }

  return body || {};
}

async function debuggerClick(tabId, point) {
  if (!tabId) {
    throw new Error("没有可点击的标签页");
  }
  const target = { tabId };
  const x = Math.round(Number(point && point.x) || 0);
  const y = Math.round(Number(point && point.y) || 0);
  if (x <= 0 || y <= 0) {
    throw new Error("点击坐标无效");
  }

  let attached = false;
  try {
    await chrome.debugger.attach(target, "1.3");
    attached = true;
    await chrome.debugger.sendCommand(target, "Input.dispatchMouseEvent", {
      type: "mouseMoved",
      x,
      y,
      button: "none",
      clickCount: 0
    });
    await chrome.debugger.sendCommand(target, "Input.dispatchMouseEvent", {
      type: "mousePressed",
      x,
      y,
      button: "left",
      buttons: 1,
      clickCount: 1
    });
    await chrome.debugger.sendCommand(target, "Input.dispatchMouseEvent", {
      type: "mouseReleased",
      x,
      y,
      button: "left",
      buttons: 0,
      clickCount: 1
    });
  } finally {
    if (attached) {
      try {
        await chrome.debugger.detach(target);
      } catch {
        // The click already happened; detach failures are not actionable here.
      }
    }
  }
}

async function generateNexusUrlInMainWorld(tabId, payload) {
  if (!tabId) {
    throw new Error("没有可请求的 Nexus 标签页");
  }
  const fileId = String(payload && payload.fileId ? payload.fileId : "").trim();
  const gameId = String(payload && payload.gameId ? payload.gameId : "").trim();
  if (!fileId || !gameId) {
    throw new Error("缺少 Nexus file_id 或 game_id");
  }

  const results = await chrome.scripting.executeScript({
    target: { tabId },
    world: "MAIN",
    args: [fileId, gameId],
    func: async (fid, gid) => {
      const response = await fetch("/Core/Libs/Common/Managers/Downloads?GenerateDownloadUrl", {
        method: "POST",
        credentials: "same-origin",
        headers: {
          "Content-Type": "application/x-www-form-urlencoded; charset=UTF-8",
          "X-Requested-With": "XMLHttpRequest"
        },
        body: new URLSearchParams({ fid, game_id: gid }).toString()
      });
      const text = await response.text();
      if (!response.ok) {
        return { ok: false, error: "Nexus 生成链接失败：HTTP " + response.status, body: text.slice(0, 300) };
      }
      let data = null;
      try {
        data = JSON.parse(text);
      } catch (_) {
        data = null;
      }
      const url = data && typeof data.url === "string"
        ? data.url
        : ((text.match(/https:\/\/[^"'\s<>]+\.zip[^"'\s<>]*/i) || [])[0] || "");
      if (!url) {
        return { ok: false, error: "Nexus 未返回下载 URL", body: text.slice(0, 300) };
      }
      return { ok: true, url };
    }
  });

  const result = results && results[0] ? results[0].result : null;
  if (!result || !result.ok) {
    throw new Error((result && result.error) || "Nexus 页面主世界请求失败");
  }
  return result.url;
}

async function triggerNexusSlowDownloadInMainWorld(tabId) {
  if (!tabId) {
    throw new Error("没有可操作的 Nexus 标签页");
  }
  const results = await chrome.scripting.executeScript({
    target: { tabId },
    world: "MAIN",
    func: () => {
      function visible(el) {
        if (!el) return false;
        const style = getComputedStyle(el);
        const rect = el.getBoundingClientRect();
        return rect.width > 0 && rect.height > 0 && style.display !== "none" && style.visibility !== "hidden";
      }

      const component = document.querySelector("mod-file-download");
      if (component) {
        component.dispatchEvent(new CustomEvent("slowDownload", { bubbles: true, composed: true }));
        return { ok: true, method: "slowDownload-event" };
      }

      const shadowHosts = Array.from(document.querySelectorAll("*")).filter((el) => el.shadowRoot);
      for (const host of shadowHosts) {
        const buttons = Array.from(host.shadowRoot.querySelectorAll("button, a, [role='button']"));
        const button = buttons.find((el) => /slow\s*download/i.test((el.textContent || "").trim()) && visible(el));
        if (button) {
          button.click();
          return { ok: true, method: "shadow-button" };
        }
      }

      const buttons = Array.from(document.querySelectorAll("button, a, [role='button']"));
      const button = buttons.find((el) => /slow\s*download/i.test((el.textContent || "").trim()) && visible(el));
      if (button) {
        button.click();
        return { ok: true, method: "page-button" };
      }
      return { ok: false, error: "未找到 Nexus slow download 组件" };
    }
  });
  const result = results && results[0] ? results[0].result : null;
  if (!result || !result.ok) {
    throw new Error((result && result.error) || "触发 Nexus slowDownload 失败");
  }
  return result;
}

async function startCapture(payload, sender) {
  const now = Date.now();
  const capture = {
    active: true,
    createdAt: now,
    expiresAt: now + 10 * 60 * 1000,
    tabId: payload.tabId || (sender.tab && sender.tab.id) || null,
    gameDomain: payload.gameDomain || "stardewvalley",
    modId: Number(payload.modId || 0),
    fileId: Number(payload.fileId || 0),
    modName: payload.modName || "",
    pageUrl: payload.pageUrl || (sender.tab && sender.tab.url) || "",
    pendingUrl: ""
  };
  await setState({
    capture,
    lastInstall: {
      status: "capturing",
      message: "正在等待 Nexus CDN 下载链接",
      updatedAt: now,
      pageUrl: capture.pageUrl,
      modId: capture.modId,
      fileId: capture.fileId
    }
  });
  return capture;
}

async function storeCapturedDownloadUrl(downloadUrl, downloadItemId) {
  const config = await getConfig();
  const state = await getState();
  const capture = state.capture;
  if (!capture || !capture.active || Date.now() > capture.expiresAt) {
    return false;
  }

  if (config.cancelBrowserDownload && typeof downloadItemId === "number") {
    try {
      await chrome.downloads.cancel(downloadItemId);
      await chrome.downloads.erase({ id: downloadItemId });
    } catch {
      // If Chrome already finished or cannot erase the item, the panel install can still continue.
    }
  }

  await setState({
    capture: { ...capture, pendingUrl: downloadUrl },
    lastInstall: {
      status: "ready",
      message: "已拿到临时 ZIP 链接，等待提交",
      updatedAt: Date.now(),
      modId: capture.modId,
      fileId: capture.fileId,
      url: redactDownloadUrl(downloadUrl)
    }
  });

  if (capture.tabId) {
    try {
      await chrome.tabs.sendMessage(capture.tabId, { type: "CAPTURED_URL_READY", url: redactDownloadUrl(downloadUrl) });
    } catch {
      // The content script also polls state; a missed message is harmless.
    }
  }
  return true;
}

async function finishInstall(downloadUrl) {
  const config = await getConfig();
  const state = await getState();
  const capture = state.capture;
  if (!capture || !capture.active || Date.now() > capture.expiresAt) {
    throw new Error("捕获状态已过期，请重新打开 Nexus 下载页");
  }

  await setState({
    capture: { ...capture, active: false, pendingUrl: downloadUrl },
    lastInstall: {
      status: "posting",
      message: "正在提交给面板",
      updatedAt: Date.now(),
      modId: capture.modId,
      fileId: capture.fileId,
      url: redactDownloadUrl(downloadUrl)
    }
  });

  try {
    const result = await postRemoteInstall(downloadUrl, capture);
    const jobsUrl = panelJobUrl(config, result.jobId || "");
    await setState({
      lastInstall: {
        status: "queued",
        message: result.jobId ? `面板已创建安装任务：${result.jobId}` : "面板已接收安装请求",
        updatedAt: Date.now(),
        modId: capture.modId,
        fileId: capture.fileId,
        jobId: result.jobId || "",
        url: redactDownloadUrl(downloadUrl)
      }
    });
    await notify("已提交到面板", result.jobId ? `安装任务：${result.jobId}` : "远程安装任务已创建");
    return { ...result, jobsUrl };
  } catch (error) {
    await setState({
      lastInstall: {
        status: "failed",
        message: statusTextFromError(error),
        updatedAt: Date.now(),
        modId: capture.modId,
        fileId: capture.fileId,
        url: redactDownloadUrl(downloadUrl)
      }
    });
    await notify("提交失败", statusTextFromError(error));
    throw error;
  }
}

chrome.runtime.onInstalled.addListener(async () => {
  const config = await getConfig();
  await setConfig(config);
});

chrome.runtime.onMessage.addListener((message, sender, sendResponse) => {
  (async () => {
    switch (message && message.type) {
      case "GET_CONFIG": {
        sendResponse({ ok: true, config: await getConfig(), state: await getState() });
        return;
      }
      case "SAVE_CONFIG": {
        sendResponse({ ok: true, config: await setConfig(message.config || {}) });
        return;
      }
      case "START_CAPTURE": {
        sendResponse({ ok: true, capture: await startCapture(message.payload || {}, sender) });
        return;
      }
      case "CLEAR_STATE": {
        await setState({ capture: null, lastInstall: null });
        sendResponse({ ok: true });
        return;
      }
      case "CAPTURE_URL": {
        const url = message.url || "";
        if (!isNexusArchiveDownloadUrl(url)) {
          sendResponse({ ok: false, error: "不是 Nexus CDN ZIP 链接" });
          return;
        }
        const stored = await storeCapturedDownloadUrl(url, null);
        if (!stored) {
          sendResponse({ ok: false, error: "捕获状态已过期，请重新打开 Nexus 下载页" });
          return;
        }
        sendResponse({ ok: true });
        return;
      }
      case "SUBMIT_CAPTURED_URL": {
        const state = await getState();
        const capture = state.capture;
        const url = (message.url || (capture && capture.pendingUrl) || "").trim();
        if (!isNexusArchiveDownloadUrl(url)) {
          sendResponse({ ok: false, error: "还没有可提交的 Nexus CDN ZIP 链接" });
          return;
        }
        const result = await finishInstall(url);
        sendResponse({ ok: true, ...result });
        return;
      }
      case "INSTALL_URL": {
        const url = message.url || "";
        if (!isNexusArchiveDownloadUrl(url)) {
          sendResponse({ ok: false, error: "不是 Nexus CDN ZIP 链接" });
          return;
        }
        const result = await finishInstall(url);
        sendResponse({ ok: true, ...result });
        return;
      }
      case "DEBUGGER_CLICK": {
        const tabId = (sender.tab && sender.tab.id) || message.tabId;
        await debuggerClick(tabId, message.point || {});
        sendResponse({ ok: true });
        return;
      }
      case "GENERATE_NEXUS_URL": {
        const tabId = (sender.tab && sender.tab.id) || message.tabId;
        const url = await generateNexusUrlInMainWorld(tabId, message.payload || {});
        sendResponse({ ok: true, url });
        return;
      }
      case "TRIGGER_NEXUS_SLOW_DOWNLOAD": {
        const tabId = (sender.tab && sender.tab.id) || message.tabId;
        const result = await triggerNexusSlowDownloadInMainWorld(tabId);
        sendResponse({ ok: true, result });
        return;
      }
      default:
        sendResponse({ ok: false, error: "unknown_message" });
    }
  })().catch((error) => {
    sendResponse({ ok: false, error: statusTextFromError(error) });
  });
  return true;
});

chrome.downloads.onCreated.addListener((item) => {
  if (!item || !item.url || !isNexusArchiveDownloadUrl(item.url)) {
    return;
  }
  void storeCapturedDownloadUrl(item.url, item.id);
});

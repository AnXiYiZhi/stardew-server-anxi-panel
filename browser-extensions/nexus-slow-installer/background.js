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

async function updateConfig(patch) {
  const current = await getConfig();
  return await setConfig({ ...current, ...patch });
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

function captureKeyFrom(payload, sender) {
  if (payload && payload.captureKey) {
    return String(payload.captureKey);
  }
  if (payload && payload.batchId && payload.itemId) {
    return `${payload.batchId}:${payload.itemId}`;
  }
  const tabId = payload && payload.tabId ? payload.tabId : (sender && sender.tab && sender.tab.id);
  return tabId ? `tab:${tabId}` : "default";
}

function batchContextFromPayload(payload, captureKey) {
  const key = String((payload && payload.captureKey) || captureKey || "");
  let batchId = payload && payload.batchId ? String(payload.batchId) : "";
  let itemId = payload && payload.itemId ? String(payload.itemId) : "";
  if ((!batchId || !itemId) && key.includes(":")) {
    const parts = key.split(":", 2);
    batchId = batchId || parts[0] || "";
    itemId = itemId || parts[1] || "";
  }
  const resolvedKey = key || (batchId && itemId ? `${batchId}:${itemId}` : "");
  return {
    batchId,
    itemId,
    captureKey: resolvedKey,
    autoSubmit: Boolean(payload && payload.autoSubmit)
  };
}

function getCaptures(state) {
  return state && state.captures && typeof state.captures === "object" ? state.captures : {};
}

function getBatches(state) {
  return state && state.batches && typeof state.batches === "object" ? state.batches : {};
}

function findCapture(state, captureKey) {
  const captures = getCaptures(state);
  if (captureKey && captures[captureKey]) {
    return captures[captureKey];
  }
  if (state && state.capture) {
    return state.capture;
  }
  const active = Object.values(captures)
    .filter((capture) => capture && capture.active && Date.now() <= capture.expiresAt)
    .sort((a, b) => (a.createdAt || 0) - (b.createdAt || 0));
  return active[0] || null;
}

function captureKeyForCapture(state, capture) {
  if (!capture) return "";
  const captures = getCaptures(state);
  for (const [key, value] of Object.entries(captures)) {
    if (value === capture || (value && value.createdAt === capture.createdAt && value.tabId === capture.tabId)) {
      return key;
    }
  }
  return capture.captureKey || "";
}

function captureKeyFromDownloadItem(state, item) {
  if (!item) return "";
  const referrer = item.referrer || item.finalUrl || "";
  if (!referrer) return "";
  const pageInfo = parseNexusPageUrl(referrer);
  if (!pageInfo) return "";
  const captures = getCaptures(state);
  for (const [key, capture] of Object.entries(captures)) {
    if (!capture || !capture.active) continue;
    if (Number(capture.modId || 0) !== Number(pageInfo.modId || 0)) continue;
    if (pageInfo.fileId && Number(capture.fileId || 0) !== Number(pageInfo.fileId || 0)) continue;
    return key;
  }
  return "";
}

function normalizeBatchItemStatus(status) {
  if (status === "queued" || status === "done") return 100;
  if (status === "posting") return 85;
  if (status === "ready") return 65;
  if (status === "capturing") return 35;
  if (status === "opening") return 10;
  if (status === "failed") return 100;
  return 0;
}

function calculateBatchProgress(batch) {
  const items = batch && Array.isArray(batch.items) ? batch.items : [];
  if (items.length === 0) return 0;
  const total = items.reduce((sum, item) => sum + normalizeBatchItemStatus(item.status), 0);
  return Math.max(0, Math.min(100, Math.round(total / items.length)));
}

function isBatchComplete(batch) {
  const items = batch && Array.isArray(batch.items) ? batch.items : [];
  return items.length > 0 && items.every((item) => item.status === "queued" || item.status === "done");
}

function isBatchFailed(batch) {
  const items = batch && Array.isArray(batch.items) ? batch.items : [];
  return items.some((item) => item.status === "failed");
}

async function setBatchItemStatus(batchId, itemId, patch) {
  if (!batchId || !itemId) return null;
  const state = await getState();
  const batches = getBatches(state);
  const batch = batches[batchId];
  if (!batch) return null;
  const items = (batch.items || []).map((item) => (
    item.id === itemId
      ? { ...item, ...patch, updatedAt: Date.now() }
      : item
  ));
  const nextBatch = {
    ...batch,
    items,
    updatedAt: Date.now()
  };
  nextBatch.progress = calculateBatchProgress(nextBatch);
  nextBatch.status = isBatchFailed(nextBatch)
    ? "failed"
    : isBatchComplete(nextBatch)
      ? "done"
      : "running";
  const nextBatches = { ...batches, [batchId]: nextBatch };
  await setState({ batches: nextBatches });
  await notifyPanelBatch(nextBatch);
  return nextBatch;
}

async function notifyPanelBatch(batch) {
  if (!batch || !batch.panelTabId) return;
  try {
    await chrome.tabs.sendMessage(batch.panelTabId, { type: "BATCH_STATUS_UPDATE", batch });
  } catch {
    // The panel may have been refreshed; polling still works.
  }
}

function withBatchParams(rawUrl, batchId, item) {
  const url = new URL(rawUrl);
  url.searchParams.set("anxi_auto", "1");
  url.searchParams.set("anxi_auto_submit", "1");
  url.searchParams.set("anxi_batch", batchId);
  url.searchParams.set("anxi_item", item.id);
  return url.toString();
}

function normalizedBatchTargetUrl(rawUrl) {
  const value = String(rawUrl || "").trim();
  if (!value) return "";
  try {
    const url = new URL(value);
    url.hash = "";
    for (const key of ["anxi_auto", "anxi_auto_submit", "anxi_batch", "anxi_item"]) {
      url.searchParams.delete(key);
    }
    return url.toString();
  } catch {
    return value;
  }
}

function batchTargetKey(target) {
  const modId = Number(target && target.modId ? target.modId : 0);
  if (modId > 0) {
    return `mod:${modId}`;
  }
  const url = normalizedBatchTargetUrl(target && target.url);
  return url ? `url:${url}` : "";
}

function shouldReplaceBatchTarget(existing, candidate) {
  if (!existing) return true;
  if (!existing.url && candidate && candidate.url) return true;
  const existingRole = String(existing.role || "");
  const candidateRole = String((candidate && candidate.role) || "");
  return existingRole !== "target" && candidateRole === "target";
}

function uniqueBatchTargets(targets) {
  const byKey = new Map();
  for (const target of targets || []) {
    const key = batchTargetKey(target);
    if (!key) continue;
    const normalized = {
      ...target,
      url: normalizedBatchTargetUrl(target && target.url)
    };
    if (shouldReplaceBatchTarget(byKey.get(key), normalized)) {
      byKey.set(key, normalized);
    }
  }
  return Array.from(byKey.values());
}

async function getBatch(batchId) {
  const state = await getState();
  const batch = getBatches(state)[batchId] || null;
  if (!batch) return null;
  return {
    ...batch,
    progress: calculateBatchProgress(batch),
    status: isBatchFailed(batch) ? "failed" : isBatchComplete(batch) ? "done" : batch.status
  };
}

async function failBatchItemLater(batchId, itemId, timeoutMs) {
  globalThis.setTimeout(async () => {
    const batch = await getBatch(batchId);
    const item = batch && (batch.items || []).find((candidate) => candidate.id === itemId);
    if (!item || item.status === "queued" || item.status === "done" || item.status === "failed") {
      return;
    }
    await setBatchItemStatus(batchId, itemId, {
      status: "failed",
      message: "background page did not submit zip in time"
    });
  }, timeoutMs);
}

async function startBatchInstall(payload, sender) {
  const targets = uniqueBatchTargets(Array.isArray(payload && payload.targets) ? payload.targets : []);
  if (targets.length === 0) {
    throw new Error("no install targets");
  }
  const batchId = payload.batchId || `batch_${Date.now()}_${Math.random().toString(16).slice(2)}`;
  const panelTabId = sender && sender.tab && sender.tab.id ? sender.tab.id : null;
  const state = await getState();
  const batches = getBatches(state);
  const existingBatch = batches[batchId];
  if (existingBatch) {
    const nextBatch = panelTabId && existingBatch.panelTabId !== panelTabId
      ? { ...existingBatch, panelTabId, updatedAt: Date.now() }
      : existingBatch;
    if (nextBatch !== existingBatch) {
      await setState({ batches: { ...batches, [batchId]: nextBatch } });
    }
    await notifyPanelBatch(nextBatch);
    return await getBatch(batchId);
  }
  const now = Date.now();
  const items = targets.map((target, index) => ({
    id: target.id || `item_${index + 1}`,
    role: target.role || "mod",
    modId: Number(target.modId || 0),
    name: target.name || "",
    url: target.url || "",
    status: "opening",
    message: "opening background page",
    progress: 10,
    createdAt: now,
    updatedAt: now
  }));
  const batch = {
    id: batchId,
    status: "running",
    progress: calculateBatchProgress({ items }),
    panelTabId,
    createdAt: now,
    updatedAt: now,
    items
  };

  await setState({ batches: { ...batches, [batchId]: batch } });

  for (const item of items) {
    if (!item.url) {
      await setBatchItemStatus(batchId, item.id, { status: "failed", message: "missing Nexus URL" });
      continue;
    }
    try {
      const tab = await chrome.tabs.create({ url: withBatchParams(item.url, batchId, item), active: false });
      await setBatchItemStatus(batchId, item.id, {
        status: "opening",
        message: "background page opened",
        tabId: tab.id || null
      });
      void failBatchItemLater(batchId, item.id, 180000);
    } catch (error) {
      await setBatchItemStatus(batchId, item.id, {
        status: "failed",
        message: statusTextFromError(error)
      });
    }
  }

  return await getBatch(batchId);
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

async function fetchWithTimeout(url, options, timeoutMs) {
  const controller = new AbortController();
  const timer = globalThis.setTimeout(() => controller.abort(), timeoutMs);
  try {
    return await fetch(url, { ...options, signal: controller.signal });
  } catch (error) {
    if (error && error.name === "AbortError") {
      throw new Error(`提交面板超时：${Math.round(timeoutMs / 1000)} 秒内没有响应`);
    }
    throw error;
  } finally {
    globalThis.clearTimeout(timer);
  }
}

async function postRemoteInstall(downloadUrl, capture) {
  downloadUrl = String(downloadUrl || "").trim();
  if (!isNexusArchiveDownloadUrl(downloadUrl)) {
    throw new Error("还没有拿到 Nexus CDN ZIP 下载链接，不能创建面板安装任务");
  }
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

  const panelResult = await postRemoteInstallViaPanel(downloadUrl, capture, config, mod);
  if (panelResult) {
    return panelResult;
  }

  const endpoint = remoteInstallEndpoint(config);
  let response = null;
  try {
    response = await fetchWithTimeout(endpoint, {
      method: "POST",
      credentials: "include",
      headers: {
        "Content-Type": "application/json",
        "X-Anxi-Nexus-Installer": "0.1.2"
      },
      body: JSON.stringify({ url: downloadUrl, mod })
    }, 30000);
  } catch (error) {
    const message = statusTextFromError(error);
    throw new Error(`${message}；请确认扩展里的面板地址可访问且已登录管理员：${new URL(endpoint).origin}`);
  }

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

async function postRemoteInstallViaPanel(downloadUrl, capture, config, mod) {
  if (!capture || !capture.batchId) {
    return null;
  }
  const state = await getState();
  const batch = getBatches(state)[capture.batchId];
  if (!batch || !batch.panelTabId) {
    return null;
  }
  try {
    const response = await chrome.tabs.sendMessage(batch.panelTabId, {
      type: "PANEL_REMOTE_INSTALL",
      payload: {
        url: downloadUrl,
        mod,
        instanceId: config.instanceId || DEFAULT_CONFIG.instanceId
      }
    });
    if (!response || !response.ok) {
      throw new Error(response && response.error ? response.error : "panel page did not return an install result");
    }
    return response.result || {};
  } catch (error) {
    const message = statusTextFromError(error);
    if (/Receiving end does not exist|Could not establish connection|Extension context invalidated/i.test(message)) {
      return null;
    }
    throw new Error(`panel page submit failed: ${message}`);
  }
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

async function triggerNexusManualDownloadInMainWorld(tabId, params) {
  if (!tabId) {
    throw new Error("没有可操作的 Nexus 标签页");
  }
  const results = await chrome.scripting.executeScript({
    target: { tabId },
    world: "MAIN",
    args: [params || {}],
    func: (anxiParams) => {
      function textOf(node) {
        return (node && node.textContent ? node.textContent : "").replace(/\s+/g, " ").trim();
      }

      function visible(el) {
        if (!el) return false;
        const style = getComputedStyle(el);
        const rect = el.getBoundingClientRect();
        return rect.width > 0 && rect.height > 0 && style.display !== "none" && style.visibility !== "hidden" && style.pointerEvents !== "none";
      }

      function withParams(rawUrl) {
        const url = new URL(rawUrl, window.location.href);
        for (const [key, value] of Object.entries(anxiParams || {})) {
          if (value) {
            url.searchParams.set(key, value);
          }
        }
        return url.toString();
      }

      function elementHref(node) {
        if (!(node instanceof Element)) return "";
        const link = node instanceof HTMLAnchorElement ? node : node.closest("a[href]");
        if (link && link.href) return link.href;
        for (const attr of ["href", "data-href", "data-url", "data-download-url"]) {
          const value = node.getAttribute(attr);
          if (!value) continue;
          try {
            return new URL(value, window.location.href).toString();
          } catch (_) {
            // Try the next attribute.
          }
        }
        return "";
      }

      const candidates = Array.from(document.querySelectorAll("button, a, [role='button'], input[type='button'], input[type='submit']"));
      const matches = candidates.filter((node) => {
        const value = node instanceof HTMLInputElement ? node.value : textOf(node);
        return visible(node) && /manual\s+download/i.test(value);
      });
      matches.sort((a, b) => {
        const aText = a instanceof HTMLInputElement ? a.value : textOf(a);
        const bText = b instanceof HTMLInputElement ? b.value : textOf(b);
        const aExact = /^manual\s+download$/i.test(aText) ? 0 : 1;
        const bExact = /^manual\s+download$/i.test(bText) ? 0 : 1;
        if (aExact !== bExact) return aExact - bExact;
        const aRect = a.getBoundingClientRect();
        const bRect = b.getBoundingClientRect();
        return (aRect.top - bRect.top) || ((bRect.width * bRect.height) - (aRect.width * aRect.height));
      });
      const button = matches[0] || null;
      if (!button) {
        return { ok: false, error: "未找到 Manual download 按钮" };
      }

      const href = elementHref(button);
      if (href) {
        window.location.assign(withParams(href));
        return { ok: true, method: "manual-href" };
      }

      button.click();
      return { ok: true, method: "manual-main-click" };
    }
  });
  const result = results && results[0] ? results[0].result : null;
  if (!result || !result.ok) {
    throw new Error((result && result.error) || "触发 Nexus Manual download 失败");
  }
  return result;
}

async function startCapture(payload, sender) {
  const now = Date.now();
  const captureKey = captureKeyFrom(payload || {}, sender);
  const capture = {
    active: true,
    captureKey,
    batchId: payload.batchId || "",
    itemId: payload.itemId || "",
    autoSubmit: Boolean(payload.autoSubmit),
    closeTabOnComplete: Boolean(payload.closeTabOnComplete),
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
  const state = await getState();
  await setState({
    captures: { ...getCaptures(state), [captureKey]: capture },
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
  if (capture.batchId && capture.itemId) {
    await setBatchItemStatus(capture.batchId, capture.itemId, {
      status: "capturing",
      message: "capturing zip link",
      tabId: capture.tabId,
      captureKey
    });
  }
  return capture;
}

async function storeCapturedDownloadUrl(downloadUrl, downloadItemId, captureKey, downloadItem, contextPayload) {
  const config = await getConfig();
  const state = await getState();
  const matchedCaptureKey = captureKey || captureKeyFromDownloadItem(state, downloadItem);
  const capture = findCapture(state, matchedCaptureKey);
  if (!capture || !capture.active || Date.now() > capture.expiresAt) {
    return false;
  }
  const context = batchContextFromPayload(contextPayload || {}, matchedCaptureKey);
  const resolvedCaptureKey = context.captureKey || matchedCaptureKey || captureKeyForCapture(state, capture);

  if (config.cancelBrowserDownload && typeof downloadItemId === "number") {
    try {
      await chrome.downloads.cancel(downloadItemId);
      await chrome.downloads.erase({ id: downloadItemId });
    } catch {
      // If Chrome already finished or cannot erase the item, the panel install can still continue.
    }
  }

  const captures = getCaptures(state);
  const nextCapture = {
    ...capture,
    batchId: capture.batchId || context.batchId,
    itemId: capture.itemId || context.itemId,
    captureKey: resolvedCaptureKey || capture.captureKey,
    autoSubmit: Boolean(capture.autoSubmit || context.autoSubmit),
    closeTabOnComplete: Boolean(capture.closeTabOnComplete || context.autoSubmit),
    pendingUrl: downloadUrl
  };
  const capturedFromBrowserDownload = typeof downloadItemId === "number";
  await setState({
    captures: resolvedCaptureKey ? { ...captures, [resolvedCaptureKey]: nextCapture } : captures,
    capture: nextCapture,
    lastInstall: {
      status: "ready",
      message: "已拿到临时 ZIP 链接，等待提交",
      updatedAt: Date.now(),
      modId: capture.modId,
      fileId: capture.fileId,
      url: redactDownloadUrl(downloadUrl)
    }
  });

  let notifiedContentScript = false;
  if (nextCapture.tabId && (!nextCapture.autoSubmit || capturedFromBrowserDownload)) {
    try {
      await chrome.tabs.sendMessage(nextCapture.tabId, {
        type: "CAPTURED_URL_READY",
        captureKey: resolvedCaptureKey,
        autoSubmit: Boolean(nextCapture.autoSubmit),
        url: redactDownloadUrl(downloadUrl)
      });
      notifiedContentScript = true;
    } catch {
      // The content script also polls state; a missed message is harmless.
    }
  }
  if (nextCapture.batchId && nextCapture.itemId) {
    await setBatchItemStatus(nextCapture.batchId, nextCapture.itemId, {
      status: "ready",
      message: "zip link captured",
      captureKey: resolvedCaptureKey
    });
  }
  if (nextCapture.autoSubmit && capturedFromBrowserDownload) {
    globalThis.setTimeout(async () => {
      const latestState = await getState();
      const latestCapture = findCapture(latestState, resolvedCaptureKey);
      if (
        latestCapture &&
        latestCapture.active &&
        !latestCapture.autoSubmitting &&
        !latestCapture.jobId &&
        Date.now() <= latestCapture.expiresAt
      ) {
        await autoSubmitCapturedDownloadUrl(resolvedCaptureKey, downloadUrl);
      }
    }, notifiedContentScript ? 3000 : 0);
  }
  return true;
}

async function autoSubmitCapturedDownloadUrl(captureKey, downloadUrl) {
  const state = await getState();
  const capture = findCapture(state, captureKey);
  if (!capture || !capture.active || !capture.autoSubmit || capture.autoSubmitting || capture.jobId || Date.now() > capture.expiresAt) {
    return;
  }
  const resolvedCaptureKey = captureKey || captureKeyForCapture(state, capture);
  const captures = getCaptures(state);
  const nextCapture = { ...capture, pendingUrl: downloadUrl, autoSubmitting: true };
  await setState({
    captures: resolvedCaptureKey ? { ...captures, [resolvedCaptureKey]: nextCapture } : captures,
    capture: nextCapture
  });
  try {
    await finishInstall(downloadUrl, resolvedCaptureKey);
  } catch {
    // finishInstall records the failure in extension state and batch status.
  }
}

async function finishInstall(downloadUrl, captureKey) {
  downloadUrl = String(downloadUrl || "").trim();
  if (!isNexusArchiveDownloadUrl(downloadUrl)) {
    throw new Error("还没有拿到 Nexus CDN ZIP 下载链接，不能创建面板安装任务");
  }
  const config = await getConfig();
  const state = await getState();
  const rawCapture = findCapture(state, captureKey);
  if (!rawCapture || !rawCapture.active || Date.now() > rawCapture.expiresAt) {
    throw new Error("捕获状态已过期，请重新打开 Nexus 下载页");
  }

  const context = batchContextFromPayload({}, captureKey);
  const resolvedCaptureKey = context.captureKey || captureKey || captureKeyForCapture(state, rawCapture);
  const capture = {
    ...rawCapture,
    batchId: rawCapture.batchId || context.batchId,
    itemId: rawCapture.itemId || context.itemId,
    captureKey: resolvedCaptureKey || rawCapture.captureKey,
    autoSubmit: Boolean(rawCapture.autoSubmit || context.autoSubmit),
    closeTabOnComplete: Boolean(rawCapture.closeTabOnComplete || context.autoSubmit)
  };
  const postingCapture = { ...capture, pendingUrl: downloadUrl, autoSubmitting: true };

  await setState({
    captures: resolvedCaptureKey ? { ...getCaptures(state), [resolvedCaptureKey]: postingCapture } : getCaptures(state),
    capture: postingCapture,
    lastInstall: {
      status: "posting",
      message: "正在提交给面板",
      updatedAt: Date.now(),
      modId: capture.modId,
      fileId: capture.fileId,
      url: redactDownloadUrl(downloadUrl)
    }
  });
  if (capture.batchId && capture.itemId) {
    await setBatchItemStatus(capture.batchId, capture.itemId, {
      status: "posting",
      message: "submitting to panel"
    });
  }

  try {
    const result = await postRemoteInstall(downloadUrl, capture);
    const jobsUrl = panelJobUrl(config, result.jobId || "");
    const doneState = await getState();
    const doneCaptures = getCaptures(doneState);
    const doneCapture = { ...postingCapture, active: false, pendingUrl: downloadUrl, jobId: result.jobId || "" };
    await setState({
      captures: resolvedCaptureKey ? { ...doneCaptures, [resolvedCaptureKey]: doneCapture } : doneCaptures,
      capture: doneCapture,
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
    if (capture.batchId && capture.itemId) {
      await setBatchItemStatus(capture.batchId, capture.itemId, {
        status: "queued",
        message: "panel job created",
        jobId: result.jobId || ""
      });
    }
    if (capture.closeTabOnComplete && capture.tabId) {
      try {
        await chrome.tabs.remove(capture.tabId);
      } catch {
        // The user may already have closed the tab.
      }
    }
    return { ...result, jobsUrl };
  } catch (error) {
    const failedState = await getState();
    const failedCaptures = getCaptures(failedState);
    const failedCapture = { ...postingCapture, active: false, error: statusTextFromError(error) };
    await setState({
      captures: resolvedCaptureKey ? { ...failedCaptures, [resolvedCaptureKey]: failedCapture } : failedCaptures,
      capture: failedCapture,
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
    if (capture.batchId && capture.itemId) {
      await setBatchItemStatus(capture.batchId, capture.itemId, {
        status: "failed",
        message: statusTextFromError(error)
      });
    }
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
      case "REGISTER_PANEL": {
        const panelBaseUrl = normalizePanelBaseUrl(message.panelBaseUrl || (message.config && message.config.panelBaseUrl));
        const instanceId = String(message.instanceId || (message.config && message.config.instanceId) || DEFAULT_CONFIG.instanceId).trim();
        if (!panelBaseUrl) {
          sendResponse({ ok: false, error: "invalid panel url" });
          return;
        }
        sendResponse({ ok: true, config: await updateConfig({ panelBaseUrl, instanceId: instanceId || DEFAULT_CONFIG.instanceId }) });
        return;
      }
      case "PING": {
        sendResponse({ ok: true, config: await getConfig(), state: await getState() });
        return;
      }
      case "START_CAPTURE": {
        sendResponse({ ok: true, capture: await startCapture(message.payload || {}, sender) });
        return;
      }
      case "CLEAR_STATE": {
        await setState({ capture: null, captures: {}, batches: {}, lastInstall: null });
        sendResponse({ ok: true });
        return;
      }
      case "START_BATCH_INSTALL": {
        sendResponse({ ok: true, batch: await startBatchInstall(message.payload || {}, sender) });
        return;
      }
      case "GET_BATCH_STATUS": {
        sendResponse({ ok: true, batch: await getBatch(message.batchId || "") });
        return;
      }
      case "CAPTURE_URL": {
        const url = message.url || "";
        if (!isNexusArchiveDownloadUrl(url)) {
          sendResponse({ ok: false, error: "不是 Nexus CDN ZIP 链接" });
          return;
        }
        const requestCaptureKey = captureKeyFrom(message, sender);
        const stored = await storeCapturedDownloadUrl(url, null, requestCaptureKey, null, message);
        if (!stored) {
          sendResponse({ ok: false, error: "捕获状态已过期，请重新打开 Nexus 下载页" });
          return;
        }
        sendResponse({ ok: true });
        return;
      }
      case "SUBMIT_CAPTURED_URL": {
        const state = await getState();
        const requestCaptureKey = captureKeyFrom(message, sender);
        const capture = findCapture(state, requestCaptureKey);
        const url = (message.url || (capture && capture.pendingUrl) || "").trim();
        if (!isNexusArchiveDownloadUrl(url)) {
          sendResponse({ ok: false, error: "还没有可提交的 Nexus CDN ZIP 链接" });
          return;
        }
        const result = await finishInstall(url, requestCaptureKey);
        sendResponse({ ok: true, ...result });
        return;
      }
      case "INSTALL_URL": {
        const url = message.url || "";
        if (!isNexusArchiveDownloadUrl(url)) {
          sendResponse({ ok: false, error: "不是 Nexus CDN ZIP 链接" });
          return;
        }
        const result = await finishInstall(url, captureKeyFrom(message, sender));
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
      case "TRIGGER_NEXUS_MANUAL_DOWNLOAD": {
        const tabId = (sender.tab && sender.tab.id) || message.tabId;
        const result = await triggerNexusManualDownloadInMainWorld(tabId, message.params || {});
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
  void storeCapturedDownloadUrl(item.url, item.id, "", item);
});

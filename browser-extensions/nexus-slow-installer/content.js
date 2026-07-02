(() => {
  let pageInfo = parseNexusPageUrl(window.location.href);
  if (!pageInfo) {
    return;
  }

  let config = { ...DEFAULT_CONFIG };
  let hasStarted = false;
  let overlay = null;
  let statusNode = null;
  let submitButton = null;
  let pendingDownloadUrl = "";
  let submitting = false;
  let observer = null;
  let currentUrl = window.location.href;
  let additionalDownloadClicking = false;
  let lastAdditionalDownloadClickAt = 0;
  const BACKGROUND_PENDING_URL = "__anxi_background_pending_url__";
  const AUTOMATION_SESSION_KEY = "anxiNexusInstallerAutomation";

  function textOf(node) {
    return (node && node.textContent ? node.textContent : "").replace(/\s+/g, " ").trim();
  }

  function isVisible(node) {
    if (!(node instanceof HTMLElement)) {
      return false;
    }
    const rect = node.getBoundingClientRect();
    const style = window.getComputedStyle(node);
    return rect.width > 0 && rect.height > 0 && style.visibility !== "hidden" && style.display !== "none" && style.pointerEvents !== "none";
  }

  function findSlowDownloadButton() {
    const candidates = Array.from(document.querySelectorAll("button, a, [role='button'], input[type='button'], input[type='submit']"));
    const matches = candidates.filter((node) => {
      const value = node instanceof HTMLInputElement ? node.value : textOf(node);
      return isVisible(node) && /slow\s+download/i.test(value);
    });
    matches.sort((a, b) => {
      const aText = a instanceof HTMLInputElement ? a.value : textOf(a);
      const bText = b instanceof HTMLInputElement ? b.value : textOf(b);
      const aExact = /^slow\s+download$/i.test(aText) ? 0 : 1;
      const bExact = /^slow\s+download$/i.test(bText) ? 0 : 1;
      if (aExact !== bExact) {
        return aExact - bExact;
      }
      const aRect = a.getBoundingClientRect();
      const bRect = b.getBoundingClientRect();
      return (aRect.width * aRect.height) - (bRect.width * bRect.height);
    });
    return matches[0] || null;
  }

  function findManualDownloadButton() {
    const candidates = Array.from(document.querySelectorAll("button, a, [role='button'], input[type='button'], input[type='submit']"));
    const matches = candidates.filter((node) => {
      const value = node instanceof HTMLInputElement ? node.value : textOf(node);
      return isVisible(node) && /manual\s+download/i.test(value);
    });
    matches.sort((a, b) => {
      const aText = a instanceof HTMLInputElement ? a.value : textOf(a);
      const bText = b instanceof HTMLInputElement ? b.value : textOf(b);
      const aExact = /^manual\s+download$/i.test(aText) ? 0 : 1;
      const bExact = /^manual\s+download$/i.test(bText) ? 0 : 1;
      if (aExact !== bExact) {
        return aExact - bExact;
      }
      const aRect = a.getBoundingClientRect();
      const bRect = b.getBoundingClientRect();
      return (aRect.top - bRect.top) || ((bRect.width * bRect.height) - (aRect.width * aRect.height));
    });
    return matches[0] || null;
  }

  function elementHref(node) {
    if (!(node instanceof Element)) {
      return "";
    }
    const link = node instanceof HTMLAnchorElement ? node : node.closest("a[href]");
    if (link && link.href) {
      return link.href;
    }
    for (const attr of ["href", "data-href", "data-url", "data-download-url"]) {
      const value = node.getAttribute(attr);
      if (value) {
        try {
          return new URL(value, window.location.href).toString();
        } catch {
          // Try the next attribute.
        }
      }
    }
    return "";
  }

  function withCurrentAnxiParams(rawUrl) {
    const target = new URL(rawUrl, window.location.href);
    const current = new URL(window.location.href);
    for (const key of ["anxi_auto", "anxi_auto_submit", "anxi_batch", "anxi_item"]) {
      const value = current.searchParams.get(key);
      if (value) {
        target.searchParams.set(key, value);
      }
    }
    const batch = batchParams();
    if (batch.autoSubmit) {
      target.searchParams.set("anxi_auto", "1");
      target.searchParams.set("anxi_auto_submit", "1");
    }
    if (batch.batchId) {
      target.searchParams.set("anxi_batch", batch.batchId);
    }
    if (batch.itemId) {
      target.searchParams.set("anxi_item", batch.itemId);
    }
    return target.toString();
  }

  function navigateWithCurrentAnxiParams(rawUrl) {
    try {
      const nextUrl = withCurrentAnxiParams(rawUrl);
      if (nextUrl && nextUrl !== window.location.href) {
        window.location.assign(nextUrl);
        return true;
      }
    } catch {
      // Fall back to event-based clicks when Nexus gives us a JS-only button.
    }
    return false;
  }

  function currentAnxiParams() {
    const params = {};
    try {
      const current = new URL(window.location.href);
      for (const key of ["anxi_auto", "anxi_auto_submit", "anxi_batch", "anxi_item"]) {
        const value = current.searchParams.get(key);
        if (value) {
          params[key] = value;
        }
      }
    } catch {
      // No automation params to preserve.
    }
    const batch = batchParams();
    if (batch.autoSubmit) {
      params.anxi_auto = "1";
      params.anxi_auto_submit = "1";
    }
    if (batch.batchId) {
      params.anxi_batch = batch.batchId;
    }
    if (batch.itemId) {
      params.anxi_item = batch.itemId;
    }
    return params;
  }

  function closestAdditionalFilesDialog(node) {
    let current = node instanceof Element ? node : null;
    for (let depth = 0; current && depth < 10; depth += 1) {
      const text = textOf(current).toLowerCase();
      const className = String(current.className || "");
      const isDialogLike = current.getAttribute("role") === "dialog" || /modal|dialog|popup|reveal/i.test(className);
      if (text.includes("additional files required") || text.includes("requires one or more additional files")) {
        return current;
      }
      if (isDialogLike && text.includes("additional files") && text.includes("required")) {
        return current;
      }
      current = current.parentElement;
    }
    return null;
  }

  function findAdditionalFilesDownloadButton() {
    const candidates = Array.from(document.querySelectorAll("button, a, [role='button'], input[type='button'], input[type='submit']"));
    const matches = candidates.filter((node) => {
      const value = node instanceof HTMLInputElement ? node.value : textOf(node);
      return isVisible(node) && /^download$/i.test(value) && closestAdditionalFilesDialog(node);
    });
    matches.sort((a, b) => {
      const aRect = a.getBoundingClientRect();
      const bRect = b.getBoundingClientRect();
      return (aRect.top - bRect.top) || (aRect.left - bRect.left);
    });
    return matches[0] || null;
  }

  async function clickAdditionalFilesDownloadIfPresent() {
    if (!hasStarted || additionalDownloadClicking || Date.now() - lastAdditionalDownloadClickAt < 2500) {
      return false;
    }
    const button = findAdditionalFilesDownloadButton();
    if (!button) {
      return false;
    }
    additionalDownloadClicking = true;
    lastAdditionalDownloadClickAt = Date.now();
    setStatus("检测到 Nexus 前置确认弹窗，正在点击 Download...");
    try {
      const href = elementHref(button);
      if (href && navigateWithCurrentAnxiParams(href)) {
        return true;
      }
      await dispatchExtensionClick(button);
    } catch {
      dispatchMouseLikeClick(button);
    } finally {
      window.setTimeout(() => {
        additionalDownloadClicking = false;
      }, 900);
    }
    return true;
  }

  function findDirectArchiveLink() {
    const links = Array.from(document.querySelectorAll("a[href]"));
    const found = links.find((link) => isNexusArchiveDownloadUrl(link.href));
    return found ? found.href : "";
  }

  function findNexusGameId() {
    const explicit = document.querySelector("[data-game-id]");
    if (explicit && explicit.dataset && explicit.dataset.gameId) {
      return explicit.dataset.gameId;
    }
    const section = document.getElementById("section");
    if (section && section.dataset && section.dataset.gameId) {
      return section.dataset.gameId;
    }
    if (pageInfo && pageInfo.gameDomain === "stardewvalley") {
      return "1303";
    }
    return "";
  }

  function decodeHtmlEntities(value) {
    const textarea = document.createElement("textarea");
    textarea.innerHTML = value;
    return textarea.value;
  }

  function findEmbeddedDownloadUrl(root = document) {
    const slowButton = root.getElementById ? root.getElementById("slowDownloadButton") : null;
    if (slowButton && slowButton.dataset && slowButton.dataset.downloadUrl && isNexusArchiveDownloadUrl(slowButton.dataset.downloadUrl)) {
      return slowButton.dataset.downloadUrl;
    }
    const attrButton = root.querySelector ? root.querySelector("[data-download-url]") : null;
    if (attrButton && attrButton.getAttribute("data-download-url")) {
      const candidate = decodeHtmlEntities(attrButton.getAttribute("data-download-url"));
      if (isNexusArchiveDownloadUrl(candidate)) {
        return candidate;
      }
    }
    const nestedComponents = root.querySelectorAll ? Array.from(root.querySelectorAll("mod-file-download")) : [];
    for (const component of nestedComponents) {
      if (component.shadowRoot) {
        const candidate = findEmbeddedDownloadUrl(component.shadowRoot);
        if (candidate) {
          return candidate;
        }
      }
    }
    const html = root.documentElement ? root.documentElement.innerHTML : (root.innerHTML || "");
    const patterns = [
      /const\s+downloadUrl\s*=\s*'([^']+)'/i,
      /id=["']slowDownloadButton["'][\s\S]*?data-download-url=["']([^"']+)["']/i,
      /data-download-url=["']([^"']+\.zip[^"']*)["']/i
    ];
    for (const pattern of patterns) {
      const match = html.match(pattern);
      if (match && match[1]) {
        const candidate = decodeHtmlEntities(match[1]);
        if (isNexusArchiveDownloadUrl(candidate)) {
          return candidate;
        }
      }
    }
    return "";
  }

  async function generateNexusDownloadUrl() {
    const fileId = pageInfo && pageInfo.fileId ? String(pageInfo.fileId) : "";
    const gameId = findNexusGameId();
    if (!fileId || !gameId) {
      throw new Error("缺少 Nexus file_id 或 game_id");
    }

    const embedded = findEmbeddedDownloadUrl();
    if (embedded) {
      return embedded;
    }

    const response = await chrome.runtime.sendMessage({
      type: "GENERATE_NEXUS_URL",
      payload: { fileId, gameId }
    });
    if (!response || !response.ok) {
      throw new Error(response && response.error ? response.error : "Nexus 主世界请求失败");
    }
    if (isNexusArchiveDownloadUrl(response.url)) {
      return response.url;
    }
    if (response.url) {
      throw new Error("Nexus 返回的不是 ZIP 临时链接");
    }

    const fallbackResponse = await fetch("/Core/Libs/Common/Managers/Downloads?GenerateDownloadUrl", {
      method: "POST",
      credentials: "include",
      headers: {
        "Content-Type": "application/x-www-form-urlencoded; charset=UTF-8",
        "X-Requested-With": "XMLHttpRequest"
      },
      body: new URLSearchParams({ fid: fileId, game_id: gameId }).toString()
    });
    const text = await fallbackResponse.text();
    if (!fallbackResponse.ok) {
      throw new Error(`Nexus 生成链接失败：HTTP ${fallbackResponse.status}`);
    }

    let parsed = null;
    try {
      parsed = JSON.parse(text);
    } catch {
      parsed = null;
    }
    const candidates = [];
    if (parsed && typeof parsed.url === "string") {
      candidates.push(parsed.url);
    }
    const textMatch = text.match(/https:\/\/[^"'\s<>]+\.zip[^"'\s<>]*/i);
    if (textMatch) {
      candidates.push(decodeHtmlEntities(textMatch[0]));
    }
    for (const candidate of candidates) {
      if (isNexusArchiveDownloadUrl(candidate)) {
        return candidate;
      }
    }
    throw new Error("Nexus 未返回可用的 ZIP 临时链接");
  }

  function setStatus(message) {
    if (statusNode) {
      statusNode.textContent = message;
    }
  }

  function setSubmitEnabled(enabled) {
    if (submitButton) {
      submitButton.disabled = !enabled;
      submitButton.textContent = enabled ? "提交到面板" : "等待 ZIP 链接";
    }
  }

  function hasAutoFlag() {
    try {
      const url = new URL(window.location.href);
      return url.searchParams.get("anxi_auto") === "1" || readAutomationParams().autoSubmit;
    } catch {
      return readAutomationParams().autoSubmit;
    }
  }

  function emptyBatchParams() {
    return { batchId: "", itemId: "", captureKey: "", autoSubmit: false };
  }

  function readAutomationParams() {
    try {
      const raw = window.sessionStorage.getItem(AUTOMATION_SESSION_KEY);
      if (!raw) {
        return emptyBatchParams();
      }
      const parsed = JSON.parse(raw);
      if (!parsed || !parsed.expiresAt || Date.now() > Number(parsed.expiresAt)) {
        window.sessionStorage.removeItem(AUTOMATION_SESSION_KEY);
        return emptyBatchParams();
      }
      if (parsed.modId && pageInfo && Number(parsed.modId) !== Number(pageInfo.modId)) {
        return emptyBatchParams();
      }
      const batchId = String(parsed.batchId || "");
      const itemId = String(parsed.itemId || "");
      return {
        batchId,
        itemId,
        captureKey: String(parsed.captureKey || (batchId && itemId ? `${batchId}:${itemId}` : "")),
        autoSubmit: Boolean(parsed.autoSubmit)
      };
    } catch {
      return emptyBatchParams();
    }
  }

  function rememberAutomationParams(params) {
    if (!params || (!params.autoSubmit && !params.batchId && !params.itemId && !params.captureKey)) {
      return;
    }
    try {
      const batchId = String(params.batchId || "");
      const itemId = String(params.itemId || "");
      window.sessionStorage.setItem(AUTOMATION_SESSION_KEY, JSON.stringify({
        batchId,
        itemId,
        captureKey: String(params.captureKey || (batchId && itemId ? `${batchId}:${itemId}` : "")),
        autoSubmit: Boolean(params.autoSubmit),
        modId: pageInfo && pageInfo.modId ? pageInfo.modId : 0,
        expiresAt: Date.now() + 15 * 60 * 1000
      }));
    } catch {
      // Losing this only falls back to the visible submit button.
    }
  }

  function batchParams() {
    try {
      const url = new URL(window.location.href);
      const batchId = url.searchParams.get("anxi_batch") || "";
      const itemId = url.searchParams.get("anxi_item") || "";
      const current = {
        batchId,
        itemId,
        captureKey: batchId && itemId ? `${batchId}:${itemId}` : "",
        autoSubmit: url.searchParams.get("anxi_auto_submit") === "1"
      };
      if (current.autoSubmit || current.batchId || current.itemId) {
        rememberAutomationParams(current);
        return current;
      }
      return readAutomationParams();
    } catch {
      return readAutomationParams();
    }
  }

  function createOverlay() {
    if (overlay) {
      return;
    }
    overlay = document.createElement("div");
    overlay.id = "anxi-nexus-installer-overlay";
    overlay.innerHTML = `
      <div class="anxi-status">正在获取 ZIP 链接</div>
      <div class="anxi-actions">
        <button type="button" class="anxi-primary" disabled>等待 ZIP 链接</button>
      </div>
    `;
    const style = document.createElement("style");
    style.textContent = `
      #anxi-nexus-installer-overlay {
        position: fixed;
        right: 18px;
        bottom: 18px;
        z-index: 2147483647;
        box-sizing: border-box;
        width: 310px;
        padding: 14px;
        color: #f6ead2;
        background: rgba(32, 24, 18, 0.96);
        border: 1px solid #d6a85f;
        border-radius: 8px;
        box-shadow: 0 12px 28px rgba(0, 0, 0, 0.35);
        font: 14px/1.45 system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
      }
      #anxi-nexus-installer-overlay .anxi-status {
        margin: 0 0 10px;
        color: #fff6df;
      }
      #anxi-nexus-installer-overlay .anxi-actions {
        display: flex;
        gap: 8px;
        flex-wrap: wrap;
      }
      #anxi-nexus-installer-overlay button {
        cursor: pointer;
        border: 1px solid #d6a85f;
        border-radius: 6px;
        padding: 7px 10px;
        color: #2b2118;
        background: #f6d083;
        font: inherit;
      }
      #anxi-nexus-installer-overlay button:disabled {
        cursor: default;
        opacity: 0.62;
      }
    `;
    document.documentElement.appendChild(style);
    document.documentElement.appendChild(overlay);
    statusNode = overlay.querySelector(".anxi-status");
    submitButton = overlay.querySelector(".anxi-primary");
    submitButton.addEventListener("click", () => {
      void submitCapturedUrl();
    });
    setSubmitEnabled(false);
  }

  async function beginCapture(clickSlow) {
    hasStarted = true;
    const batch = batchParams();
    rememberAutomationParams(batch);
    await chrome.runtime.sendMessage({
      type: "START_CAPTURE",
      payload: {
        ...pageInfo,
        modName: document.querySelector("h1") ? textOf(document.querySelector("h1")) : "",
        pageUrl: window.location.href,
        batchId: batch.batchId,
        itemId: batch.itemId,
        captureKey: batch.captureKey,
        autoSubmit: batch.autoSubmit,
        closeTabOnComplete: batch.autoSubmit
      }
    });

    if (!pageInfo.fileId) {
      setStatus("正在打开 Nexus 文件下载页...");
      const clicked = await clickManualDownloadWhenReady();
      if (!clicked) {
        setStatus("未找到 Manual download 按钮，请刷新页面后重试。");
      }
      return;
    }

    setStatus("正在获取 Nexus 临时 ZIP 链接...");
    const directLink = findDirectArchiveLink();
    if (directLink) {
      await captureUrl(directLink);
      return;
    }

    let directError = "";
    try {
      const generatedUrl = await generateNexusDownloadUrl();
      await captureUrl(generatedUrl);
      return;
    } catch (error) {
      directError = error && error.message ? error.message : String(error);
      setStatus(`直接生成链接失败，尝试自动触发页面下载：${directError}`);
    }

    if (!clickSlow) {
      setStatus("自动生成链接失败。");
      return;
    }

    const clicked = await clickSlowDownloadWhenReady();
    setStatus(clicked ? `已通过扩展调试点击 Slow download，等待浏览器下载事件。直接生成失败：${directError}` : `自动捕获失败：${directError || "没有找到 Slow download 按钮"}`);
  }

  async function captureUrl(url) {
    const batch = batchParams();
    rememberAutomationParams(batch);
    pendingDownloadUrl = url;
    const response = await chrome.runtime.sendMessage({
      type: "CAPTURE_URL",
      url,
      captureKey: batch.captureKey,
      batchId: batch.batchId,
      itemId: batch.itemId,
      autoSubmit: batch.autoSubmit
    });
    if (!response || !response.ok) {
      throw new Error(response && response.error ? response.error : "保存 ZIP 链接失败");
    }
    setStatus("ZIP 链接已获取");
    if (batch.autoSubmit) {
      setStatus("ZIP 链接已获取，后台正在自动提交到面板...");
      void submitCapturedUrl();
      return;
    }
    setSubmitEnabled(true);
  }

  async function submitCapturedUrl() {
    if (!pendingDownloadUrl || submitting) {
      return;
    }
    const batch = batchParams();
    rememberAutomationParams(batch);
    submitting = true;
    setSubmitEnabled(false);
    setStatus("正在提交到面板...");
    try {
      const message = pendingDownloadUrl === BACKGROUND_PENDING_URL
        ? {
            type: "SUBMIT_CAPTURED_URL",
            captureKey: batch.captureKey,
            batchId: batch.batchId,
            itemId: batch.itemId,
            autoSubmit: batch.autoSubmit
          }
        : {
            type: "SUBMIT_CAPTURED_URL",
            url: pendingDownloadUrl,
            captureKey: batch.captureKey,
            batchId: batch.batchId,
            itemId: batch.itemId,
            autoSubmit: batch.autoSubmit
          };
      const response = await chrome.runtime.sendMessage(message);
      if (!response || !response.ok) {
        throw new Error(response && response.error ? response.error : "提交失败");
      }
      setStatus("已提交，正在返回任务日志...");
      if (batch.autoSubmit) {
        setStatus("Submitted to panel.");
        return;
      }
      if (response.jobsUrl) {
        window.location.assign(response.jobsUrl);
      }
    } catch (error) {
      submitting = false;
      setStatus(error && error.message ? error.message : String(error));
      setSubmitEnabled(true);
    }
  }

  async function dispatchExtensionClick(target) {
    target.scrollIntoView({ block: "center", inline: "center", behavior: "instant" });
    if (typeof target.focus === "function") {
      target.focus({ preventScroll: true });
    }
    const rect = target.getBoundingClientRect();
    const x = rect.left + rect.width / 2;
    const y = rect.top + rect.height / 2;
    const response = await chrome.runtime.sendMessage({
      type: "DEBUGGER_CLICK",
      point: { x, y }
    });
    if (!response || !response.ok) {
      throw new Error(response && response.error ? response.error : "debugger click failed");
    }
  }

  function dispatchMouseLikeClick(target) {
    const rect = target.getBoundingClientRect();
    const x = rect.left + rect.width / 2;
    const y = rect.top + rect.height / 2;
    const eventOptions = {
      bubbles: true,
      cancelable: true,
      composed: true,
      view: window,
      clientX: x,
      clientY: y,
      screenX: window.screenX + x,
      screenY: window.screenY + y,
      button: 0,
      buttons: 1
    };
    for (const type of ["pointerover", "mouseover", "pointerenter", "mouseenter", "pointerdown", "mousedown", "pointerup", "mouseup", "click"]) {
      const event = type.startsWith("pointer")
        ? new PointerEvent(type, { ...eventOptions, pointerId: 1, pointerType: "mouse", isPrimary: true })
        : new MouseEvent(type, eventOptions);
      target.dispatchEvent(event);
    }
    if (typeof target.click === "function") {
      target.click();
    }
  }

  function clickSlowDownloadWhenReady() {
    return new Promise((resolve) => {
      const deadline = Date.now() + 30000;
      let clicking = false;
      const tryClick = () => {
        if (clicking) {
          return false;
        }
        const component = document.querySelector("mod-file-download");
        if (component) {
          clicking = true;
          setStatus("找到 Nexus 下载组件，正在触发 slowDownload 事件...");
          chrome.runtime.sendMessage({ type: "TRIGGER_NEXUS_SLOW_DOWNLOAD" })
            .then((response) => {
              if (!response || !response.ok) {
                throw new Error(response && response.error ? response.error : "slowDownload event failed");
              }
              resolve(true);
            })
            .catch(() => {
              component.dispatchEvent(new CustomEvent("slowDownload", { bubbles: true, composed: true }));
              resolve(true);
            });
          return true;
        }
        const button = findSlowDownloadButton();
        if (button) {
          clicking = true;
          setStatus("找到 Slow download，正在用扩展权限触发...");
          chrome.runtime.sendMessage({ type: "TRIGGER_NEXUS_SLOW_DOWNLOAD" })
            .then(() => resolve(true))
            .catch(() => {
              dispatchExtensionClick(button)
                .then(() => resolve(true))
                .catch(() => {
                  dispatchMouseLikeClick(button);
                  resolve(true);
                });
            });
          return true;
        }
        if (Date.now() > deadline) {
          resolve(false);
          return true;
        }
        return false;
      };
      if (tryClick()) {
        return;
      }
      observer = new MutationObserver(() => {
        if (tryClick() && observer) {
          observer.disconnect();
          observer = null;
        }
      });
      observer.observe(document.documentElement, { childList: true, subtree: true });
      window.setTimeout(() => {
        if (observer) {
          observer.disconnect();
          observer = null;
          resolve(false);
        }
      }, 31000);
    });
  }

  function clickManualDownloadWhenReady() {
    return new Promise((resolve) => {
      const deadline = Date.now() + 30000;
      let clicking = false;
      const tryClick = () => {
        if (clicking) {
          return false;
        }
        const button = findManualDownloadButton();
        if (button) {
          clicking = true;
          setStatus("已找到 Manual download，正在进入下载页...");
          const href = elementHref(button);
          if (href && navigateWithCurrentAnxiParams(href)) {
            window.setTimeout(() => {
              void clickAdditionalFilesDownloadIfPresent();
            }, 500);
            resolve(true);
            return true;
          }
          chrome.runtime.sendMessage({ type: "TRIGGER_NEXUS_MANUAL_DOWNLOAD", params: currentAnxiParams() })
            .then(() => {
              window.setTimeout(() => {
                void clickAdditionalFilesDownloadIfPresent();
              }, 500);
              resolve(true);
            })
            .catch(() => {
              dispatchExtensionClick(button)
                .then(() => {
                  window.setTimeout(() => {
                    void clickAdditionalFilesDownloadIfPresent();
                  }, 500);
                  resolve(true);
                })
                .catch(() => {
                  dispatchMouseLikeClick(button);
                  window.setTimeout(() => {
                    void clickAdditionalFilesDownloadIfPresent();
                  }, 500);
                  resolve(true);
                });
            });
          return true;
        }
        if (Date.now() > deadline) {
          resolve(false);
          return true;
        }
        return false;
      };
      if (tryClick()) {
        return;
      }
      observer = new MutationObserver(() => {
        if (tryClick() && observer) {
          observer.disconnect();
          observer = null;
        }
      });
      observer.observe(document.documentElement, { childList: true, subtree: true });
      window.setTimeout(() => {
        if (observer) {
          observer.disconnect();
          observer = null;
          resolve(false);
        }
      }, 31000);
    });
  }

  function removeOverlay() {
    if (overlay) {
      overlay.remove();
      overlay = null;
      statusNode = null;
      submitButton = null;
    }
  }

  function resetForCurrentUrl() {
    const nextInfo = parseNexusPageUrl(window.location.href);
    if (!nextInfo) {
      return;
    }
    if (window.location.href === currentUrl && pageInfo && nextInfo.fileId === pageInfo.fileId && nextInfo.modId === pageInfo.modId) {
      return;
    }
    currentUrl = window.location.href;
    pageInfo = nextInfo;
    hasStarted = false;
    pendingDownloadUrl = "";
    submitting = false;
    if (observer) {
      observer.disconnect();
      observer = null;
    }
    removeOverlay();
    createOverlay();
    if (config.autoStartOnNexusFilePage && (pageInfo.fileId || hasAutoFlag())) {
      window.setTimeout(() => {
        if (!hasStarted) {
          void beginCapture(Boolean(config.autoClickSlowDownload));
        }
      }, 900);
    }
  }

  function watchUrlChanges() {
    const originalPushState = history.pushState;
    const originalReplaceState = history.replaceState;
    history.pushState = function pushState(...args) {
      const result = originalPushState.apply(this, args);
      window.setTimeout(resetForCurrentUrl, 0);
      return result;
    };
    history.replaceState = function replaceState(...args) {
      const result = originalReplaceState.apply(this, args);
      window.setTimeout(resetForCurrentUrl, 0);
      return result;
    };
    window.addEventListener("popstate", () => window.setTimeout(resetForCurrentUrl, 0));
    window.setInterval(resetForCurrentUrl, 1200);
    window.setInterval(() => {
      void clickAdditionalFilesDownloadIfPresent();
    }, 700);
  }

  async function init() {
    const response = await chrome.runtime.sendMessage({ type: "GET_CONFIG" });
    if (response && response.ok && response.config) {
      config = { ...DEFAULT_CONFIG, ...response.config };
    }
    createOverlay();
    watchUrlChanges();
    chrome.runtime.onMessage.addListener((message) => {
      if (message && message.type === "CAPTURED_URL_READY") {
        if (message.captureKey || message.autoSubmit) {
          const known = batchParams();
          const captureKey = String(message.captureKey || known.captureKey || "");
          const parts = captureKey.includes(":") ? captureKey.split(":", 2) : [known.batchId, known.itemId];
          rememberAutomationParams({
            batchId: parts[0] || known.batchId,
            itemId: parts[1] || known.itemId,
            captureKey,
            autoSubmit: Boolean(message.autoSubmit || known.autoSubmit)
          });
        }
        pendingDownloadUrl = BACKGROUND_PENDING_URL;
        if (message.autoSubmit || batchParams().autoSubmit) {
          setStatus("ZIP 链接已获取，正在提交到面板...");
          void submitCapturedUrl();
          return;
        }
        setStatus("ZIP 链接已获取");
        setSubmitEnabled(true);
      }
    });
    if (config.autoStartOnNexusFilePage && (pageInfo.fileId || hasAutoFlag()) && !hasStarted) {
      window.setTimeout(() => {
        void beginCapture(Boolean(config.autoClickSlowDownload));
      }, 900);
    }
  }

  void init();
})();

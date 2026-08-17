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

  // deepQueryAll matches a selector across the whole document AND every open
  // shadow root. Nexus's redesigned mod page renders the download controls
  // (Vortex / Manual buttons, file links) inside Web Components, so a plain
  // document.querySelectorAll cannot see them.
  function deepQueryAll(selector) {
    const results = [];
    const visit = (root) => {
      let matches = [];
      try {
        matches = Array.from(root.querySelectorAll(selector));
      } catch {
        matches = [];
      }
      for (const node of matches) {
        results.push(node);
      }
      let hosts = [];
      try {
        hosts = Array.from(root.querySelectorAll("*"));
      } catch {
        hosts = [];
      }
      for (const host of hosts) {
        if (host.shadowRoot) {
          visit(host.shadowRoot);
        }
      }
    };
    visit(document);
    return results;
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

  // nodeLabel returns the accessible label for a control, falling back to
  // aria-label/title when the button is icon-only (no text node).
  function nodeLabel(node) {
    const text = node instanceof HTMLInputElement ? node.value : textOf(node);
    if (text) {
      return text;
    }
    if (node && typeof node.getAttribute === "function") {
      return (node.getAttribute("aria-label") || node.getAttribute("title") || "").replace(/\s+/g, " ").trim();
    }
    return "";
  }

  // The current Nexus mod page splits the flow into two controls:
  //   1. a short "Manual" button in the header that opens a "Download mod file"
  //      modal (isShortManualLabel), and
  //   2. a real "Manual download" button — the old header button on legacy
  //      layouts, and the button inside that modal — that performs the actual
  //      download/navigation (isManualDownloadLabel).
  // We click (1) to open the modal, then (2) to proceed.
  function isManualDownloadLabel(label) {
    return /manual\s+download/i.test(label.replace(/\s+/g, " ").trim());
  }

  function isShortManualLabel(label) {
    return /^manual$/i.test(label.replace(/\s+/g, " ").trim());
  }

  function findLabeledControl(predicate) {
    const candidates = deepQueryAll("button, a, [role='button'], input[type='button'], input[type='submit']");
    const matches = candidates.filter((node) => isVisible(node) && predicate(nodeLabel(node)));
    matches.sort((a, b) => {
      const aRect = a.getBoundingClientRect();
      const bRect = b.getBoundingClientRect();
      return (aRect.top - bRect.top) || ((bRect.width * bRect.height) - (aRect.width * aRect.height));
    });
    return matches[0] || null;
  }

  // controlLabel merges visible text with tracking/aria metadata so a control
  // can be classified even when its text is icon-only, wrapped in nested spans,
  // or padded with screen-reader-only words.
  function controlLabel(node) {
    const parts = [nodeLabel(node)];
    if (node && typeof node.getAttribute === "function") {
      parts.push(node.getAttribute("data-tracking") || "");
      parts.push(node.getAttribute("aria-label") || "");
      parts.push(node.getAttribute("title") || "");
    }
    return parts.join(" ").replace(/\s+/g, " ").trim().toLowerCase();
  }

  // findManualDownloadControl locates the manual-download control the way the
  // working community userscripts do: Nexus tags every download control with a
  // `data-tracking` value containing "Download". The manual one mentions
  // "manual"; the mod-manager one mentions "vortex"/"mod manager". This is far
  // more robust than matching the visible button text, which changed to a bare
  // "Manual" and can carry hidden words. Falls back to text matching for
  // legacy layouts.
  function findManualDownloadControl() {
    const controls = deepQueryAll('a[data-tracking*="Download" i], button[data-tracking*="Download" i]');
    const manual = controls.filter((node) => {
      if (!isVisible(node)) {
        return false;
      }
      const label = controlLabel(node);
      return /manual/.test(label) && !/vortex|mod\s*manager/.test(label);
    });
    // Prefer a control whose href already carries a file id (the real per-file
    // link) over the header opener that only launches the modal.
    manual.sort((a, b) => {
      const aHasFile = fileIdFromUrl(elementHref(a)) ? 0 : 1;
      const bHasFile = fileIdFromUrl(elementHref(b)) ? 0 : 1;
      if (aHasFile !== bHasFile) {
        return aHasFile - bHasFile;
      }
      return a.getBoundingClientRect().top - b.getBoundingClientRect().top;
    });
    if (manual[0]) {
      return manual[0];
    }
    return findLabeledControl((label) => isManualDownloadLabel(label) || isShortManualLabel(label));
  }

  // openNexusFileList reveals the file id, which Nexus loads lazily. It clicks
  // the manual-download control (opens the "Download mod file" modal / loads the
  // files list in-page); if no control is found and we are not already on the
  // files tab, it navigates there, where the list loads via AJAX.
  function openNexusFileList() {
    const control = findManualDownloadControl();
    if (control) {
      setStatus("正在打开 Nexus 文件列表...");
      dispatchExtensionClick(control).catch(() => dispatchMouseLikeClick(control));
      return true;
    }
    if (!/[?&]tab=files/i.test(window.location.href) && pageInfo.gameDomain && pageInfo.modId) {
      const filesUrl = `${window.location.origin}/${pageInfo.gameDomain}/mods/${pageInfo.modId}?tab=files`;
      setStatus("正在打开 Nexus 文件页...");
      return navigateWithCurrentAnxiParams(filesUrl);
    }
    return false;
  }

  // waitForFileIdOnPage polls (and observes) the DOM until the lazily-loaded
  // file id appears, resolving 0 on timeout.
  function waitForFileIdOnPage(timeoutMs, expectedVersion = "") {
    return new Promise((resolve) => {
      const deadline = Date.now() + timeoutMs;
      let obs = null;
      const cleanup = () => {
        window.clearInterval(interval);
        if (obs) {
          obs.disconnect();
          obs = null;
        }
      };
      const tick = () => {
        const id = findFileIdOnPage(expectedVersion);
        if (id) {
          cleanup();
          resolve(id);
          return;
        }
        if (Date.now() > deadline) {
          cleanup();
          resolve(0);
        }
      };
      const interval = window.setInterval(tick, 500);
      obs = new MutationObserver(tick);
      obs.observe(document.documentElement, { childList: true, subtree: true });
      tick();
    });
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
    for (const key of ["anxi_auto", "anxi_auto_submit", "anxi_batch", "anxi_item", "anxi_version"]) {
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
    if (batch.expectedVersion) {
      target.searchParams.set("anxi_version", batch.expectedVersion);
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

  function fileIdFromUrl(rawUrl) {
    try {
      const url = new URL(rawUrl, window.location.href);
      const raw = url.searchParams.get("file_id") || url.searchParams.get("file");
      const id = Number(raw);
      return Number.isInteger(id) && id > 0 ? id : 0;
    } catch {
      return 0;
    }
  }

  // urlIsForCurrentMod guards against picking up a file id that belongs to a
  // different mod linked on the page (e.g. a required dependency like SMAPI).
  function urlIsForCurrentMod(rawUrl) {
    try {
      const url = new URL(rawUrl, window.location.href);
      const match = url.pathname.match(/^\/[^/]+\/mods\/(\d+)/i);
      return !!match && Number(match[1]) === pageInfo.modId;
    } catch {
      return false;
    }
  }

  function fileIdsWithin(node) {
    const ids = new Set();
    const addNode = (candidate) => {
      if (!(candidate instanceof Element)) {
        return;
      }
      const href = elementHref(candidate);
      const hrefID = href && urlIsForCurrentMod(href) ? fileIdFromUrl(href) : 0;
      if (hrefID) {
        ids.add(hrefID);
      }
      for (const attr of ["data-id", "data-file-id", "data-fileid", "file-id", "fileid"]) {
        const id = Number(candidate.getAttribute(attr));
        if (Number.isInteger(id) && id > 0) {
          ids.add(id);
        }
      }
    };
    addNode(node);
    if (node && typeof node.querySelectorAll === "function") {
      for (const child of node.querySelectorAll("a[href*='file_id='], a[href*='file='], dd[data-id], [data-file-id], [data-fileid], mod-file-download")) {
        addNode(child);
      }
    }
    return ids;
  }

  function parentElementAcrossShadow(node) {
    if (node && node.parentElement) {
      return node.parentElement;
    }
    const root = node && typeof node.getRootNode === "function" ? node.getRootNode() : null;
    return root && root.host instanceof Element ? root.host : null;
  }

  function directFileId(node) {
    if (!(node instanceof Element)) {
      return 0;
    }
    const href = elementHref(node);
    const hrefID = href && urlIsForCurrentMod(href) ? fileIdFromUrl(href) : 0;
    if (hrefID) {
      return hrefID;
    }
    for (const attr of ["data-id", "data-file-id", "data-fileid", "file-id", "fileid"]) {
      const id = Number(node.getAttribute(attr));
      if (Number.isInteger(id) && id > 0) {
        return id;
      }
    }
    return 0;
  }

  function canonicalFileCandidateNode(node, fileId) {
    let current = node instanceof Element ? node : null;
    for (let depth = 0; current && depth < 10; depth += 1) {
      const tag = current.tagName.toLowerCase();
      const ownsFileID = directFileId(current) === fileId;
      if (ownsFileID && (
        tag === "dd" ||
        tag === "mod-file-download" ||
        current.hasAttribute("data-file-id") ||
        current.hasAttribute("data-fileid")
      )) {
        return current;
      }
      current = parentElementAcrossShadow(current);
    }
    return node;
  }

  function isNexusFileRowBoundary(node) {
    if (!(node instanceof Element)) {
      return false;
    }
    const tag = node.tagName.toLowerCase();
    if (["li", "tr", "article"].includes(tag)) {
      return true;
    }
    if (tag === "dd" && node.hasAttribute("data-id")) {
      return true;
    }
    if (node.hasAttribute("data-file-id") || node.hasAttribute("data-fileid")) {
      return true;
    }
    return /file[-_\s]*(expander|row|card|item)|mod[-_\s]*file/i.test(String(node.className || ""));
  }

  // Collect text only while an ancestor still belongs to one Nexus file. This
  // prevents an old file row from inheriting the latest version text from the
  // surrounding list, which is what made the previous "first file_id wins"
  // behavior select stale archives on pages containing hidden duplicate rows.
  function fileCandidateContext(node, fileId) {
    const parts = [];
    let visible = false;
    let current = canonicalFileCandidateNode(node, fileId);
    if (current && current.tagName.toLowerCase() === "dd") {
      const label = current.previousElementSibling;
      if (label && label.tagName.toLowerCase() === "dt") {
        visible = visible || isVisible(label);
        parts.push(textOf(label));
        for (const attr of ["data-version", "data-file-version", "aria-label", "title"]) {
          parts.push(label.getAttribute(attr) || "");
        }
      }
    }
    for (let depth = 0; current && depth < 10; depth += 1) {
      const nestedIDs = fileIdsWithin(current);
      if (Array.from(nestedIDs).some((id) => id !== fileId)) {
        break;
      }
      visible = visible || isVisible(current);
      parts.push(textOf(current));
      for (const attr of ["data-version", "data-file-version", "aria-label", "title"]) {
        parts.push(current.getAttribute(attr) || "");
      }
      if (isNexusFileRowBoundary(current)) {
        break;
      }
      current = parentElementAcrossShadow(current);
    }
    return { contextText: parts.join(" ").replace(/\s+/g, " ").trim(), visible };
  }

  // findFileIdOnPage recovers every current-mod file candidate and, when an
  // expected version is known, returns only a row whose own text identifies
  // that exact version. An unmatched version returns 0 instead of silently
  // falling back to an older file.
  function findFileIdOnPage(expectedVersion = "") {
    const candidates = [];
    let order = 0;
    const addCandidate = (fileId, node) => {
      if (!Number.isInteger(fileId) || fileId <= 0 || !(node instanceof Element)) {
        return;
      }
      const context = fileCandidateContext(node, fileId);
      candidates.push({ fileId, node, order: order++, ...context });
    };

    for (const link of deepQueryAll("a[href]")) {
      const id = fileIdFromUrl(link.href);
      if (id && urlIsForCurrentMod(link.href)) {
        addCandidate(id, link);
      }
    }
    for (const node of deepQueryAll("dd[data-id], [data-file-id], [data-fileid], mod-file-download")) {
      for (const attr of ["data-id", "data-file-id", "data-fileid", "file-id", "fileid"]) {
        const id = Number(node.getAttribute && node.getAttribute(attr));
        if (Number.isInteger(id) && id > 0) {
          addCandidate(id, node);
          break;
        }
      }
    }

    const selected = chooseNexusFileCandidate(candidates, expectedVersion);
    return selected ? Number(selected.fileId) : 0;
  }

  function findCurrentNexusVersion() {
    for (const node of deepQueryAll("[data-version], [data-file-version]")) {
      for (const attr of ["data-version", "data-file-version"]) {
        const value = normalizeNexusExpectedVersion(node.getAttribute && node.getAttribute(attr));
        if (value) {
          return value;
        }
      }
    }
    const bodyText = textOf(document.body);
    const match = bodyText.match(/\bversion\s*:?\s*v?([0-9]+(?:\.[0-9a-z][0-9a-z+.-]*)+)/i);
    return normalizeNexusExpectedVersion(match && match[1]);
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
    return { batchId: "", itemId: "", captureKey: "", autoSubmit: false, expectedVersion: "" };
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
        autoSubmit: Boolean(parsed.autoSubmit),
        expectedVersion: normalizeNexusExpectedVersion(parsed.expectedVersion)
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
        expectedVersion: normalizeNexusExpectedVersion(params.expectedVersion),
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
      const remembered = readAutomationParams();
      const current = mergeNexusAutomationParams({
        batchId,
        itemId,
        autoSubmit: url.searchParams.get("anxi_auto_submit") === "1",
        expectedVersion: url.searchParams.get("anxi_version")
      }, remembered);
      if (current.autoSubmit || current.batchId || current.itemId) {
        rememberAutomationParams(current);
        return current;
      }
      return remembered;
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
    const initialBatch = batchParams();
    const batch = {
      ...initialBatch,
      expectedVersion: normalizeNexusExpectedVersion(initialBatch.expectedVersion || findCurrentNexusVersion())
    };
    rememberAutomationParams(batch);
    if (batch.autoSubmit && !batch.expectedVersion) {
      await reportCaptureFailure("Nexus 页面没有提供最新版本号，已停止自动安装");
      return;
    }
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
        expectedVersion: batch.expectedVersion,
        closeTabOnComplete: batch.autoSubmit
      }
    });

    // Some legacy Nexus files land directly on the free-download interstitial.
    // In that state the correct file id is already known, but trying to
    // regenerate the URL first can fall back to the same manual-download link
    // forever. Trigger the visible slow-download control before entering the
    // file-discovery path again.
    if (clickSlow && (document.querySelector("mod-file-download") || findSlowDownloadButton())) {
      setStatus("已进入 Nexus 慢速下载页，正在自动触发 Slow download...");
      const clicked = await clickSlowDownloadWhenReady();
      if (!clicked) {
        await reportCaptureFailure("已进入 Nexus 慢速下载页，但没有找到 Slow download 按钮");
      }
      return;
    }

    if (!pageInfo.fileId || batch.expectedVersion) {
      // Preferred path: recover the file id and build the download link
      // directly via generateNexusDownloadUrl, skipping the fragile button/
      // navigation flow. Nexus loads the file id lazily (files tab / modal),
      // so when it isn't present yet we open the file list and poll for it.
      setStatus(batch.expectedVersion ? `正在识别最新版本 v${batch.expectedVersion}...` : "正在识别 Nexus 文件...");
      let fileId = findFileIdOnPage(batch.expectedVersion);
      if (!fileId) {
        openNexusFileList();
        fileId = await waitForFileIdOnPage(20000, batch.expectedVersion);
      }
      if (fileId) {
        pageInfo = { ...pageInfo, fileId };
        setStatus(batch.expectedVersion
          ? `已锁定 v${batch.expectedVersion}（file_id=${fileId}），正在获取 ZIP 链接...`
          : "已识别 Nexus 文件，正在直接获取 ZIP 链接...");
        const directArchive = findDirectArchiveLink();
        if (directArchive) {
          await captureUrl(directArchive);
          return;
        }
        try {
          const generatedUrl = await generateNexusDownloadUrl();
          await captureUrl(generatedUrl);
          return;
        } catch (error) {
          const message = error && error.message ? error.message : String(error);
          setStatus(`直接获取失败，尝试点击下载入口：${message}`);
        }
      } else if (batch.expectedVersion) {
        await reportCaptureFailure(`没有找到版本 v${batch.expectedVersion} 对应的 Nexus 文件，未下载其它版本`);
        return;
      }

      // Last resort: click through the manual-download control / modal.
      const clicked = await clickManualDownloadWhenReady();
      if (!clicked) {
        setStatus("未找到下载入口，请刷新页面后重试。");
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

  async function reportCaptureFailure(message) {
    setStatus(message);
    const batch = batchParams();
    if (!batch.batchId || !batch.itemId) {
      return;
    }
    await chrome.runtime.sendMessage({
      type: "CAPTURE_FAILED",
      batchId: batch.batchId,
      itemId: batch.itemId,
      message
    });
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
      autoSubmit: batch.autoSubmit,
      expectedVersion: batch.expectedVersion,
      fileId: Number(pageInfo.fileId || 0)
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
            autoSubmit: batch.autoSubmit,
            expectedVersion: batch.expectedVersion,
            fileId: Number(pageInfo.fileId || 0)
          }
        : {
            type: "SUBMIT_CAPTURED_URL",
            url: pendingDownloadUrl,
            captureKey: batch.captureKey,
            batchId: batch.batchId,
            itemId: batch.itemId,
            autoSubmit: batch.autoSubmit,
            expectedVersion: batch.expectedVersion,
            fileId: Number(pageInfo.fileId || 0)
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
      let lastShortClickAt = 0;
      const tryClick = () => {
        if (clicking) {
          return false;
        }
        const control = findManualDownloadControl();
        if (control) {
          const href = elementHref(control);
          // A control that points at a specific file is the real download link.
          if (fileIdFromUrl(href)) {
            clicking = true;
            setStatus("已找到下载链接，正在进入下载页...");
            if (navigateWithCurrentAnxiParams(href)) {
              window.setTimeout(() => {
                void clickAdditionalFilesDownloadIfPresent();
              }, 500);
              resolve(true);
              return true;
            }
            dispatchExtensionClick(control)
              .catch(() => dispatchMouseLikeClick(control))
              .finally(() => {
                window.setTimeout(() => {
                  void clickAdditionalFilesDownloadIfPresent();
                }, 500);
                resolve(true);
              });
            return true;
          }
          // Otherwise it's the header opener: click it (throttled) to reveal the
          // file list / modal, then keep observing for the real download link.
          if (Date.now() - lastShortClickAt > 4000) {
            lastShortClickAt = Date.now();
            setStatus("正在打开 Nexus 文件列表...");
            dispatchExtensionClick(control).catch(() => dispatchMouseLikeClick(control));
          }
          return false;
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
            autoSubmit: Boolean(message.autoSubmit || known.autoSubmit),
            expectedVersion: known.expectedVersion
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

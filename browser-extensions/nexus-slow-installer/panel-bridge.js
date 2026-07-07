(() => {
  const PANEL_SOURCE = "ANXI_PANEL_NEXUS_INSTALL";
  const EXTENSION_SOURCE = "ANXI_NEXUS_INSTALLER";
  let config = DEFAULT_CONFIG;
  let allowed = false;
  let initError = "";

  function sameOriginAsPanel(panelBaseUrl) {
    try {
      const panelUrl = new URL(normalizePanelBaseUrl(panelBaseUrl));
      const pageUrl = new URL(window.location.href);
      if (panelUrl.origin === pageUrl.origin) {
        return true;
      }
      const loopbackHosts = new Set(["localhost", "127.0.0.1", "::1"]);
      return loopbackHosts.has(panelUrl.hostname) && loopbackHosts.has(pageUrl.hostname);
    } catch {
      return false;
    }
  }

  function panelRemoteInstallEndpoint(instanceId) {
    const resolvedInstanceId = encodeURIComponent(instanceId || DEFAULT_CONFIG.instanceId);
    return `/api/instances/${resolvedInstanceId}/mods/remote/install`;
  }

  async function verifyPanelPage() {
    const response = await fetch("/api/auth/me", {
      method: "GET",
      credentials: "include",
      headers: {
        "X-Anxi-Nexus-Installer": "0.1.1"
      }
    });
    if (!response.ok) {
      throw new Error(`panel auth check returned HTTP ${response.status}`);
    }
    const body = await response.json();
    if (!body || !body.user || typeof body.user.username !== "string") {
      throw new Error("current page did not return a panel user");
    }
    return body.user;
  }

  async function registerCurrentPanel(instanceId) {
    await verifyPanelPage();
    const response = await chrome.runtime.sendMessage({
      type: "REGISTER_PANEL",
      panelBaseUrl: window.location.origin,
      instanceId: instanceId || DEFAULT_CONFIG.instanceId
    });
    if (!response || !response.ok) {
      throw new Error(response && response.error ? response.error : "extension did not save panel address");
    }
    return response.config || null;
  }

  async function fetchWithTimeout(url, options, timeoutMs) {
    const controller = new AbortController();
    const timer = window.setTimeout(() => controller.abort(), timeoutMs);
    try {
      return await fetch(url, { ...options, signal: controller.signal });
    } catch (error) {
      if (error && error.name === "AbortError") {
        throw new Error(`panel request timed out after ${Math.round(timeoutMs / 1000)} seconds`);
      }
      throw error;
    } finally {
      window.clearTimeout(timer);
    }
  }

  async function postRemoteInstallFromPanel(payload) {
    const url = String((payload && payload.url) || "").trim();
    if (!isNexusArchiveDownloadUrl(url)) {
      throw new Error("missing Nexus CDN ZIP download url");
    }
    const response = await fetchWithTimeout(panelRemoteInstallEndpoint(payload && payload.instanceId), {
      method: "POST",
      credentials: "include",
      headers: {
        "Content-Type": "application/json",
        "X-Anxi-Nexus-Installer": "0.1.1"
      },
      body: JSON.stringify({
        url,
        mod: payload && payload.mod ? payload.mod : undefined
      })
    }, 30000);

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
        : `panel returned HTTP ${response.status}`;
      const code = body && body.error && body.error.code ? ` (${body.error.code})` : "";
      throw new Error(`${message}${code}`);
    }

    return body || {};
  }

  async function sendToBackground(type, data) {
    if (type === "PING") {
      const instanceId = data.instanceId || (data.payload && data.payload.instanceId) || DEFAULT_CONFIG.instanceId;
      const nextConfig = await registerCurrentPanel(instanceId);
      config = nextConfig ? { ...DEFAULT_CONFIG, ...nextConfig } : { ...config, panelBaseUrl: window.location.origin, instanceId };
      allowed = sameOriginAsPanel(config.panelBaseUrl);
      if (!allowed) {
        throw new Error(`panel origin mismatch after registration: page=${window.location.origin}, extension=${normalizePanelBaseUrl(config.panelBaseUrl)}`);
      }
      initError = "";
      const stateResponse = await chrome.runtime.sendMessage({ type: "GET_CONFIG" });
      return {
        ok: true,
        config,
        state: stateResponse && stateResponse.state
      };
    }
    if (type === "START_BATCH_INSTALL") {
      return chrome.runtime.sendMessage({ type, payload: data.payload || {} });
    }
    if (type === "GET_BATCH_STATUS") {
      return chrome.runtime.sendMessage({ type, batchId: data.batchId || "" });
    }
    if (type === "CLEAR_STATE") {
      return chrome.runtime.sendMessage({ type });
    }
    return { ok: false, error: "unsupported_message" };
  }

  async function init() {
    try {
      const configResponse = await chrome.runtime.sendMessage({ type: "GET_CONFIG" });
      config = configResponse && configResponse.config ? configResponse.config : DEFAULT_CONFIG;
      allowed = sameOriginAsPanel(config.panelBaseUrl);
      if (!allowed) {
        initError = `panel origin mismatch: page=${window.location.origin}, extension=${normalizePanelBaseUrl(config.panelBaseUrl)}`;
      }
    } catch (error) {
      initError = error && error.message ? error.message : String(error);
    }

    window.addEventListener("message", (event) => {
      if (event.source !== window || event.origin !== window.location.origin) {
        return;
      }
      const data = event.data || {};
      if (data.source !== PANEL_SOURCE || !data.type || !data.requestId) {
        return;
      }
      const forward = () => {
        void sendToBackground(data.type, data)
          .then((response) => {
            window.postMessage({
              source: EXTENSION_SOURCE,
              type: `${data.type}_RESULT`,
              requestId: data.requestId,
              ok: Boolean(response && response.ok),
              batch: response && response.batch,
              config: response && response.config,
              state: response && response.state,
              error: response && response.error ? response.error : ""
            }, window.location.origin);
          })
          .catch((error) => {
            window.postMessage({
              source: EXTENSION_SOURCE,
              type: `${data.type}_RESULT`,
              requestId: data.requestId,
              ok: false,
              error: error && error.message ? error.message : String(error)
            }, window.location.origin);
          });
      };

      if (data.type === "PING") {
        forward();
        return;
      }

      if (!allowed) {
        window.postMessage({
          source: EXTENSION_SOURCE,
          type: `${data.type}_RESULT`,
          requestId: data.requestId,
          ok: false,
          error: initError || "panel bridge is not allowed on this page"
        }, window.location.origin);
        return;
      }

      forward();
    });

    chrome.runtime.onMessage.addListener((message, sender, sendResponse) => {
      if (!allowed) {
        if (message && message.type === "PANEL_REMOTE_INSTALL") {
          sendResponse({ ok: false, error: initError || "panel bridge is not allowed on this page" });
        }
        return false;
      }
      if (message && message.type === "PANEL_REMOTE_INSTALL") {
        void postRemoteInstallFromPanel(message.payload || {})
          .then((result) => sendResponse({ ok: true, result }))
          .catch((error) => sendResponse({
            ok: false,
            error: error && error.message ? error.message : String(error)
          }));
        return true;
      }
      if (message && message.type === "BATCH_STATUS_UPDATE") {
        window.postMessage({
          source: EXTENSION_SOURCE,
          type: "BATCH_STATUS_UPDATE",
          batch: message.batch || null
        }, window.location.origin);
      }
      return false;
    });

    window.postMessage({
      source: EXTENSION_SOURCE,
      type: "BRIDGE_READY",
      ok: allowed,
      error: initError
    }, window.location.origin);
  }

  void init();
})();

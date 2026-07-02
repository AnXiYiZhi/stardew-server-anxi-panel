const fields = {
  panelBaseUrl: document.getElementById("panelBaseUrl"),
  instanceId: document.getElementById("instanceId"),
  autoStartOnNexusFilePage: document.getElementById("autoStartOnNexusFilePage"),
  autoClickSlowDownload: document.getElementById("autoClickSlowDownload"),
  cancelBrowserDownload: document.getElementById("cancelBrowserDownload"),
  statusText: document.getElementById("statusText")
};

function formConfig() {
  return {
    panelBaseUrl: fields.panelBaseUrl.value,
    instanceId: fields.instanceId.value,
    autoStartOnNexusFilePage: fields.autoStartOnNexusFilePage.checked,
    autoClickSlowDownload: fields.autoClickSlowDownload.checked,
    cancelBrowserDownload: fields.cancelBrowserDownload.checked
  };
}

function fillConfig(config) {
  fields.panelBaseUrl.value = config.panelBaseUrl || "";
  fields.instanceId.value = config.instanceId || "stardew";
  fields.autoStartOnNexusFilePage.checked = Boolean(config.autoStartOnNexusFilePage);
  fields.autoClickSlowDownload.checked = Boolean(config.autoClickSlowDownload);
  fields.cancelBrowserDownload.checked = Boolean(config.cancelBrowserDownload);
}

function showState(state) {
  const install = state && state.lastInstall;
  if (!install) {
    fields.statusText.textContent = "还没有捕获记录。";
    return;
  }
  const time = install.updatedAt ? new Date(install.updatedAt).toLocaleTimeString() : "";
  fields.statusText.textContent = `${install.status}: ${install.message}${time ? ` (${time})` : ""}`;
}

async function load() {
  const response = await chrome.runtime.sendMessage({ type: "GET_CONFIG" });
  if (response && response.ok) {
    fillConfig(response.config);
    showState(response.state);
  }
}

document.getElementById("save").addEventListener("click", async () => {
  const response = await chrome.runtime.sendMessage({ type: "SAVE_CONFIG", config: formConfig() });
  if (response && response.ok) {
    fillConfig(response.config);
    fields.statusText.textContent = "设置已保存。";
  } else {
    fields.statusText.textContent = response && response.error ? response.error : "保存失败。";
  }
});

document.getElementById("captureCurrent").addEventListener("click", async () => {
  const tabs = await chrome.tabs.query({ active: true, currentWindow: true });
  const tab = tabs[0];
  const info = tab && tab.url ? parseNexusPageUrl(tab.url) : null;
  if (!tab || !info) {
    fields.statusText.textContent = "当前标签页不是 Nexus Mod 文件页。";
    return;
  }
  const response = await chrome.runtime.sendMessage({ type: "START_CAPTURE", payload: { ...info, tabId: tab.id } });
  fields.statusText.textContent = response && response.ok ? "已开始捕获，请在页面点击 Slow download。" : "开始捕获失败。";
});

void load();

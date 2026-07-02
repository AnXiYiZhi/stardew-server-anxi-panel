const els = {
  panelBaseUrl: document.getElementById("panelBaseUrl"),
  instanceId: document.getElementById("instanceId"),
  autoStartOnNexusFilePage: document.getElementById("autoStartOnNexusFilePage"),
  autoClickSlowDownload: document.getElementById("autoClickSlowDownload"),
  cancelBrowserDownload: document.getElementById("cancelBrowserDownload"),
  state: document.getElementById("state")
};

function collectConfig() {
  return {
    panelBaseUrl: els.panelBaseUrl.value,
    instanceId: els.instanceId.value,
    autoStartOnNexusFilePage: els.autoStartOnNexusFilePage.checked,
    autoClickSlowDownload: els.autoClickSlowDownload.checked,
    cancelBrowserDownload: els.cancelBrowserDownload.checked
  };
}

function applyConfig(config) {
  els.panelBaseUrl.value = config.panelBaseUrl || "";
  els.instanceId.value = config.instanceId || "stardew";
  els.autoStartOnNexusFilePage.checked = Boolean(config.autoStartOnNexusFilePage);
  els.autoClickSlowDownload.checked = Boolean(config.autoClickSlowDownload);
  els.cancelBrowserDownload.checked = Boolean(config.cancelBrowserDownload);
}

async function refresh() {
  const response = await chrome.runtime.sendMessage({ type: "GET_CONFIG" });
  if (response && response.ok) {
    applyConfig(response.config);
    els.state.textContent = JSON.stringify(response.state || {}, null, 2);
  }
}

document.getElementById("save").addEventListener("click", async () => {
  const response = await chrome.runtime.sendMessage({ type: "SAVE_CONFIG", config: collectConfig() });
  els.state.textContent = response && response.ok ? "设置已保存。" : `保存失败：${response && response.error ? response.error : "unknown"}`;
  await refresh();
});

document.getElementById("clear").addEventListener("click", async () => {
  await chrome.runtime.sendMessage({ type: "CLEAR_STATE" });
  await refresh();
});

void refresh();

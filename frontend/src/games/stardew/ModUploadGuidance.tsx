import './ModUploadGuidance.css'

export const MOD_UPLOAD_TOOLTIP = '支持一次选择一个或多个 ZIP，或者一个 ZIP 中包含多个 Mod 文件夹；不支持 ZIP 套 ZIP，请先解压内层 ZIP。'

export function ModUploadGuidance() {
  return (
    <div className="sd-mod-upload-guide" role="note" aria-label="Mod ZIP 上传说明">
      <div className="sd-mod-upload-guide-row sd-mod-upload-guide-row--supported">
        <span className="sd-mod-upload-guide-badge">支持</span>
        <span>一次选择一个或多个 ZIP；或者一个 ZIP 中包含多个 Mod 文件夹。</span>
      </div>
      <div className="sd-mod-upload-guide-row sd-mod-upload-guide-row--unsupported">
        <span className="sd-mod-upload-guide-badge">不支持</span>
        <span>ZIP 中再包含 ZIP。请先解压内层 ZIP，再把它作为单独的 ZIP 上传。</span>
      </div>
    </div>
  )
}

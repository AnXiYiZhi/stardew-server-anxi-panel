# Stardew UI Extracted Assets

从 `source/reference.png` 中按区域、按钮、背景、贴图拆出的可复用 PNG 素材。

## 目录

| 目录 | 用途 |
|------|------|
| `backgrounds/` | 可平铺或作为页面背景的羊皮纸、木纹、黑色舞台背景 |
| `layout/` | 四个主界面的无字整窗壳，可直接作为 HTML 原型背景 |
| `panels/` | 表单、表格、指标卡、告警条等无字面板底 |
| `navigation/` | 左侧导航项、顶部标签、内容页标签 |
| `buttons/` | 启动、停止、重启、复制、快捷工具等无字按钮底 |
| `fields/` | 输入框、搜索框、下拉框等无字表单控件 |
| `icons/` | 导航、状态、按钮图标，已尽量透明抠底 |
| `sprites/` | 农舍、云、宝箱、树、栅栏等贴图 |

## 文件

- `manifest.json`：记录每个素材的分类、文件名、源图坐标、清理方式和说明。
- `preview.html`：浏览器预览页，可直接打开查看全部素材。
- `contact-sheet.png`：文件管理器里快速扫一眼的总览图。
- `source/reference.png`：原始参考图备份。

## 使用建议

- 按钮和输入框建议作为 CSS `background-image` 或 `<img>` 底图使用，文字和图标用 HTML 重新叠加。
- `layout/*_shell_blank.png` 是快速原型用的大背景；真实实现时更建议拆成 `backgrounds/` + `panels/` + `buttons/` 组合。
- 如果后续参考图更新，运行 `scripts/extract-ui-assets.py` 会按同一命名重新导出素材。

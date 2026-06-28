from __future__ import annotations

import json
import shutil
from dataclasses import dataclass, field
from pathlib import Path

import numpy as np
from PIL import Image, ImageDraw


ROOT = Path(__file__).resolve().parents[1]
SOURCE = Path(r"C:\Users\anxi\Desktop\ig_0f07c610938a538d016a3f9e1577b48191a322d57d3249e8ff.png")
OUT = ROOT / "docs" / "prototypes" / "assets" / "ui-extracted"


@dataclass
class Asset:
    category: str
    name: str
    box: tuple[int, int, int, int]
    kind: str
    clean: str = "raw"
    border: int = 0
    description: str = ""
    transparent: bool = False
    masks: list[tuple[int, int, int, int]] = field(default_factory=list)


def crop(src: Image.Image, box: tuple[int, int, int, int]) -> Image.Image:
    x, y, w, h = box
    return src.crop((x, y, x + w, y + h)).convert("RGBA")


def tile_fill(size: tuple[int, int], tile: Image.Image) -> Image.Image:
    out = Image.new("RGBA", size)
    tile = tile.convert("RGBA")
    for y in range(0, size[1], tile.height):
        for x in range(0, size[0], tile.width):
            out.alpha_composite(tile, (x, y))
    return out


def subtle_texture(color: tuple[int, int, int], size: tuple[int, int], seed: int) -> Image.Image:
    rng = np.random.default_rng(seed)
    base = np.zeros((size[1], size[0], 4), dtype=np.uint8)
    noise = rng.integers(-4, 5, size=(size[1], size[0], 1))
    rgb = np.clip(np.array(color, dtype=np.int16).reshape(1, 1, 3) + noise, 0, 255)
    base[:, :, :3] = rgb.astype(np.uint8)
    base[:, :, 3] = 255
    return Image.fromarray(base, "RGBA")


def median_color(img: Image.Image, rect: tuple[int, int, int, int] | None = None) -> tuple[int, int, int]:
    arr_img = img.crop(rect) if rect else img
    arr = np.asarray(arr_img.convert("RGB")).reshape(-1, 3)
    return tuple(np.median(arr, axis=0).astype(np.uint8).tolist())


def blank_inner(img: Image.Image, border: int, seed: int, texture: Image.Image | None = None) -> Image.Image:
    if border <= 0:
        return img
    out = img.copy()
    w, h = out.size
    if w <= border * 2 or h <= border * 2:
        return out
    rect = (border, border, w - border, h - border)
    if texture is None:
        color = median_color(out, (border, max(border, h // 2), w - border, h - border))
        fill = subtle_texture(color, (rect[2] - rect[0], rect[3] - rect[1]), seed)
    else:
        fill = tile_fill((rect[2] - rect[0], rect[3] - rect[1]), texture)
    out.alpha_composite(fill, rect[:2])
    return out


def fill_masks(img: Image.Image, masks: list[tuple[int, int, int, int]], texture: Image.Image) -> Image.Image:
    out = img.copy()
    for x, y, w, h in masks:
        fill = tile_fill((w, h), texture)
        out.alpha_composite(fill, (x, y))
    return out


def remove_edge_background(img: Image.Image, threshold: int = 28) -> Image.Image:
    rgba = np.asarray(img.convert("RGBA")).copy()
    rgb = rgba[:, :, :3].astype(np.int16)
    h, w = rgb.shape[:2]
    corners = np.array(
        [
            rgb[0, 0],
            rgb[0, w - 1],
            rgb[h - 1, 0],
            rgb[h - 1, w - 1],
        ],
        dtype=np.int16,
    )
    bg = np.median(corners, axis=0)
    diff = np.linalg.norm(rgb - bg, axis=2)
    candidate = diff < threshold

    # Flood-fill only connected edge background, so same-colored interior pixels survive.
    seen = np.zeros((h, w), dtype=bool)
    stack: list[tuple[int, int]] = []
    for x in range(w):
        if candidate[0, x]:
            stack.append((x, 0))
        if candidate[h - 1, x]:
            stack.append((x, h - 1))
    for y in range(h):
        if candidate[y, 0]:
            stack.append((0, y))
        if candidate[y, w - 1]:
            stack.append((w - 1, y))

    while stack:
        x, y = stack.pop()
        if x < 0 or y < 0 or x >= w or y >= h or seen[y, x] or not candidate[y, x]:
            continue
        seen[y, x] = True
        stack.extend(((x + 1, y), (x - 1, y), (x, y + 1), (x, y - 1)))

    rgba[seen, 3] = 0
    return Image.fromarray(rgba, "RGBA")


def make_preview(manifest: list[dict[str, object]]) -> None:
    groups: dict[str, list[dict[str, object]]] = {}
    for item in manifest:
        groups.setdefault(str(item["category"]), []).append(item)

    css = """
    body{margin:0;background:#14100b;color:#ead8ad;font:14px/1.45 system-ui,-apple-system,Segoe UI,sans-serif}
    main{padding:24px;max-width:1280px;margin:auto}
    h1{font-size:22px;margin:0 0 8px}
    h2{font-size:16px;margin:28px 0 12px;color:#f3c15e}
    .grid{display:grid;grid-template-columns:repeat(auto-fill,minmax(180px,1fr));gap:14px}
    figure{margin:0;background:#22180f;border:1px solid #5a3d20;border-radius:6px;padding:10px}
    .checker{min-height:72px;display:flex;align-items:center;justify-content:center;padding:8px;background-color:#2b2118;background-image:linear-gradient(45deg,#36291d 25%,transparent 25%),linear-gradient(-45deg,#36291d 25%,transparent 25%),linear-gradient(45deg,transparent 75%,#36291d 75%),linear-gradient(-45deg,transparent 75%,#36291d 75%);background-size:20px 20px;background-position:0 0,0 10px,10px -10px,-10px 0}
    img{max-width:100%;image-rendering:auto}
    figcaption{margin-top:8px;color:#f5e6c6;font-size:12px;word-break:break-all}
    small{display:block;color:#b89b66;margin-top:3px}
    """
    html = [
        "<!doctype html><html><head><meta charset='utf-8'><title>Stardew UI Extracted Assets</title>",
        f"<style>{css}</style></head><body><main>",
        "<h1>Stardew UI Extracted Assets</h1>",
        "<p>所有按钮和输入类素材已清理文字，可直接作为 HTML/CSS 背景复用。坐标来源保存在 manifest.json。</p>",
    ]
    for category, items in groups.items():
        html.append(f"<h2>{category}</h2><section class='grid'>")
        for item in items:
            rel = item["file"]
            html.append(
                "<figure><div class='checker'>"
                f"<img src='{rel}' alt='{item['name']}'>"
                "</div>"
                f"<figcaption>{item['name']}<small>{item['description']}</small></figcaption></figure>"
            )
        html.append("</section>")
    html.append("</main></body></html>")
    (OUT / "preview.html").write_text("\n".join(html), encoding="utf-8")


def main() -> None:
    src = Image.open(SOURCE).convert("RGB")
    OUT.mkdir(parents=True, exist_ok=True)
    (OUT / "source").mkdir(exist_ok=True)
    shutil.copy2(SOURCE, OUT / "source" / "reference.png")

    parchment_color = median_color(crop(src, (580, 360, 88, 82)))
    parchment = subtle_texture(parchment_color, (128, 128), 999)
    wood = crop(src, (226, 340, 96, 112)).resize((128, 128))
    frame_wood = crop(src, (348, 38, 256, 14)).resize((128, 32))

    assets: list[Asset] = [
        Asset("backgrounds", "background_parchment_tile", (580, 360, 88, 82), "texture", "parchment", description="无文字羊皮纸背景纹理，可平铺"),
        Asset("backgrounds", "background_sidebar_wood_tile", (226, 340, 96, 112), "texture", "raw", description="深色木板侧栏纹理，可平铺"),
        Asset("backgrounds", "background_frame_wood_strip", (348, 38, 256, 14), "texture", "raw", description="窗口木框横向纹理"),
        Asset("backgrounds", "background_app_black", (0, 690, 160, 120), "texture", "raw", description="原型黑色舞台背景"),
        Asset("layout", "layout_install_window_shell_blank", (215, 36, 570, 459), "shell", "masks", description="安装向导整窗空白壳", masks=[(12, 16, 105, 432), (132, 18, 424, 426)]),
        Asset("layout", "layout_overview_window_shell_blank", (796, 36, 730, 461), "shell", "masks", description="日常运行总览整窗空白壳", masks=[(12, 16, 105, 434), (132, 19, 582, 426)]),
        Asset("layout", "layout_saves_mods_window_shell_blank", (214, 510, 571, 489), "shell", "masks", description="存档与模组维护整窗空白壳", masks=[(13, 17, 105, 461), (133, 19, 424, 454)]),
        Asset("layout", "layout_health_window_shell_blank", (795, 510, 730, 489), "shell", "masks", description="诊断健康检查整窗空白壳", masks=[(13, 17, 105, 461), (133, 19, 582, 454)]),
        Asset("panels", "panel_parchment_form_blank", (353, 197, 217, 163), "panel", "inner", 4, "表单卡片空白底"),
        Asset("panels", "panel_parchment_section_blank", (923, 138, 593, 113), "panel", "inner", 3, "大区块羊皮纸面板"),
        Asset("panels", "panel_metric_card_blank", (928, 285, 87, 55), "panel", "inner", 3, "小型指标卡"),
        Asset("panels", "panel_table_area_blank", (352, 550, 383, 174), "panel", "inner", 3, "表格区域空白底"),
        Asset("panels", "panel_mod_card_blank", (356, 788, 96, 68), "panel", "inner", 2, "模组卡片空白底"),
        Asset("panels", "panel_warning_row_blank", (929, 824, 239, 35), "panel", "inner", 3, "告警条空白底"),
        Asset("navigation", "nav_item_default_blank", (224, 102, 105, 29), "nav", "inner", 5, "默认左侧导航项"),
        Asset("navigation", "nav_item_active_green_blank", (806, 102, 99, 29), "nav", "inner", 5, "激活左侧导航项"),
        Asset("navigation", "nav_item_active_saves_blank", (224, 653, 105, 28), "nav", "inner", 5, "存档页激活导航项"),
        Asset("navigation", "nav_quick_help_blank", (227, 459, 96, 28), "nav", "inner", 5, "左下快速帮助按钮"),
        Asset("navigation", "tab_top_green_blank", (215, 13, 119, 27), "tab", "inner", 4, "顶部绿色编号标签"),
        Asset("navigation", "tab_content_active_blank", (928, 548, 62, 29), "tab", "inner", 4, "内容页激活标签"),
        Asset("navigation", "tab_content_inactive_blank", (990, 548, 63, 29), "tab", "inner", 4, "内容页普通标签"),
        Asset("buttons", "button_server_start_green_blank", (929, 169, 76, 34), "button", "inner", 5, "启动服务器绿色按钮"),
        Asset("buttons", "button_server_stop_red_blank", (1011, 169, 88, 33), "button", "inner", 5, "停止服务器红色按钮"),
        Asset("buttons", "button_server_restart_tan_blank", (1104, 169, 84, 34), "button", "inner", 5, "重启服务器浅色按钮"),
        Asset("buttons", "button_next_step_green_blank", (687, 408, 64, 27), "button", "inner", 5, "下一步绿色按钮"),
        Asset("buttons", "button_primary_small_green_blank", (354, 578, 58, 24), "button", "inner", 4, "小号绿色操作按钮"),
        Asset("buttons", "button_secondary_small_tan_blank", (416, 578, 61, 24), "button", "inner", 4, "小号浅色操作按钮"),
        Asset("buttons", "button_delete_small_tan_blank", (548, 578, 46, 24), "button", "inner", 4, "小号删除操作按钮"),
        Asset("buttons", "button_quick_tool_gold_blank", (1376, 611, 122, 26), "button", "inner", 5, "右侧快捷工具金色按钮"),
        Asset("buttons", "button_health_check_brown_blank", (947, 751, 100, 22), "button", "inner", 4, "健康检查深棕按钮"),
        Asset("buttons", "button_copy_small_blank", (1421, 103, 48, 25), "button", "inner", 4, "小号复制按钮"),
        Asset("buttons", "button_menu_square_blank", (1476, 102, 33, 27), "button", "inner", 4, "方形更多菜单按钮"),
        Asset("buttons", "button_table_icon_square_blank", (1273, 396, 17, 17), "button", "inner", 3, "表格行小图标按钮"),
        Asset("fields", "field_invite_code_input_blank", (1206, 169, 185, 31), "field", "inner", 5, "邀请码输入框"),
        Asset("fields", "field_search_compact_blank", (686, 557, 75, 20), "field", "inner", 4, "紧凑搜索框"),
        Asset("fields", "field_select_dropdown_blank", (584, 335, 169, 23), "field", "inner", 4, "下拉选择框"),
        Asset("fields", "field_path_input_blank", (583, 285, 122, 24), "field", "inner", 4, "路径输入框"),
        Asset("sprites", "sprite_farmhouse_scene", (30, 120, 158, 92), "sprite", "raw", description="首屏农舍贴图，保留原背景"),
        Asset("sprites", "sprite_cloud_left", (30, 124, 31, 15), "sprite", "transparent", description="像素云朵左"),
        Asset("sprites", "sprite_cloud_right", (151, 126, 34, 14), "sprite", "transparent", description="像素云朵右"),
        Asset("sprites", "sprite_chest", (31, 608, 22, 27), "sprite", "transparent", description="宝箱小贴图"),
        Asset("sprites", "sprite_tree", (67, 606, 19, 29), "sprite", "transparent", description="树小贴图"),
        Asset("sprites", "sprite_fence", (96, 604, 31, 31), "sprite", "transparent", description="栅栏小贴图"),
        Asset("sprites", "sprite_blue_device", (133, 608, 14, 27), "sprite", "transparent", description="蓝色设备小贴图"),
        Asset("sprites", "sprite_blue_gem", (165, 607, 20, 28), "sprite", "transparent", description="蓝色宝石小贴图"),
        Asset("icons", "icon_sidebar_chicken", (225, 56, 27, 31), "icon", "transparent", description="侧栏鸡头像"),
        Asset("icons", "icon_nav_overview_home", (229, 108, 13, 13), "icon", "transparent", description="导航概览图标"),
        Asset("icons", "icon_nav_server_control", (229, 137, 13, 13), "icon", "transparent", description="导航服务器控制图标"),
        Asset("icons", "icon_nav_saves", (229, 167, 13, 13), "icon", "transparent", description="导航存档图标"),
        Asset("icons", "icon_nav_mods", (229, 197, 13, 13), "icon", "transparent", description="导航模组图标"),
        Asset("icons", "icon_nav_tasks", (229, 226, 13, 13), "icon", "transparent", description="导航任务图标"),
        Asset("icons", "icon_nav_players", (229, 257, 13, 13), "icon", "transparent", description="导航玩家图标"),
        Asset("icons", "icon_nav_settings", (229, 286, 13, 13), "icon", "transparent", description="导航设置图标"),
        Asset("icons", "icon_nav_diagnostics", (229, 317, 13, 13), "icon", "transparent", description="导航诊断图标"),
        Asset("icons", "icon_top_summary_save", (938, 109, 17, 17), "icon", "transparent", description="顶部存档摘要图标"),
        Asset("icons", "icon_top_summary_players", (1054, 109, 16, 17), "icon", "transparent", description="顶部玩家摘要图标"),
        Asset("icons", "icon_top_summary_time", (1154, 109, 16, 17), "icon", "transparent", description="顶部运行时间摘要图标"),
        Asset("icons", "icon_top_summary_version", (1246, 109, 16, 17), "icon", "transparent", description="顶部版本摘要图标"),
        Asset("icons", "icon_button_play", (943, 180, 14, 16), "icon", "transparent", description="启动播放图标"),
        Asset("icons", "icon_button_stop", (1030, 181, 13, 13), "icon", "transparent", description="停止图标"),
        Asset("icons", "icon_button_restart", (1122, 180, 14, 14), "icon", "transparent", description="重启图标"),
    ]

    manifest: list[dict[str, object]] = []
    for index, asset in enumerate(assets):
        img = crop(src, asset.box)
        if asset.clean == "parchment":
            img = parchment.copy()
        elif asset.clean == "inner":
            img = blank_inner(img, asset.border, index)
        elif asset.clean == "masks":
            tex = wood if asset.name.endswith("shell_blank") else parchment
            img = fill_masks(img, asset.masks, parchment)
            if "window_shell" in asset.name:
                # Restore the sidebar mask with wood texture after the parchment pass.
                sidebar = asset.masks[0]
                img.alpha_composite(tile_fill((sidebar[2], sidebar[3]), wood), sidebar[:2])
        elif asset.clean == "transparent" or asset.transparent:
            img = remove_edge_background(img)

        category_dir = OUT / asset.category
        category_dir.mkdir(parents=True, exist_ok=True)
        rel = f"{asset.category}/{asset.name}.png"
        img.save(OUT / rel)
        manifest.append(
            {
                "category": asset.category,
                "name": asset.name,
                "file": rel,
                "source": str(SOURCE),
                "box": {"x": asset.box[0], "y": asset.box[1], "width": asset.box[2], "height": asset.box[3]},
                "kind": asset.kind,
                "clean": asset.clean,
                "description": asset.description,
            }
        )

    (OUT / "manifest.json").write_text(json.dumps(manifest, ensure_ascii=False, indent=2), encoding="utf-8")
    make_preview(manifest)

    # A small visual index for quick file-browser inspection.
    thumbs = []
    for item in manifest:
        im = Image.open(OUT / str(item["file"])).convert("RGBA")
        im.thumbnail((128, 96), Image.LANCZOS)
        thumbs.append((str(item["name"]), im.copy()))
    cols, cell_w, cell_h = 5, 220, 150
    rows = (len(thumbs) + cols - 1) // cols
    sheet = Image.new("RGB", (cols * cell_w, rows * cell_h), (24, 18, 12))
    draw = ImageDraw.Draw(sheet)
    for i, (name, im) in enumerate(thumbs):
        x = (i % cols) * cell_w
        y = (i // cols) * cell_h
        tile = Image.new("RGBA", (cell_w, cell_h), (34, 24, 16, 255))
        tile.alpha_composite(im, ((cell_w - im.width) // 2, 12))
        sheet.paste(tile.convert("RGB"), (x, y))
        draw.text((x + 10, y + 112), name[:31], fill=(238, 218, 174))
    sheet.save(OUT / "contact-sheet.png")

    print(f"Wrote {len(manifest)} assets to {OUT}")


if __name__ == "__main__":
    main()

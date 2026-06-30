from __future__ import annotations

from pathlib import Path
from textwrap import wrap

from PIL import Image, ImageDraw, ImageFont, ImageFilter


ROOT = Path(__file__).resolve().parents[1]
ASSET = ROOT / "frontend" / "public" / "assets" / "stardew" / "ui"
OUT = ROOT / "docs" / "prototypes" / "stardew-v2-pages"

W, H = 1600, 1000
TOP = 58
SIDE = 226
RAIL = 318
MARGIN = 28

FONT_REG = Path("C:/Windows/Fonts/NotoSansSC-VF.ttf")
FONT_BOLD = Path("C:/Windows/Fonts/msyhbd.ttc")
FONT_MONO = Path("C:/Windows/Fonts/consola.ttf")


def font(size: int, bold: bool = False, mono: bool = False) -> ImageFont.FreeTypeFont:
    if mono and FONT_MONO.exists():
        return ImageFont.truetype(str(FONT_MONO), size)
    path = FONT_BOLD if bold and FONT_BOLD.exists() else FONT_REG
    return ImageFont.truetype(str(path), size)


F = {
    "tiny": font(18),
    "small": font(22),
    "small_b": font(22, True),
    "body": font(25),
    "body_b": font(25, True),
    "h3": font(30, True),
    "h2": font(38, True),
    "h1": font(52, True),
    "mono": font(25, mono=True),
    "mono_b": font(34, True, True),
}


C = {
    "ink": "#34200e",
    "ink2": "#6a4524",
    "muted": "#8c6d43",
    "paper": "#f6dfaa",
    "paper2": "#fff0c9",
    "wood": "#7a3f1c",
    "wood2": "#4a240e",
    "green": "#34833d",
    "green2": "#67b348",
    "red": "#b84232",
    "gold": "#d69224",
    "blue": "#4e91b7",
    "line": "#b88645",
    "dark": "#180d04",
}


def load(name: str, size: tuple[int, int] | None = None) -> Image.Image:
    p = ASSET / name
    im = Image.open(p).convert("RGBA")
    if size:
        im = im.resize(size, Image.Resampling.NEAREST)
    return im


def tile(canvas: Image.Image, name: str, box: tuple[int, int, int, int], alpha: int = 255) -> None:
    tex = load(name)
    if alpha != 255:
        tex.putalpha(alpha)
    x0, y0, x1, y1 = box
    for y in range(y0, y1, tex.height):
        for x in range(x0, x1, tex.width):
            canvas.alpha_composite(tex, (x, y))


def rect(d: ImageDraw.ImageDraw, xy, fill, outline=None, width=1):
    d.rectangle(xy, fill=fill, outline=outline, width=width)


def shadow_box(canvas: Image.Image, xy, fill, outline="#7a4a22", width=4, shadow=9):
    x0, y0, x1, y1 = xy
    overlay = Image.new("RGBA", canvas.size, (0, 0, 0, 0))
    od = ImageDraw.Draw(overlay)
    od.rectangle((x0 + shadow, y0 + shadow, x1 + shadow, y1 + shadow), fill=(54, 29, 10, 70))
    overlay = overlay.filter(ImageFilter.GaussianBlur(4))
    canvas.alpha_composite(overlay)
    d = ImageDraw.Draw(canvas)
    d.rectangle(xy, fill=fill, outline=outline, width=width)
    d.rectangle((x0 + 7, y0 + 7, x1 - 7, y1 - 7), outline="#ffe8b3", width=2)
    d.rectangle((x0 + 12, y0 + 12, x1 - 12, y1 - 12), outline="#c58b49", width=1)


def text(d: ImageDraw.ImageDraw, xy, s: str, fill=C["ink"], size="body", anchor=None):
    d.text(xy, s, font=F[size], fill=fill, anchor=anchor)


def fit_text(d: ImageDraw.ImageDraw, xy, s: str, max_w: int, fill=C["ink"], size="body", line_gap=6):
    x, y = xy
    lines: list[str] = []
    for para in s.splitlines() or [""]:
        current = ""
        for ch in para:
            trial = current + ch
            if d.textlength(trial, font=F[size]) <= max_w:
                current = trial
            else:
                if current:
                    lines.append(current)
                current = ch
        if current:
            lines.append(current)
    for line in lines:
        d.text((x, y), line, font=F[size], fill=fill)
        y += F[size].size + line_gap
    return y


def button(d: ImageDraw.ImageDraw, xy, label, kind="green", icon=None, disabled=False):
    x0, y0, x1, y1 = xy
    palette = {
        "green": ("#327c35", "#9dde65", "#1e4d1f", "#faffd9"),
        "tan": ("#d9aa60", "#fff0bb", "#7a4c20", "#3a210c"),
        "red": ("#a13a2c", "#f09a78", "#612018", "#fff0dc"),
        "gold": ("#d79b27", "#ffe08a", "#7a4d12", "#40220a"),
        "blue": ("#427b9d", "#9ed6e8", "#23445a", "#f2ffff"),
    }[kind]
    fill, hi, outline, fg = palette
    if disabled:
        fill, hi, outline, fg = "#b9aa8d", "#e6d9bd", "#867456", "#665b49"
    d.rectangle(xy, fill=fill, outline=outline, width=3)
    d.line((x0 + 4, y0 + 4, x1 - 4, y0 + 4), fill=hi, width=2)
    d.line((x0 + 4, y1 - 5, x1 - 4, y1 - 5), fill="#5a2b13", width=2)
    tx = (x0 + x1) // 2
    if icon:
        d.text((x0 + 16, (y0 + y1) // 2), icon, font=F["small_b"], fill=fg, anchor="lm")
        tx += 10
    d.text((tx, (y0 + y1) // 2 - 1), label, font=F["small_b"], fill=fg, anchor="mm")


def pill(d: ImageDraw.ImageDraw, xy, label, color="green"):
    x0, y0, x1, y1 = xy
    colors = {
        "green": ("#e5f5c7", "#37823a", "#235727"),
        "red": ("#ffe0d6", "#b84232", "#7b2119"),
        "gold": ("#fff1bf", "#d69224", "#6a4510"),
        "blue": ("#ddf2ff", "#4e91b7", "#24506b"),
        "gray": ("#ebe0cf", "#98856c", "#665540"),
    }[color]
    d.rounded_rectangle(xy, radius=4, fill=colors[0], outline=colors[1], width=2)
    d.ellipse((x0 + 9, y0 + 10, x0 + 21, y0 + 22), fill=colors[1])
    d.text((x0 + 29, (y0 + y1) // 2), label, font=F["small_b"], fill=colors[2], anchor="lm")


def icon(name: str, size=30) -> Image.Image:
    return load(f"icons/{name}.png", (size, size))


NAV = [
    ("总览", "icon_nav_overview_home", "overview"),
    ("服务器", "icon_nav_server_control", "server"),
    ("存档", "icon_nav_saves", "saves"),
    ("任务日志", "icon_nav_tasks", "jobs"),
    ("玩家", "icon_nav_players", "players"),
    ("模组", "icon_nav_mods", "mods"),
    ("诊断", "icon_nav_diagnostics", "diagnostics"),
    ("安装", "icon_sidebar_chicken", "install"),
    ("设置", "icon_nav_settings", "settings"),
]


def shell(active: str, title: str, subtitle: str) -> tuple[Image.Image, ImageDraw.ImageDraw]:
    im = Image.new("RGBA", (W, H), C["paper"])
    tile(im, "backgrounds/background_parchment_tile.png", (SIDE, TOP, W - RAIL, H))
    tile(im, "backgrounds/background_sidebar_wood_tile.png", (0, TOP, SIDE, H))
    tile(im, "backgrounds/background_parchment_tile.png", (W - RAIL, TOP, W, H))
    tile(im, "backgrounds/background_frame_wood_strip.png", (0, 0, W, TOP))
    d = ImageDraw.Draw(im)
    rect(d, (0, 0, W, TOP), fill=(20, 11, 4, 205), outline="#44230f", width=2)
    rect(d, (0, TOP, SIDE, H), fill=(44, 20, 7, 92), outline="#44230f", width=2)
    rect(d, (W - RAIL, TOP, W, H), fill=(246, 223, 170, 166), outline="#9b6932", width=2)

    chick = icon("icon_sidebar_chicken", 34)
    im.alpha_composite(chick, (18, 12))
    text(d, (62, 14), "Stardew Anxi Panel", "#ffecc2", "body_b")
    pill(d, (360, 12, 484, 44), "运行中", "green")
    pill(d, (500, 12, 668, 44), "春 12 · 08:40", "gold")
    text(d, (W - 292, 18), "管理员 anxi", "#f7ddb0", "small_b")
    button(d, (W - 112, 12, W - 24, 44), "退出", "tan")

    y = TOP + 22
    text(d, (20, y), "朋友农场", "#ffecc2", "h3")
    text(d, (20, y + 38), "Single Game Mode", "#cfa66f", "small")
    y += 92
    for label, icon_name, route in NAV:
        active_row = route == active
        bx = (18, y, SIDE - 18, y + 54)
        if active_row:
            d.rectangle(bx, fill="#f3c879", outline="#412009", width=3)
            d.line((25, y + 6, SIDE - 25, y + 6), fill="#fff0be", width=2)
            fg = "#3a210c"
        else:
            d.rectangle(bx, fill=(90, 45, 16, 145), outline="#8d5a2d", width=2)
            fg = "#f1d6a2"
        im.alpha_composite(icon(icon_name, 30), (30, y + 12))
        text(d, (72, y + 13), label, fg, "small_b")
        y += 62
        if route == "diagnostics":
            y += 18

    main_x = SIDE + MARGIN
    main_w = W - SIDE - RAIL - MARGIN * 2
    text(d, (main_x, TOP + 26), title, C["ink"], "h2")
    fit_text(d, (main_x, TOP + 76), subtitle, main_w - 16, C["ink2"], "small", 4)

    rx = W - RAIL + 24
    text(d, (rx, TOP + 28), "今日状态", C["ink"], "h3")
    ops = [
        ("Docker", "正常", "green"),
        ("Junimo", "已连接", "green"),
        ("存档", "LunaFarm_1225", "gold"),
        ("最近任务", "启动成功", "blue"),
    ]
    oy = TOP + 82
    for k, v, c in ops:
        card(d, (rx, oy, W - 24, oy + 80), "", "", tint="#fff4cc")
        pill(d, (rx + 14, oy + 20, rx + 118, oy + 52), k, c)
        text(d, (rx + 132, oy + 24), v, C["ink"], "small_b")
        oy += 94
    text(d, (rx, H - 145), "下一步", C["ink"], "h3")
    card(d, (rx, H - 103, W - 24, H - 28), "重启后应用模组", "2 个模组变更等待应用", tint="#fff1bf")
    return im, d


def card(d, xy, title="", desc="", tint="#fff0c9", outline="#b88645"):
    x0, y0, x1, y1 = xy
    d.rectangle(xy, fill=tint, outline=outline, width=3)
    d.line((x0 + 5, y0 + 5, x1 - 5, y0 + 5), fill="#fff9dc", width=2)
    if title:
        text(d, (x0 + 18, y0 + 14), title, C["ink"], "body_b")
    if desc:
        fit_text(d, (x0 + 18, y0 + 50), desc, x1 - x0 - 36, C["ink2"], "small", 4)


def table(d, xy, headers, rows, widths):
    x0, y0, x1, y1 = xy
    d.rectangle(xy, fill="#fff3c8", outline="#b88645", width=3)
    h = 46
    d.rectangle((x0, y0, x1, y0 + h), fill="#d5ae68", outline="#8a5a2c", width=2)
    x = x0 + 14
    for head, w in zip(headers, widths):
        text(d, (x, y0 + 11), head, "#3b210d", "small_b")
        x += w
    y = y0 + h
    for i, row in enumerate(rows):
        fill = "#fff8dc" if i % 2 == 0 else "#f7e6b9"
        d.rectangle((x0, y, x1, y + 52), fill=fill)
        x = x0 + 14
        for cell, w in zip(row, widths):
            text(d, (x, y + 13), cell, C["ink"], "small")
            x += w
        d.line((x0, y + 52, x1, y + 52), fill="#e0c184", width=1)
        y += 52


def auth_page(kind: str) -> Image.Image:
    im = Image.new("RGBA", (W, H), "#84c7e4")
    d = ImageDraw.Draw(im)
    for yy in range(H):
        if yy < 520:
            t = yy / 520
            r = int(118 + 48 * t)
            g = int(193 + 25 * t)
            b = int(231 - 30 * t)
            d.line((0, yy, W, yy), fill=(r, g, b))
        elif yy < 690:
            d.line((0, yy, W, yy), fill="#71b94f")
        else:
            d.line((0, yy, W, yy), fill="#8f5a2d")

    d.ellipse((118, 74, 236, 192), fill="#ffd96b", outline="#e1972b", width=4)
    cloud_l = load("sprites/sprite_cloud_left.png", (230, 90))
    cloud_r = load("sprites/sprite_cloud_right.png", (250, 92))
    im.alpha_composite(cloud_l, (390, 90))
    im.alpha_composite(cloud_r, (680, 138))
    im.alpha_composite(cloud_l, (104, 274))

    # Layered Stardew-like hills and crop rows.
    d.polygon([(0, 505), (260, 385), (520, 512), (780, 402), (1060, 510), (1320, 388), (1600, 500), (1600, 690), (0, 690)], fill="#5fac48")
    d.polygon([(0, 585), (320, 490), (760, 620), (1120, 500), (1600, 590), (1600, 705), (0, 705)], fill="#3f8f3d")
    d.rectangle((0, 690, W, 1000), fill="#8f5a2d")
    for yy in range(714, 1000, 58):
        d.polygon([(0, yy), (W, yy - 28), (W, yy + 14), (0, yy + 46)], fill="#a96a32")
        d.line((0, yy + 44, W, yy + 2), fill="#6b3717", width=3)
    for xx in range(64, 850, 74):
        for yy in range(724, 966, 58):
            d.ellipse((xx, yy, xx + 22, yy + 14), fill="#5fab46")
            d.line((xx + 11, yy + 14, xx + 11, yy + 30), fill="#387a2e", width=3)

    scene = load("sprites/sprite_farmhouse_scene.png").resize((560, 250), Image.Resampling.NEAREST)
    im.alpha_composite(scene, (160, 420))
    im.alpha_composite(load("sprites/sprite_tree.png", (145, 178)), (92, 450))
    im.alpha_composite(load("sprites/sprite_chest.png", (82, 82)), (670, 606))
    d.rectangle((0, 0, W, H), fill=(45, 22, 4, 34))

    shadow_box(im, (930, 150, 1395, 814), "#f4cc82", "#5b2f18", 6, 14)
    d = ImageDraw.Draw(im)
    im.alpha_composite(icon("icon_sidebar_chicken", 58), (967, 205))
    text(d, (1038, 204), "Stardew Anxi Panel", "#3a210c", "h3")
    if kind == "setup":
        title, desc, btn = "创建第一个管理员", "首次进入面板。创建管理员后直接进入朋友农场控制台。", "创建管理员"
        fields = ["管理员用户名", "管理员密码", "确认密码"]
    else:
        title, desc, btn = "回到朋友农场", "输入面板账号，继续管理 JunimoServer、存档和模组。", "登录"
        fields = ["用户名", "密码"]
    text(d, (970, 278), title, C["ink"], "h2")
    fit_text(d, (970, 334), desc, 350, C["ink2"], "small", 5)
    y = 420
    for field in fields:
        text(d, (970, y), field, C["ink"], "small_b")
        d.rectangle((970, y + 34, 1345, y + 78), fill="#fff2c8", outline="#7b4a24", width=3)
        d.line((976, y + 40, 1338, y + 40), fill="#fff9df", width=2)
        y += 100
    button(d, (970, y + 12, 1345, y + 62), btn, "green")
    text(d, (970, 768), "像素农场主题 · Docker Socket 本地面板", "#6a4524", "small")
    return im


def overview() -> Image.Image:
    im, d = shell("overview", "总览", "把服务器状态、邀请码、存档、模组与最近任务压缩在第一屏，适合日常一眼巡检。")
    x, y = SIDE + MARGIN, TOP + 128
    scene = load("sprites/sprite_farmhouse_scene.png").resize((900, 145), Image.Resampling.NEAREST)
    im.alpha_composite(scene, (x, y))
    d.rectangle((x, y, x + 900, y + 145), outline="#7a4a22", width=4)
    d.rectangle((x, y, x + 900, y + 145), fill=(246, 223, 170, 88))
    pill(d, (x + 24, y + 22, x + 158, y + 58), "运行中", "green")
    text(d, (x + 182, y + 24), "邀请码  XF7A-29QK", C["ink"], "h3")
    button(d, (x + 660, y + 24, x + 790, y + 72), "复制", "gold")
    y += 172
    metrics = [("在线玩家", "3 / 8", "Abby, Sam, Leah", "green"), ("当前存档", "LunaFarm", "春 12 · 第 2 年", "gold"), ("模组", "28 个", "2 个等待重启", "blue"), ("健康", "全部正常", "Docker / Compose / 目录", "green")]
    for i, (a, b, c, col) in enumerate(metrics):
        xx = x + (i % 2) * 448
        yy = y + (i // 2) * 130
        card(d, (xx, yy, xx + 420, yy + 110), a, c)
        text(d, (xx + 230, yy + 38), b, C[col], "h3")
    y += 285
    table(d, (x, y, x + 900, y + 230), ["时间", "事件", "结果"], [["08:41", "服务器启动", "成功"], ["08:43", "读取邀请码", "成功"], ["08:50", "上传模组 Automate", "待重启"]], [150, 470, 230])
    return im


def install() -> Image.Image:
    im, d = shell("install", "首次安装向导", "把 Steam 凭据、镜像拉取、Steam Guard 和游戏下载做成连续任务，避免用户迷失在日志里。")
    x, y = SIDE + MARGIN, TOP + 135
    card(d, (x, y, x + 900, y + 92), "安装状态：Steam 认证中", "当前阶段 steam_guard_required，等待验证码输入。", "#fff3c8")
    y += 116
    steps = ["准备环境", "拉取镜像", "Steam 认证", "下载游戏", "完成"]
    for i, s in enumerate(steps):
        xx = x + i * 178
        col = "green" if i < 2 else "gold" if i == 2 else "gray"
        pill(d, (xx, y, xx + 152, y + 38), s, col)
        if i < 4:
            d.line((xx + 156, y + 19, xx + 176, y + 19), fill="#8a5a2c", width=3)
    y += 68
    card(d, (x, y, x + 430, y + 250), "安装配置", "Steam 用户名\nSteam 密码\nVNC 密码\nJunimo 镜像版本：推荐最新版")
    button(d, (x + 32, y + 178, x + 214, y + 226), "确认安装", "green")
    card(d, (x + 464, y, x + 900, y + 250), "Steam Guard", "请输入邮箱验证码，或在手机 App 批准登录。日志继续实时滚动。", "#fff8dc")
    d.rectangle((x + 498, y + 106, x + 760, y + 154), fill="#fff2c8", outline="#7b4a24", width=3)
    text(d, (x + 516, y + 118), "验证码  8X2Q9", C["muted"], "small")
    button(d, (x + 498, y + 176, x + 692, y + 224), "提交验证码", "green")
    y += 282
    d.rectangle((x, y, x + 900, y + 230), fill="#1b1108", outline="#6e421e", width=4)
    text(d, (x + 18, y + 16), "安装日志", "#ffe9b4", "body_b")
    logs = ["[pull] JunimoServer image ready", "[steam] Steam Guard required", "[steam] Waiting for code input", "[panel] passwords redacted"]
    for i, log in enumerate(logs):
        text(d, (x + 24, y + 60 + i * 36), log, "#9ee38a" if i != 1 else "#ffd36b", "small")
    return im


def server() -> Image.Image:
    im, d = shell("server", "服务器控制", "生命周期操作、邀请码、全服喊话和 allowlist 命令集中在一个像素控制台里。")
    x, y = SIDE + MARGIN, TOP + 135
    card(d, (x, y, x + 900, y + 120), "当前状态", "运行中 · 当前存档 LunaFarm_1225 · 更新时间 08:41", "#fff3c8")
    pill(d, (x + 650, y + 34, x + 804, y + 72), "运行中", "green")
    y += 150
    button(d, (x, y, x + 152, y + 54), "启动", "green", "▶", True)
    button(d, (x + 172, y, x + 324, y + 54), "停止", "red", "■")
    button(d, (x + 344, y, x + 496, y + 54), "重启", "tan", "↻")
    d.rectangle((x + 540, y, x + 900, y + 54), fill="#fff2c8", outline="#7b4a24", width=3)
    text(d, (x + 562, y + 12), "邀请码 XF7A-29QK", C["ink"], "body_b")
    y += 92
    card(d, (x, y, x + 900, y + 128), "全服消息", "向在线玩家发送 say 消息。", "#fff8dc")
    d.rectangle((x + 24, y + 62, x + 650, y + 105), fill="#fff2c8", outline="#7b4a24", width=2)
    text(d, (x + 42, y + 72), "今晚 9 点打矿洞，记得带炸弹", C["muted"], "small")
    button(d, (x + 684, y + 58, x + 840, y + 108), "发送", "green")
    y += 158
    table(d, (x, y, x + 900, y + 240), ["命令", "权限", "说明"], [["info", "所有用户", "服务器信息"], ["invitecode", "所有用户", "刷新邀请码"], ["settings-show", "管理员", "查看配置"]], [230, 180, 430])
    return im


def saves() -> Image.Image:
    im, d = shell("saves", "存档与备份", "存档选择、新建/上传、运行中保护、备份恢复和彻底删除都在同一页完成。")
    x, y = SIDE + MARGIN, TOP + 135
    button(d, (x, y, x + 160, y + 50), "新建存档", "green")
    button(d, (x + 176, y, x + 336, y + 50), "上传存档", "gold")
    button(d, (x + 352, y, x + 512, y + 50), "刷新", "tan")
    y += 80
    saves_rows = [["LunaFarm_1225", "Luna", "第 2 年 春 12", "当前启动"], ["RiverTown_901", "Sam", "第 1 年 夏 5", "可删除"], ["Meadow_778", "Marnie", "第 3 年 秋 22", "备用"]]
    table(d, (x, y, x + 900, y + 225), ["存档", "农夫", "日期", "状态"], saves_rows, [300, 160, 230, 170])
    y += 260
    text(d, (x, y), "备份与恢复", C["ink"], "h3")
    y += 48
    backup_rows = [["LunaFarm_1225_0830.zip", "LunaFarm", "第 2 年 春 12", "恢复 / 删除"], ["RiverTown_901_0629.zip", "RiverTown", "第 1 年 夏 5", "恢复 / 删除"]]
    table(d, (x, y, x + 900, y + 170), ["备份 ZIP", "原存档", "内容", "操作"], backup_rows, [320, 180, 230, 150])
    return im


def jobs() -> Image.Image:
    im, d = shell("jobs", "任务与日志", "把后台长任务、实时 SSE 日志和失败修复入口放在可追踪的任务中心。")
    x, y = SIDE + MARGIN, TOP + 135
    table(d, (x, y, x + 900, y + 220), ["任务", "状态", "开始时间", "结果"], [["stardew_start", "成功", "08:40", "邀请码已刷新"], ["save_backup_delete", "成功", "08:12", "已删除 ZIP"], ["stardew_install", "失败", "昨天", "VNC 端口占用"]], [300, 140, 190, 230])
    y += 258
    card(d, (x, y, x + 900, y + 90), "修复建议：VNC 端口被占用", "将端口从 5800 改为 5801 后重试启动。", "#fff1bf")
    button(d, (x + 650, y + 22, x + 850, y + 68), "更换端口", "gold")
    y += 120
    d.rectangle((x, y, x + 900, y + 260), fill="#1b1108", outline="#6e421e", width=4)
    text(d, (x + 18, y + 18), "实时日志流", "#ffe9b4", "body_b")
    logs = ["08:40:12 docker compose up -d", "08:40:31 JunimoServer ready", "08:40:35 attach-cli invitecode", "08:40:36 invite code refreshed [REDACTED]"]
    for i, log in enumerate(logs):
        text(d, (x + 24, y + 66 + i * 38), log, "#9ee38a", "small")
    return im


def players() -> Image.Image:
    im, d = shell("players", "玩家", "在线玩家、最近加入记录和未来白名单/权限入口保留在 Stardew 专属页。")
    x, y = SIDE + MARGIN, TOP + 135
    cards = [("Abby", "在线 42 分钟", "green"), ("Sam", "在线 18 分钟", "green"), ("Leah", "刚加入", "gold")]
    for i, (name, sub, col) in enumerate(cards):
        xx = x + i * 300
        card(d, (xx, y, xx + 270, y + 145), name, sub, "#fff8dc")
        pill(d, (xx + 22, y + 88, xx + 130, y + 124), "在线", col)
    y += 190
    table(d, (x, y, x + 900, y + 270), ["玩家", "事件", "时间", "备注"], [["Abby", "加入游戏", "08:12", "通过邀请码"], ["Sam", "发送消息", "08:29", "准备下矿"], ["Leah", "离开游戏", "昨天", "正常退出"]], [180, 270, 170, 250])
    y += 304
    card(d, (x, y, x + 900, y + 95), "待接入能力", "白名单、封禁、玩家角色和在线人数 API 将通过 Junimo/SMAPI 能力接入。", "#eef6cc")
    return im


def mods() -> Image.Image:
    im, d = shell("mods", "模组", "像背包物品栏一样管理 Mod：搜索、上传、删除、版本状态和重启提示都要清楚。")
    x, y = SIDE + MARGIN, TOP + 135
    d.rectangle((x, y, x + 520, y + 48), fill="#fff2c8", outline="#7b4a24", width=3)
    text(d, (x + 18, y + 11), "搜索模组 / 文件名", C["muted"], "small")
    button(d, (x + 550, y, x + 710, y + 50), "上传", "green")
    button(d, (x + 728, y, x + 900, y + 50), "重启应用", "gold")
    y += 85
    mods_data = [("SMAPI", "4.1.8", "核心"), ("Automate", "2.1.0", "待重启"), ("Content Patcher", "2.7.3", "正常"), ("Lookup Anything", "1.46", "正常"), ("SkullCavern", "0.9", "待重启"), ("NPC Map", "3.0", "正常")]
    for i, (name, ver, status) in enumerate(mods_data):
        xx = x + (i % 3) * 300
        yy = y + (i // 3) * 165
        card(d, (xx, yy, xx + 276, yy + 138), name, f"版本 {ver}", "#fff8dc")
        pill(d, (xx + 18, yy + 88, xx + 142, yy + 122), status, "gold" if status == "待重启" else "green")
    y += 360
    card(d, (x, y, x + 900, y + 92), "安全边界", "上传 ZIP 后仅写入挂载 Mods 目录；删除/应用仍要求服务端停机或重启确认。", "#fff1bf")
    return im


def diagnostics() -> Image.Image:
    im, d = shell("diagnostics", "诊断", "用体检单的方式展示 Docker、Compose、数据目录、实例文件和 active save 状态。")
    x, y = SIDE + MARGIN, TOP + 135
    button(d, (x, y, x + 190, y + 52), "开始检查", "green")
    button(d, (x + 210, y, x + 370, y + 52), "刷新", "tan")
    y += 88
    rows = [["Docker daemon", "正常", "Docker Desktop 已连接"], ["Docker Compose", "正常", "compose plugin 可用"], ["数据目录", "正常", "/data 可写"], ["VNC 端口", "警告", "5800 曾被占用"], ["当前存档", "正常", "LunaFarm_1225 可读取"]]
    table(d, (x, y, x + 900, y + 330), ["检查项", "状态", "说明"], rows, [260, 160, 430])
    y += 365
    card(d, (x, y, x + 900, y + 110), "一键修复入口", "对可恢复问题直接给出操作：更换 VNC 端口、重新拉取镜像、重新扫描存档。", "#eef6cc")
    return im


def settings() -> Image.Image:
    im, d = shell("settings", "设置", "面板账号、管理员权限、审计日志和基础安全状态统一放在高阶设置页。")
    x, y = SIDE + MARGIN, TOP + 135
    card(d, (x, y, x + 430, y + 155), "当前账号", "anxi · 管理员\n权限能力：首个管理员隐藏超级权限", "#fff8dc")
    card(d, (x + 470, y, x + 900, y + 155), "安全状态", "HttpOnly session · Argon2id\n敏感日志已脱敏", "#eef6cc")
    y += 195
    table(d, (x, y, x + 900, y + 220), ["用户", "角色", "状态", "操作"], [["anxi", "管理员", "启用", "不可删除"], ["friend01", "普通用户", "启用", "禁用/删除"], ["farm-admin", "管理员", "启用", "仅首管可改"]], [220, 180, 150, 300])
    y += 260
    table(d, (x, y, x + 900, y + 220), ["时间", "操作", "操作者", "目标"], [["08:41", "instance_start", "anxi", "stardew"], ["08:12", "save_backup_delete", "anxi", "LunaFarm.zip"], ["昨天", "auth_login", "friend01", "panel"]], [170, 260, 180, 260])
    return im


def save_all():
    OUT.mkdir(parents=True, exist_ok=True)
    pages = [
        ("00-initial-setup.png", auth_page("setup")),
        ("01-login.png", auth_page("login")),
        ("02-overview.png", overview()),
        ("03-install-guide.png", install()),
        ("04-server-control.png", server()),
        ("05-saves-backups.png", saves()),
        ("06-jobs-logs.png", jobs()),
        ("07-players.png", players()),
        ("08-mods.png", mods()),
        ("09-diagnostics.png", diagnostics()),
        ("10-settings.png", settings()),
    ]
    for name, im in pages:
        im.convert("RGB").save(OUT / name, quality=95)

    thumb_w, thumb_h = 480, 300
    sheet = Image.new("RGB", (thumb_w * 2 + 36, (thumb_h + 54) * 6 + 24), "#f3dfad")
    sd = ImageDraw.Draw(sheet)
    for i, (name, im) in enumerate(pages):
        row, col = divmod(i, 2)
        x = 18 + col * (thumb_w + 18)
        y = 18 + row * (thumb_h + 54)
        thumb = im.convert("RGB").resize((thumb_w, thumb_h), Image.Resampling.LANCZOS)
        sheet.paste(thumb, (x, y))
        sd.rectangle((x, y, x + thumb_w, y + thumb_h), outline="#7a4a22", width=2)
        sd.text((x + 4, y + thumb_h + 8), name, font=F["small"], fill=C["ink"])
    sheet.save(OUT / "stardew-v2-page-prototypes-contact-sheet.png", quality=95)


if __name__ == "__main__":
    save_all()

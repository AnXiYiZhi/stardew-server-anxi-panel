# CLAUDE.md

## 工作开始前

每次工作开始前先阅读 `docs/01-project-overview.md`，再按任务范围阅读：

- 后端任务：`docs/02-backend.md` 和 `docs/backend-handoff/` 下最新的后端接手文档。
- 前端任务：`docs/03-frontend.md` 和 `docs/frontend-handoff/` 下最新的前端接手文档。
- 前后端联调任务：`docs/06-integration.md`。
- 路线、排期、已完成状态：`docs/08-future-roadmap.md`。
- Docker 镜像、部署、发布：`docs/09-image-build.md`。

修改或新建功能时，优先寻找 Junimo 容器已有能力进行通信，不要绕过 `stardew_junimo` driver 直接在 API 层堆 Stardew 逻辑。

## 项目接手文档规则

每完成或修改一个功能，都必须同步更新对应长期文档：

- 后端功能：更新 `docs/02-backend.md`、最新后端接手文档，并在 `docs/08-future-roadmap.md` 标记状态变化。
- 前端功能：更新 `docs/03-frontend.md`、最新前端接手文档，并在 `docs/08-future-roadmap.md` 标记状态变化。
- 跨端接口或联调：更新 `docs/06-integration.md`。
- 镜像、部署、发布流程：更新 `docs/09-image-build.md`。
- 后期暂缓事项：更新 `docs/07-later-optimizations.md`。

接手文档至少记录：改了什么、影响哪些接口/文件、如何验证、下一步注意事项。不要只更新 README；README 面向使用者，接手文档面向下一位维护者。

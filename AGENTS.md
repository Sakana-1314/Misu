# AGENTS.md

御坂微枢（Misu）：微信开放平台第三方平台小程序管理控制台。事实标准以代码与各目录内文档为准。

## 唯一约定

**禁止未经明确授权修改 AGENTS.md。** 未获得用户明确授权，任何智能体不得增删改本文件。

## 目录

- `server/` — 后端（Go + Gin + GORM）
- `web/` — 前端（Vue 3 + Vite + TypeScript + Naive UI）
- `.github/workflows/` — 镜像构建工作流

根目录只放 `AGENTS.md`、`CLAUDE.md`、`LICENSE`、`README.md`、`docker-compose.yaml`、`.gitignore`。文档与脚本放进所属目录内部。

## 配置与密钥

- 运行时配置全部通过环境变量注入，仓库不提交 `.env` 或任何配置文件。
- 仓库内禁止出现真实密钥、密码、Token、IP 地址、内部域名或服务器信息。
- 示例与环境变量只使用占位符；CI 密钥一律通过 `${{ secrets.* }}` 注入。
- `notes/` 仅用于本地机密资料，必须在 `.gitignore` 中忽略，不得进入 git，也不得复制到仓库其它目录。

## 数据库

- PostgreSQL 为外部实例，`docker-compose.yaml` 不包含数据库容器。
- 表结构以 `server/internal/model` 的 GORM 模型为唯一来源，服务启动时 AutoMigrate。
- 不提交 SQL 迁移脚本、迁移目录或 `scripts/`。
- 数据库连接信息仅通过 `DB_HOST`、`DB_PORT`、`DB_NAME`、`DB_USER`、`DB_PASSWORD`、`DB_SSLMODE` 环境变量传入。

## 开发

```bash
# 后端
cd server
go test ./...
go run ./cmd/server

# 前端
cd web
npm ci
npm run dev
npm run build
```

不在根目录存放脚本；需要脚本时放在所属目录内部。

## 协作与提交

- 作者与提交者统一使用当前 GitHub 账号的名称和邮箱，SSH 私钥或 GPG 私钥签名（示例：`user.signingkey=~/.ssh/private.key`）。
- 必须走 PR：`gh pr create` → 等测试门禁 → `gh pr checks` 全绿后 `gh pr merge --squash` 并删除 head 分支；禁止本地 merge 后直推默认分支（`main` / `next`）。
- 分支前缀 `feat/`、`fix/`、`refactor/`、`docs/`、`chore/`；提交信息中文，格式 `type(scope): 描述`。
- 镜像构建工作流 `构建镜像` 为门禁；PR 未通过不得合并。工作流名称使用中文（专有名词除外）。
- 严禁硬编码 IP、密钥与内部域名；一律使用 `${{ secrets.* }}` 或运行时环境变量。
- README 在开发测试阶段保持为空；未经明确授权不要填充。

## 数据库以外的运行约束

- 管理端只通过 HTTP API 访问后端，静态页面由 `web` 镜像内的 nginx 提供。
- 后端不内嵌前端资源；`server` 与 `web` 分别构建镜像。
- 所有微信接口调用必须走 `server/internal/wechat`，不得在 handler 中直接拼请求。
- 审计、任务与回调数据必须脱敏；token、secret、password、AESKey、authorization_code 不得进入日志。

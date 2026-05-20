# 配置与开发

这里收集本地把 Homer 跑起来需要的东西：环境要求、目录结构、`.env` 配置详表、调度任务、通知通道、SQL 迁移、本地开发命令、常用脚本、健康检查与排错。

回到[项目首页](../README.md)。

## 环境要求

| 工具 | 版本 | 备注 |
| --- | --- | --- |
| Go | 1.25 或更新 | `go.mod` 顶部 `toolchain go1.25.x` 锁定 |
| Node.js | 20 或更新 | 前端 Vite + TS |
| MySQL | 8.x（兼容 5.7） | 字符集 `utf8mb4` |

本地有固定 Go 路径（开发环境约定）：

```sh
export PATH=/opt/module/go/go1.25.0/bin:$PATH
```

如果用 `goenv` / `gvm` / Homebrew，直接走自己的版本管理。

## 目录结构

```text
homer/
├── main.go                 # 入口：load config → init log → buildServer → Run
├── wire.go                 # 依赖组装：DB / notify hub / ACME service / scheduler / handlers
├── go.mod / go.sum
├── .env / .env.example
├── docs/                   # 本目录：deployment / development / api 三件套
├── internal/
│   ├── acme/               # ACME 签发、续期、SSE；子包 deployer/{ssh,safeline,alicas,fnos}、providers/
│   ├── aliyun/             # 阿里云 SDK 客户端封装
│   ├── birthday/、event/    # 生日 / 事项提醒 RunOnce
│   ├── buildinfo/          # 版本/commit/build_id（ldflags 注入）
│   ├── certstore/、cdnops/  # 阿里云 CAS 库存查询 / CDN 加速域名查询与 CAS→CDN 部署
│   ├── chinesedate/        # 农历/生肖
│   ├── config/、db/         # .env 加载、GORM MySQL 初始化
│   ├── handler/            # HTTP handler（含 acme/ 子包）
│   ├── jobmonitor/         # 任务失败计数 + 告警门槛
│   ├── logx/               # slog 结构化日志
│   ├── model/              # GORM 模型（按模块拆分文件）
│   ├── notify/             # 通道适配 + Hub + 模块绑定
│   ├── router/             # API + SPA fallback
│   ├── scheduler/          # 进程内 cron (robfig/cron v3，6 段秒级)
│   ├── sms/                # SmsForwarder 适配
│   └── web/                # embedded SPA handler
├── sql/                    # 00 全量建表 + 0X_* 增量迁移（幂等存储过程）
├── frontend/               # React 19 + Vite + TS + Tailwind v4
│   ├── dist/               # vite 产物，被 //go:embed all:frontend/dist 引用
│   └── src/
├── Dockerfile / docker-compose.yml
└── bin/                    # go build 产物（gitignore）
```

## 配置（`.env`）

启动入口 `main.go` → `config.Load()` → `godotenv.Load()`（找不到 `.env` 不报错，env 全部缺省继续走）。

```sh
cp .env.example .env
```

### 必填（连不上 MySQL 即 fatal）

| KEY | 代码默认 | 说明 |
| --- | --- | --- |
| `DB_HOST` | `127.0.0.1` | |
| `DB_PORT` | `3306` | |
| `DB_USER` | `root` | |
| `DB_PASSWORD` | `""` | |
| `DB_NAME` | `""` | 必须填 |
| `DB_CHARSET` | `utf8mb4` | |
| `SERVER_PORT` | `8081` | 经 `normalizePort`，去 `:` 前缀；占用即 fatal |

### 日志

| KEY | 代码默认 | 说明 |
| --- | --- | --- |
| `LOG_LEVEL` | `info` | `debug` / `info` / `warn` / `error`，slog 标准级别 |

### 调度任务（cron）

均为 robfig/cron v3 的 **6 段秒级**：`秒 分 时 日 月 周`。

| KEY | 代码默认 | job 名 | 说明 |
| --- | --- | --- | --- |
| `BIRTHDAY_REMIND_CRON` | `0 0 9 * * *` | `birthday` | 生日提醒 |
| `EVENT_REMIND_CRON` | `0 0 9 * * *` | `event` | 事项提醒（同日去重） |
| `ACME_RENEW_CRON` | `0 0 3 * * *` | `acme-renew` | ACME 自动续期检查 |
| `ACME_DEPLOY_RETRY_CRON` | `0 * * * * *` | `acme-deploy-retry` | 部署任务兜底重试 |
| `SCHEDULER_ALERT_FAIL_THRESHOLD` | `1` | — | 同名 job 连续失败 ≥ 该值时推送 `scheduler_alert` 模块 |

> env 设为**空字符串**（不是不写）→ scheduler 把该 job 登记为 manual-only，仍可在 `/api/scheduler/jobs/:name/run` 手动触发。

### ACME

| KEY | 代码默认 | 说明 |
| --- | --- | --- |
| `ACME_DATA_DIR` | `./data/acme` | lego 私钥、acme.json 落盘；生产必须持久化 |
| `ACME_RENEW_BEFORE_DAYS` | `30` | 剩余天数 ≤ 此值触发续期 |
| `ACME_KEY_TYPE` | `ec256` | `ec256` / `ec384` / `rsa2048` / `rsa3072` / `rsa4096` / `rsa8192` |
| `ACME_DEPLOY_RETRY` | `3` | 单任务内总执行次数（含首次）；`1` = 不重试 |
| `ACME_DEPLOY_RETRY_BACKOFF_SEC` | `10` | 退避基数；实际间隔 = 基数 × 已执行次数 |

> 这些重试只对**持久化部署配置**触发的任务生效；临时部署（手动一次性测试）不进重试队列。

### 阿里云 AK/SK（留空 → 接口返回 503）

| KEY | 用途 |
| --- | --- |
| `ALIYUN_CDN_ACCESS_KEY_ID` / `ALIYUN_CDN_ACCESS_KEY_SECRET` | CDN 加速域名查询、`/api/certstore/deploy` 把 CAS 证书部署到 CDN |
| `ALIYUN_CAS_ACCESS_KEY_ID` / `ALIYUN_CAS_ACCESS_KEY_SECRET` | CAS 证书列表/删除（全局凭证） |

> ACME 内部的 `upload_cas` driver 使用 per-target AK/SK（建在 deploy target 上），**与上面这套全局凭证不共用**，可以分配不同子账号、不同权限。

### Docker 容器层（仅 docker-compose 用，不进 Config struct）

| KEY | 默认 | 说明 |
| --- | --- | --- |
| `IMAGE` | `ghcr.io/lemonzuo/homer:latest` | 拉取的镜像 |
| `HOST_PORT` | `8081` | 宿主机映射端口 |
| `TZ` | `Asia/Shanghai` | 容器时区 |
| `GIN_MODE` | `release` | Gin 框架自读 |

### 通知通道（DB 维护，不在 env）

企业微信 / Resend 邮件 / Webhook 三种通道、4 个模块（`birthday` / `event` / `bypass` / `scheduler_alert`）的配置与绑定全部走 `/api/notify/*` 或 UI 操作，运行时实时查 DB，**改动即时生效**。字段清单见 [api.md#支持的-channel-type](api.md#支持的-channel-type)。

## 启动顺序

`main.go` → `wire.buildServer`：

1. `config.Load()` → `logx.Init` → 日志打印 `homer starting`
2. `fs.Sub(frontendFS, "frontend/dist")`（embed），失败 fatal
3. `db.New(cfg)` 连 MySQL，失败 fatal
4. `gormDB.AutoMigrate(...)` 14 张表（含 birthday / event / sms / notify / scheduler / ACME 全系列 / ssh_credential），失败 fatal
5. `notify.NewHub(db)` + `notify.NewStore(db)`
6. `cdnops` / `certstore` service 构造（AK/SK 留空时标记未配置）
7. `buildACMEService` 注册 4 个 deploy driver（ssh / safeline / alicas / fnos）
8. `startScheduler` 注册 4 个 cron job → `mon.Hydrate(sched)` 预热历史 → `sched.Start()`，任一注册失败 fatal
9. `router.Setup(...)` 装 API + SPA fallback，再追加 `GET /healthz`
10. `srv.Run(":<SERVER_PORT>")`；退出时 `sched.Stop()`

**可选启用清单**（env 不填即降级）：

- `ALIYUN_CDN_*` 空 → `/api/cdnops/domains` 503
- `ALIYUN_CAS_*` 空 → `/api/certstore/*` 503
- 任一 cron env 设为空字符串 → 该 job 仅手动触发
- 通知通道未配置 → 模块手动推送接口报「企业微信未配置」类错误

## SQL 与 AutoMigrate（双轨）

Schema 变更采取**双轨并行**：GORM `AutoMigrate` + `sql/` 增量脚本，两边都要补，避免老库被 AutoMigrate 默默改坏。

`sql/` 目录文件：

| 文件 | 用途 |
| --- | --- |
| `00_schema.sql` | 全量建表；`DROP TABLE IF EXISTS` 开头，**只适合全新初始化** |
| `01_migrate_birthday_rename.sql` | 老 ruoyi 库 `sys_birthday_remind → birthday_reminder` 改名 |
| `02_acme_san_providers.sql` | `acme_domain.san_providers`：按 SAN 指定 DNS provider 覆盖 |
| `03_acme_cas_enabled.sql` | `acme_domain.cas_enabled`：是否参与 CAS（默认 `'0'`） |
| `04_acme_drop_main_domain_unique.sql` | 去掉 `acme_domain.main_domain` 唯一索引（允许并存证书） |
| `05_acme_issue_task_retry.sql` | `acme_issue_task` 增加失败重试列 |
| `06_acme_cas_decouple.sql` | CAS 改为通用 driver `upload_cas`，老 `cas_enabled` / `cas_cert_id` 不再使用 |

`0X_*.sql` 全部用存储过程做幂等保护，重复执行无副作用。新装库按编号依次跑一遍即可（也可只跑 00，跳过 0X，因为 00 已含最新列）。已有数据的库只跑 0X 增量。

启动时 AutoMigrate 只会按 GORM tag 做加列/加索引这类**追加式**变更，不会 drop。所以 `sql/00_schema.sql` 不能放进启动流程，要人工执行。

## 本地开发

后端：

```sh
export PATH=/opt/module/go/go1.25.0/bin:$PATH
go run .
```

监听 `:SERVER_PORT`（默认 `8081`）。

前端：

```sh
cd frontend
npm install
npm run dev
```

Vite 监听 `0.0.0.0:5173`，把 `/api` 反代到 `http://localhost:8081`。开发态调 UI 一律走 `http://localhost:5173`，**不要**用 `:8081`（embed 目录空，只会拿到占位 HTML）。

## 常用命令

后端：

```sh
go test ./...                 # 单元测试
go vet ./...                  # 静态分析
gofmt -l .                    # 格式检查（CI 用）
```

前端：

```sh
cd frontend
npm run lint
npm run build                 # 构建到 frontend/dist，并补回 .gitkeep
npm run check:case            # 路径大小写一致性（macOS dev / Linux build 不一致排查）
```

构建单二进制（embed 前端 + 嵌入版本信息）：

```sh
# 1. 前端
cd frontend && npm install && npm run build && cd ..

# 2. 后端
export PATH=/opt/module/go/go1.25.0/bin:$PATH
go build -ldflags="-s -w" -o bin/server .   # -s -w 砍符号表，体积省 30%+
./bin/server
```

注入版本号（release 流程一致）：

```sh
PKG=github.com/LemonZuo/homer/internal/buildinfo
go build \
  -trimpath \
  -ldflags="-s -w \
    -X ${PKG}.Version=v0.4.4 \
    -X ${PKG}.Commit=$(git rev-parse --short HEAD) \
    -X ${PKG}.BuildID=$(git rev-parse HEAD | head -c 12)" \
  -o bin/server .
```

## 健康检查与排错

- `GET /healthz` → `{"db":"ok|down","scheduler":"ok|down"}`，整体 200/503。
- `GET /api/version` → 二进制注入的版本/commit/build_id。
- `GET /api/scheduler/jobs` → 每个 job 的上次执行时间、连续失败次数；调度问题先看这里。
- SSE 调试：浏览器 DevTools 看 `EventStream` tab 是否能持续收到 `log` 事件；如果只收到第一行就断开，多半是反代缓冲没关。

常见启动失败：

| 现象 | 排查点 |
| --- | --- |
| `connect db ...` fatal | `.env` MySQL 五件套、`DB_NAME` 是否填了、防火墙、`utf8mb4` collation |
| `migrate ...` fatal | 老库字段冲突；先用对应 `sql/0X_*.sql` 把表对齐再启动 |
| `run server ... bind: address already in use` | `SERVER_PORT` 被占用，`lsof -i :8081` 看下 |
| `register scheduler job ...` fatal | cron 表达式 5 段而非 6 段（漏了秒） |

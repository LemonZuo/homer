# 配置与开发

这里收集本地把 Homer 跑起来需要的东西：环境要求、`.env` 配置、目录结构、本地开发命令、打包单二进制。

回到[项目首页](../README.md)。

## 环境要求

- Go 1.25 或更新版本。本地已有固定路径时可使用：

```sh
export PATH=/opt/module/go/go1.25.0/bin:$PATH
```

- Node.js 20 或更新版本。
- MySQL 8.x 或兼容版本。

## 目录结构

```text
homer/
├── main.go                 # 入口，组装配置、数据库、服务、路由和调度器
├── wire.go                 # ACME / scheduler 依赖组装
├── internal/
│   ├── acme/               # ACME 签发、续期、部署（SSH/SafeLine/alicas/fnos）、SSE 日志
│   ├── aliyun/             # 阿里云 SDK 客户端封装
│   ├── birthday/           # 生日提醒任务
│   ├── certstore/          # 证书库存查询/删除/部署到 CDN（当前基于阿里云 CAS）
│   ├── cdnops/             # CDN 加速域名查询与证书部署（当前基于阿里云 CDN）
│   ├── config/             # .env 配置加载
│   ├── db/                 # GORM MySQL 初始化
│   ├── event/              # 事项提醒任务
│   ├── handler/            # HTTP handler
│   ├── jobmonitor/         # 调度任务失败计数 + 告警门槛
│   ├── logx/               # slog 结构化日志封装
│   ├── model/              # GORM 模型
│   ├── notify/             # 企业微信、邮件等通知适配
│   ├── router/             # API 和 SPA 路由
│   ├── scheduler/          # 进程内 cron 调度器
│   ├── sms/                # SmsForwarder 适配
│   └── web/                # embedded SPA handler
├── frontend/               # React 前端
├── sql/                    # 首次建表 SQL
├── Dockerfile
└── docker-compose.yml
```

## 配置

先给小管家一份值班表。复制环境变量模板：

```sh
cp .env.example .env
```

至少需要配置 MySQL，这是 Homer 的记事本：

```dotenv
DB_HOST=127.0.0.1
DB_PORT=3306
DB_USER=root
DB_PASSWORD=
DB_NAME=homer
DB_CHARSET=utf8mb4
SERVER_PORT=8081
LOG_LEVEL=info
```

可选配置按模块启用：

| 配置项 | 用途 |
| --- | --- |
| `LOG_LEVEL` | slog 日志级别，可选 `debug` / `info` / `warn` / `error`，默认 `info` |
| `BIRTHDAY_REMIND_CRON` | 生日提醒 cron，6 段格式：秒 分 时 日 月 周；留空则只支持手动触发 |
| `EVENT_REMIND_CRON` | 事项提醒 cron；留空则只支持手动触发 |
| `ALIYUN_CDN_ACCESS_KEY_ID` / `ALIYUN_CDN_ACCESS_KEY_SECRET` | 阿里云 CDN 加速域名管理 |
| `ALIYUN_CAS_ACCESS_KEY_ID` / `ALIYUN_CAS_ACCESS_KEY_SECRET` | 阿里云 CAS 证书列表和「CAS → CDN」部署的全局凭证；ACME 内部的 alicas 部署 driver 使用 per-target 凭证 |
| `ACME_DATA_DIR` | ACME 账号私钥和签发工作目录，默认 `./data/acme` |
| `ACME_RENEW_BEFORE_DAYS` | 证书剩余天数小于等于该值时自动续期，默认 `30` |
| `ACME_RENEW_CRON` | ACME 自动续期检查 cron |
| `ACME_KEY_TYPE` | 证书密钥类型，可选 `ec256` / `ec384` / `rsa2048` / `rsa4096` 等，默认 `ec256` |
| `ACME_DEPLOY_RETRY` | 单次部署失败的自动重试次数（同次任务内立即重试），默认 `3` |
| `ACME_DEPLOY_RETRY_BACKOFF_SEC` | 同次部署内重试之间的等待秒数，默认 `10` |
| `ACME_DEPLOY_RETRY_CRON` | 失败部署任务的兜底重试 cron；留空则不启用 |
| `SCHEDULER_ALERT_FAIL_THRESHOLD` | 调度任务连续失败多少次触发告警，默认 `1` |

> 通知通道（企业微信应用、Webhook、邮件等）不再从 env 读取，统一在数据库里维护，通过 `/api/notify/*` 或 UI 配置；生日 / 事项 / 12306Bypass 等模块运行时按通道名引用即可。

cron 使用 `robfig/cron/v3` 的秒级格式，必须是 6 段，例如：

```text
0 0 9 * * *   # 每天 09:00:00
0 0 3 * * *   # 每天 03:00:00
```

## 本地开发

启动后端，让 Homer 先把 API、数据库和后台任务跑起来：

```sh
export PATH=/opt/module/go/go1.25.0/bin:$PATH
go run .
```

后端默认监听 `.env` 中的 `SERVER_PORT`，模板默认是 `8081`。

启动前端，打开操作台：

```sh
cd frontend
npm install
npm run dev
```

Vite 默认监听 `0.0.0.0:5173`，并将 `/api` 代理到 `http://localhost:8081`。开发态调试 UI 时访问：

```text
http://localhost:5173
```

## 常用命令

后端测试：

```sh
go test ./...
```

前端检查：

```sh
cd frontend
npm run lint
npm run build
```

构建单二进制：

```sh
cd frontend
npm install
npm run build

cd ..
export PATH=/opt/module/go/go1.25.0/bin:$PATH
go build -ldflags="-s -w" -o bin/server .
```

运行：

```sh
./bin/server
```

如需注入版本信息：

```sh
PKG=github.com/LemonZuo/homer/internal/buildinfo
go build \
  -trimpath \
  -ldflags="-s -w -X ${PKG}.Version=v0.1.0 -X ${PKG}.Commit=$(git rev-parse --short HEAD) -X ${PKG}.BuildID=$(git rev-parse HEAD | head -c 12)" \
  -o bin/server .
```

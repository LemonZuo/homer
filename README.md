# Homer 🏠

把容易忘的事、容易过期的证书、容易散落在各处的设备操作，交给一个小管家盯着。

Homer 是一个自用的小管家工具集，用来跑主动型、轻量级的后台任务和外部服务联动。它像一张常驻值班台：该提醒的时候提醒，该续期的时候续期，该转发的时候转发。

适合放进 Homer 的东西通常有三个特征：会重复、会忘、忘了会有点麻烦。

## 功能菜单 ✨

- 生日提醒：记住谁快过生日，顺手算好农历生日和生肖，到点通过企业微信提醒。
- 事项提醒：体检、续费、办事截止日这类一次性日期，提前几天开始提醒，同一天不重复吵你。
- 调度面板：看每个任务几点跑、上次跑得怎样；需要时也可以点一下“现在就跑”。
- ACME 证书管理：维护 ACME 账号、DNS provider 凭证、域名、签发任务、证书产物和续期任务。
- 证书部署：证书签好以后，可以继续送到 SSH 机器、雷池 SafeLine、阿里云 CAS 或飞牛 fnOS，不用手动复制粘贴；失败的部署任务有定时重试和手动重试。
- 阿里云 CAS / CDN：查看 CAS 证书库存、删除证书、把证书一键部署到 CDN，加速域名只读查看。
- 短信转发器：对接 SmsForwarder Android，查询配置、发送短信、查短信记录都走一个页面。
- 12306Bypass webhook 转发：接收分流抢票助手 webhook，再转发到企业微信和 Resend 邮件。

## 不做什么

- 不做通用账号保险柜：Homer 更关心“什么时候该主动做点事”。
- 不做复杂工作流平台：Homer 更偏“一台机器上跑几个靠谱的小任务”。
- 不内置公网鉴权：默认跑在可信网络里，公网访问应放在反向代理、VPN、网关或其他鉴权层之后。

## 技术栈

- 后端：Go 1.25+、Gin、GORM、MySQL、robfig/cron。
- 前端：React 19、Vite、TypeScript、Tailwind CSS v4、react-router-dom、axios、lucide-react。
- 部署：前端构建产物通过 `//go:embed all:frontend/dist` 嵌入 Go 二进制，最终可单二进制运行。

## 目录结构

```text
homer/
├── main.go                 # 入口，组装配置、数据库、服务、路由和调度器
├── wire.go                 # ACME / scheduler 依赖组装
├── internal/
│   ├── acme/               # ACME 签发、续期、部署（SSH/SafeLine/alicas/fnos）、SSE 日志
│   ├── aliyun/             # 阿里云 SDK 客户端封装
│   ├── birthday/           # 生日提醒任务
│   ├── cas/                # 阿里云 CAS 证书查询/删除/部署到 CDN
│   ├── cdn/                # 阿里云 CDN 加速域名查询
│   ├── chinesedate/        # 农历/生肖换算
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

## 环境要求

- Go 1.25 或更新版本。本地已有固定路径时可使用：

```sh
export PATH=/opt/module/go/go1.25.0/bin:$PATH
```

- Node.js 20 或更新版本。
- MySQL 8.x 或兼容版本。

## 配置 🧭

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

## 数据库初始化

首次部署时先准备好数据库，再导入 SQL。这里相当于把值班台、档案柜和任务清单先摆好：

```sh
mysql -uroot -p -e "CREATE DATABASE IF NOT EXISTS homer DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;"
mysql -uroot -p homer < sql/00_schema.sql
```

注意：`sql/00_schema.sql` 大部分表使用 `DROP TABLE IF EXISTS`，适合全新初始化。已有数据的环境不要直接重跑，应先按 SQL 内容整理增量迁移。这条提醒不有趣，但很重要。

`sql/0X_*.sql` 是增量迁移脚本（命名按时间顺序），全部用存储过程做幂等保护，可重复执行；新装库时按编号依次跑一遍即可。

当前后端启动时只会自动迁移 `sms_forwarder`，其他业务表依赖 `sql/` 里的建表语句。

## 本地开发 🛠️

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

## Docker

想少敲几行命令，可以直接用 `docker compose` 部署。仓库里的 `docker-compose.yml` 会读取 `.env`，并把它只读挂载到容器内 `/app/.env`，应用启动时也能通过 `godotenv.Load()` 读到同一份配置。

1. 准备配置：

```sh
cp .env.example .env
```

编辑 `.env`，至少填好这些项：

```dotenv
IMAGE=ghcr.io/lemonzuo/homer:latest
HOST_PORT=8081
SERVER_PORT=8081

DB_HOST=192.168.1.10
DB_PORT=3306
DB_USER=root
DB_PASSWORD=your-password
DB_NAME=homer
DB_CHARSET=utf8mb4
```

注意：容器里的 `127.0.0.1` 指向容器自己。如果 MySQL 跑在宿主机或其他机器上，`DB_HOST` 应填写宿主机可被容器访问的地址、局域网 IP、Docker 网络里的服务名，或你自己的数据库地址。

2. 初始化数据库：

```sh
mysql -h 192.168.1.10 -P 3306 -u root -p homer < sql/00_schema.sql
```

把示例里的 host、port、user 和库名替换成你的实际值；`-p` 会让 MySQL 客户端交互式询问密码。再次提醒：`sql/00_schema.sql` 大部分表使用 `DROP TABLE IF EXISTS`，只适合全新库初始化。

3. 启动服务：

```sh
mkdir -p data
docker compose pull
docker compose up -d
```

启动后访问：

```text
http://localhost:8081
```

如果改过 `HOST_PORT`，访问端口以 `.env` 里的值为准。

4. 查看状态和日志：

```sh
docker compose ps
docker compose logs -f homer
```

5. 更新镜像：

```sh
docker compose pull
docker compose up -d
```

当前 compose 会持久化这些内容：

| 宿主机路径 | 容器路径 | 用途 |
| --- | --- | --- |
| `./.env` | `/app/.env` | 应用配置，只读挂载 |
| `./data` | `/app/data` | ACME 工作目录、账号私钥、签发相关落盘数据 |

默认镜像为：

```text
ghcr.io/lemonzuo/homer:latest
```

本地构建镜像时，Dockerfile 期望存在 release 产物：

```text
dist/server-linux-amd64
dist/server-linux-arm64
```

这些文件通常由 GitHub Actions release 流程生成。

## API 概览

常用 API 前缀均为 `/api`：

| 路径 | 说明 |
| --- | --- |
| `GET /api/version` | 当前版本和 commit |
| `GET /api/tables` | 前端通用表配置 |
| `/api/birthday` | 生日提醒 CRUD |
| `POST /api/birthday/:id/notify` | 手动推送单条生日提醒 |
| `/api/event` | 事项提醒 CRUD |
| `POST /api/event/:id/notify` | 手动推送单条事项提醒 |
| `GET /api/scheduler/jobs` | 调度任务列表 |
| `POST /api/scheduler/jobs/:name/run` | 手动触发调度任务 |
| `/api/notify/*` | 通知通道配置 |
| `GET /api/cdn/domains` | 阿里云 CDN 加速域名 |
| `/api/cas/certificates` | 阿里云 CAS 证书列表和删除（全局凭证） |
| `POST /api/cas/deploy` | 将 CAS 证书部署到 CDN |
| `/api/acme/accounts` | ACME 账号 CRUD |
| `/api/acme/credentials` 、`/api/acme/ssh-credentials` | DNS provider 与 SSH 凭证 |
| `/api/acme/domains` | 域名 CRUD、签发、吊销、证书查询 |
| `/api/acme/{ssh,safeline,cas,fnos}-targets` | 各类部署目标 CRUD + 连通性测试 |
| `/api/acme/domains/:id/{ssh,safeline,cas,fnos}-deploy-configs` | 域名 → 部署目标的绑定 |
| `/api/acme/deploy/configs/:id/deploy` | 触发单个部署 |
| `/api/acme/tasks` 、 `POST /api/acme/tasks/:id/retry` 、 `GET /api/acme/tasks/:id/stream` | 任务历史、手动重试、SSE 实时日志 |
| `/api/sms/*` | SmsForwarder 配置、发送、查询 |
| `POST /api/byPass/receive` | 12306Bypass webhook 接收入口 |

## ACME 使用要点 🔐

1. 先创建 ACME 账号，也就是告诉 Homer 去哪里签证书。支持 Let's Encrypt、ZeroSSL 或自定义 ACME directory。ZeroSSL 的 EAB 信息存储在数据库中。
2. 再创建 DNS provider 凭证，也就是告诉 Homer 怎么完成 DNS-01 校验。已实现深度校验的 provider 包括 `cloudflare`、`dnspod`、`alidns`、`tencentcloud`、`huaweicloud`。
3. 创建域名配置，选择 ACME 账号和 DNS provider；密钥类型默认走全局 `ACME_KEY_TYPE`。
4. 手动签发一次确认链路可用，之后交给 `ACME_RENEW_CRON` 自动检查临期证书。
5. 需要部署时，配置部署目标和部署配置。当前支持 SSH、雷池 SafeLine、阿里云 CAS（`alicas`）、飞牛 fnOS（`fnos`，SSH 覆盖 ssls 时间戳目录 + psql 更新 trim_connect.cert + 重启 trim_nginx）四种 driver。
6. 部署失败会先按 `ACME_DEPLOY_RETRY` 在同次任务内重试；仍失败的会被 `ACME_DEPLOY_RETRY_CRON` 定时捞起来补，也可以在 UI 的任务历史里手动重试。

ACME 签发产物既会写入数据库，也会使用 `ACME_DATA_DIR` 作为本地工作目录。生产部署时应确保该目录持久化。

## 发布

仓库包含 GitHub Actions release 工作流：

- 推送 `v*` tag 时构建前端、Linux amd64 / arm64 二进制、多架构 Docker 镜像，并创建 GitHub Release。
- 手动触发 workflow 时构建 `dev-<sha>` 版本镜像。
- release 二进制通过 `-X github.com/LemonZuo/homer/internal/buildinfo.Version`、`Commit`（短 git hash）和 `BuildID`（每次构建唯一短 hash，区分同 commit 的多次构建）注入版本信息，前端会展示 `/api/version` 返回的版本号。

## 部署注意事项

- 本项目默认面向可信网络环境，当前没有内置登录鉴权。暴露到公网前应放在反向代理、VPN、内网网关或其他鉴权层之后。
- `.env`、数据库、`ACME_DATA_DIR` 中会包含访问密钥、证书私钥和 SSH 凭证，不应提交到仓库。
- `frontend/dist` 被 Go embed 引用。fresh clone 时目录需要存在；前端 `npm run build` 会在构建后补回 `dist/.gitkeep`。
- SPA 路由由后端 `NoRoute` 兜底处理，非 `/api/*` 请求会回退到前端页面。

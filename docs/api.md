# API 与 ACME 使用

这里收集 HTTP API 概览，以及 ACME 模块的使用流程要点。

回到[项目首页](../README.md)。

## API 概览

常用 API 前缀均为 `/api`：

| 路径 | 说明 |
| --- | --- |
| `GET /api/version` | 当前版本和 commit |
| `/api/birthday` | 生日提醒 CRUD |
| `POST /api/birthday/:id/notify` | 手动推送单条生日提醒 |
| `/api/event` | 事项提醒 CRUD |
| `POST /api/event/:id/notify` | 手动推送单条事项提醒 |
| `GET /api/scheduler/jobs` | 调度任务列表 |
| `POST /api/scheduler/jobs/:name/run` | 手动触发调度任务 |
| `/api/notify/*` | 通知通道配置 |
| `GET /api/cdnops/domains` | 阿里云 CDN 加速域名 |
| `/api/certstore/certificates` | 阿里云 CAS 证书列表和删除（全局凭证） |
| `POST /api/certstore/deploy` | 将 CAS 证书部署到 CDN |
| `/api/acme/accounts` | ACME 账号 CRUD |
| `/api/acme/credentials` 、`/api/acme/ssh-credentials` | DNS provider 与 SSH 凭证 |
| `/api/acme/domains` | 域名 CRUD、签发、吊销、证书查询 |
| `/api/acme/deploy/targets` 、 `POST /api/acme/deploy/targets/:id/test` | 部署目标 CRUD（kind 区分 ssh/safeline/cas/fnos）+ 连通性测试 |
| `/api/acme/domains/:id/deploy-configs` | 域名 → 部署目标的绑定 |
| `POST /api/acme/domains/:id/deploy-configs/deploy` | 按域名触发部署 |
| `/api/acme/tasks` 、 `POST /api/acme/tasks/:id/retry` 、 `GET /api/acme/tasks/:id/stream` | 任务历史、手动重试、SSE 实时日志 |
| `/api/sms/*` | SmsForwarder 配置、发送、查询 |
| `POST /api/byPass/receive` | 12306Bypass webhook 接收入口 |

## ACME 使用要点

1. 先创建 ACME 账号，也就是告诉 Homer 去哪里签证书。支持 Let's Encrypt、ZeroSSL 或自定义 ACME directory。ZeroSSL 的 EAB 信息存储在数据库中。
2. 再创建 DNS provider 凭证，也就是告诉 Homer 怎么完成 DNS-01 校验。已实现深度校验的 provider 包括 `cloudflare`、`dnspod`、`alidns`、`tencentcloud`、`huaweicloud`。
3. 创建域名配置，选择 ACME 账号和 DNS provider；密钥类型默认走全局 `ACME_KEY_TYPE`。
4. 手动签发一次确认链路可用，之后交给 `ACME_RENEW_CRON` 自动检查临期证书。
5. 需要部署时，配置部署目标和部署配置。当前支持 SSH、雷池 SafeLine、阿里云 CAS（`alicas`）、飞牛 fnOS（`fnos`，SSH 覆盖 ssls 时间戳目录 + psql 更新 trim_connect.cert + 重启 trim_nginx）四种 driver。
6. 部署失败会先按 `ACME_DEPLOY_RETRY` 在同次任务内重试；仍失败的会被 `ACME_DEPLOY_RETRY_CRON` 定时捞起来补，也可以在 UI 的任务历史里手动重试。

ACME 签发产物既会写入数据库，也会使用 `ACME_DATA_DIR` 作为本地工作目录。生产部署时应确保该目录持久化。

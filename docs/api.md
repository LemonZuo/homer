# API 与 ACME

本文涵盖 Homer 的 HTTP 接口、SSE 流、通知通道，以及 ACME 模块的使用流程要点。

回到[项目首页](../README.md)。

## 通用约定

- **前缀**：所有业务接口均挂载于 `/api` 之下。健康检查使用 `/healthz`，不计入 Gin access log。
- **CORS**：默认全开（`Access-Control-Allow-Origin: *`），允许 `GET/POST/PUT/DELETE/OPTIONS`，便于本地 Vite (`:5173`) 直接访问开发后端 (`:8081`)。
- **响应**：成功响应统一为 JSON；错误响应形如 `{"error":"..."}`。`POST/PUT` 请求体默认为 JSON。
- **认证**：Homer 自身不提供登录鉴权，公网访问应置于反代 + 鉴权层之后（参见 [docs/deployment.md](deployment.md)）。
- **未配置降级**：依赖外部凭证的模块（CDN、CAS、通知通道）AK/SK 或绑定缺失时，相关接口会返回明确错误（如 `阿里云 CDN 未配置`），而非 500。

## 健康检查与版本

| Method | Path | 说明 |
| --- | --- | --- |
| `GET` | `/healthz` | 探活；DB 不通 503，其余 200。Body：`{"status":"ok\|degraded","db":"ok\|<err>","scheduler":{"jobs":N,"running":M,"failing":K}}` |
| `GET` | `/api/version` | 返回 `{ version, commit, build_id }`，来自 `internal/buildinfo`（ldflags 注入） |

## 生日 / 事项

两个模块结构一致：标准 CRUD + 「手动推送」辅助操作。

### 生日 (`/api/birthday`)

| Method | Path | 说明 |
| --- | --- | --- |
| `GET` | `/api/birthday` | 列表 |
| `POST` | `/api/birthday` | 新建 |
| `PUT` | `/api/birthday/:id` | 更新 |
| `DELETE` | `/api/birthday/:id` | 删除 |
| `POST` | `/api/birthday/:id/notify` | 立即推送一次（按当前绑定的通道） |

### 事项 (`/api/event`)

| Method | Path | 说明 |
| --- | --- | --- |
| `GET` | `/api/event` | 列表 |
| `POST` | `/api/event` | 新建 |
| `PUT` | `/api/event/:id` | 更新 |
| `DELETE` | `/api/event/:id` | 删除 |
| `POST` | `/api/event/:id/notify` | 立即推送一次，并刷新 `last_notified_at`（避免当天 cron 再次重复） |

## 调度面板 (`/api/scheduler`)

| Method | Path | 说明 |
| --- | --- | --- |
| `GET` | `/api/scheduler/jobs` | 列出所有 job：cron、上次执行时间/结果、连续失败次数 |
| `POST` | `/api/scheduler/jobs/:name/run` | 手动触发；`:name` 取下方任务名 |

进程内当前注册 8 个 job：

| name | cron 来源 | 默认值 | 行为 |
| --- | --- | --- | --- |
| `birthday` | `BIRTHDAY_REMIND_CRON` | `0 0 9 * * *` | 扫描启用的生日，命中当日则推送 |
| `event` | `EVENT_REMIND_CRON` | `0 0 9 * * *` | 扫描启用的事项，命中提醒窗口则推送（同日去重） |
| `acme-renew` | `ACME_RENEW_CRON` | `0 0 3 * * *` | 对剩余天数 ≤ `ACME_RENEW_BEFORE_DAYS` 的证书发起续期 |
| `acme-deploy-retry` | `ACME_DEPLOY_RETRY_CRON` | `0 * * * * *` | 扫描 `status='retrying' AND next_retry_at<=now` 的部署任务并择时触发 |
| `ups-sample` | `UPS_SAMPLE_CRON` | `*/30 * * * * *` | 对每台 `ups_host(enabled='1')` 执行一轮 `upsc` 采样并更新 `ups_state` |
| `ups-cleanup` | `UPS_CLEANUP_CRON` | `0 0 4 * * *` | 按 `UPS_RETENTION_DAYS` 清理 `ups_sample` |
| `esxi-sample` | `ESXI_SAMPLE_CRON` | `*/30 * * * * *` | 对每台 `esxi_host(enabled='1')` 执行一轮 esxcli/vsish/vim-cmd 采样，差集触发阈值告警 |
| `esxi-cleanup` | `ESXI_CLEANUP_CRON` | `0 0 4 * * *` | 按 `ESXI_RETENTION_DAYS` 清理 `esxi_sample` |

将对应 env **设为空字符串** 即可将该 job 改为「仅手动触发」（`/api/scheduler/jobs/:name/run`）。`ups-sample` / `esxi-sample` 还受 `ROUTER_MAINTENANCE_WINDOW` 影响：窗口内返回 `ErrSkipped`，**不计成功也不计失败**，避免路由器/网关定时重启引发的离线误报。

## 通知 (`/api/notify`)

通知通道与模块绑定均持久化于数据库，运行时按模块名查询 `notify_binding` 表，**改动即时生效**。

| Method | Path | 说明 |
| --- | --- | --- |
| `GET` | `/api/notify/meta` | 返回所有 channel type + module 的 schema，供前端动态生成表单 |
| `GET` | `/api/notify/channels` | 通道列表 |
| `POST` | `/api/notify/channels` | 新建 |
| `PUT` | `/api/notify/channels/:id` | 更新 |
| `DELETE` | `/api/notify/channels/:id` | 删除；若仍存在模块绑定则报错 |
| `POST` | `/api/notify/channels/:id/test` | 使用该通道立即发送一条测试消息 |
| `GET` | `/api/notify/bindings` | 返回 `{ module: [channel_id...] }` |
| `PUT` | `/api/notify/bindings/:module` | 整体覆盖某模块的通道绑定（事务：先删后插） |

### 支持的 channel type

| type | Label | `config_json` 字段 |
| --- | --- | --- |
| `wework` | 企业微信 | `corp_id`, `agent_id`, `secret`, `tag_id` |
| `email` | Resend 邮件 | `api_key`, `from`, `to` |
| `webhook` | Webhook | `url`（POST `application/json`，body `{title, text}`，10s 超时） |

> 所有通道出站均包裹「重试 3 次，2s/4s 退避」逻辑；多通道并存时单路失败不影响其余通道，全部失败时合并为 `errors.Join`。

### 可绑定模块

| module key | 用途 |
| --- | --- |
| `birthday` | 生日提醒 |
| `event` | 事项提醒 |
| `bypass` | 12306Bypass 转发 |
| `scheduler_alert` | 调度任务失败告警（阈值 `SCHEDULER_ALERT_FAIL_THRESHOLD`） |
| `ups` | UPS 状态告警（离线/电池/低电状态机） |
| `esxi` | ESXi 阈值告警（CPU 温度 / CPU 使用率 / 内存使用率 / 磁盘温度 / datastore 使用率；差集语义） |

## CDN 加速域名 (`/api/cdnops`)

| Method | Path | 说明 |
| --- | --- | --- |
| `GET` | `/api/cdnops/domains` | 列出阿里云 CDN 加速域名（只读）；AK/SK 见 `ALIYUN_CDN_*` |

## 证书库存 (`/api/certstore`)

阿里云 CAS 证书的只读浏览 + 部署到 CDN（使用全局 `ALIYUN_CAS_*` AK/SK；ACME 内部使用 `alicas` driver 时另有一套 per-target AK/SK）。

| Method | Path | 说明 |
| --- | --- | --- |
| `GET` | `/api/certstore/certificates` | CAS 证书列表 |
| `DELETE` | `/api/certstore/certificates/:id` | 从 CAS 删除证书 |
| `POST` | `/api/certstore/deploy` | body `{certName}`；按 CAS 证书的「绑定域名」匹配 CDN 加速域名并下发，触发后立即返回 |



## SMS (`/api/sms`)

对接 SmsForwarder Android（多目标设备：`forwarders` 表登记设备凭证；其他接口通过 `target_id` 选择设备）。

| Method | Path | body |
| --- | --- | --- |
| `GET` | `/api/sms/forwarders` | 列设备 |
| `POST` | `/api/sms/forwarders` | 新增 |
| `PUT` | `/api/sms/forwarders/:id` | 更新 |
| `DELETE` | `/api/sms/forwarders/:id` | 删除 |
| `POST` | `/api/sms/config/query` | `{target_id}` 拉远端配置 |
| `POST` | `/api/sms/send` | `{target_id, sim_slot, phone_numbers, msg_content}` |
| `POST` | `/api/sms/query` | `{target_id, type, page_num, page_size, keyword}` |

## 12306Bypass Webhook (`/api/byPass`)

| Method | Path | 说明 |
| --- | --- | --- |
| `POST` | `/api/byPass/receive` | 分流抢票助手回调入口；按 `bypass` 模块绑定的通道扇出（典型场景：企业微信 + Resend 邮件） |

## UPS 监控 (`/api/ups`)

UPS 采样与 ACME 完全解耦，机器/凭证使用独立的 `ups_host` / `ups_ssh_credential` 两张表。

| Method | Path | 说明 |
| --- | --- | --- |
| `GET` | `/api/ups/snapshot` | 当前所有启用 UPS 的最新状态快照 |
| `GET` | `/api/ups/stream` | **SSE** 实时推送 snapshot 变更（替代轮询） |
| `GET` | `/api/ups/series` | 历史曲线，query：`host_kind=ups`、`host_id`、`ups_name`（NUT 设备名，必填）、`range=24h/7d`（默认 24h）。三个必填项任一缺失则返回 400 |
| `POST` | `/api/ups/refresh` | 立即触发一轮采样（等价于 `POST /api/scheduler/jobs/ups-sample/run`，但经 service 路径执行） |
| `GET` | `/api/ups/hosts` | UPS 机器列表（独立于 ACME 部署目标） |
| `POST` | `/api/ups/hosts` | 新建机器 |
| `PUT` | `/api/ups/hosts/:id` | 更新机器 |
| `DELETE` | `/api/ups/hosts/:id` | 删除机器 |
| `POST` | `/api/ups/hosts/:id/toggle` | 切换 enabled 开关（即时影响下一轮采样） |
| `POST` | `/api/ups/hosts/:id/test` | 连通性测试：建立 SSH 连接 + `upsc -l` 探测 |
| `GET / POST / PUT / DELETE` | `/api/ups/credentials`（`:id`） | UPS 专用 SSH 凭证（独立于 ACME 的 `ssh-credentials`） |

## ESXi 监控 (`/api/esxi`)

接口形态与 UPS 完全对称，机器/凭证使用 `esxi_host` / `esxi_ssh_credential`。

| Method | Path | 说明 |
| --- | --- | --- |
| `GET` | `/api/esxi/snapshot` | 当前所有 ESXi 主机的最新状态快照（平台/CPU/内存/温度/磁盘/MCE/USB/VM/网络拓扑） |
| `GET` | `/api/esxi/stream` | **SSE** 实时推送 snapshot 变更 |
| `GET` | `/api/esxi/series` | 历史曲线，query：`host_kind=esxi`、`host_id`、`range=1h/6h/24h/3d/7d`。当前 7 个 metric：CPU 各核温度 / 各盘温度 / CPU 使用率 / 内存已用 GiB / 各盘已用 GiB / 运行 VM 数 / MCE 累计 |
| `POST` | `/api/esxi/refresh` | 立即触发一轮采样 |
| `GET` | `/api/esxi/hosts` | ESXi 机器列表 |
| `POST` | `/api/esxi/hosts` | 新建机器 |
| `PUT` | `/api/esxi/hosts/:id` | 更新机器 |
| `DELETE` | `/api/esxi/hosts/:id` | 删除机器 |
| `POST` | `/api/esxi/hosts/:id/toggle` | 切换 enabled 开关 |
| `POST` | `/api/esxi/hosts/:id/test` | 连通性测试：建立 SSH 连接 + 执行探测命令 |
| `GET / POST / PUT / DELETE` | `/api/esxi/credentials`（`:id`） | ESXi 专用 SSH 凭证 |

阈值告警采用「新增超阈值」差集语义：连续 `ESXI_ALERT_CONSECUTIVE_SAMPLES` 轮命中阈值才推送一次，已在告警集合内的项不重复推送。详见 [development.md#esxi-监控](development.md#esxi-监控) 中的阈值清单。

## ACME (`/api/acme`)

按子模块分组。所有 `POST` 请求体形如 `{"name":..., "config_json":..., "auth_json":...}`，具体字段以前端表单为准。

### Providers

| Method | Path | 说明 |
| --- | --- | --- |
| `GET` | `/api/acme/providers` | 列出支持的 DNS provider 及其 schema |

### CA 账号

| Method | Path |
| --- | --- |
| `GET` | `/api/acme/accounts` |
| `POST` | `/api/acme/accounts` |
| `PUT` | `/api/acme/accounts/:id` |
| `DELETE` | `/api/acme/accounts/:id` |

### DNS 凭证 & SSH 凭证

| Method | Path | 说明 |
| --- | --- | --- |
| `GET / POST` | `/api/acme/credentials` `/api/acme/credentials/:id`（`DELETE`） | DNS provider 凭证 |
| `GET / POST / PUT / DELETE` | `/api/acme/ssh-credentials`（`:id`） | SSH 凭证（账号 + 密码/私钥 + passphrase） |

### 部署目标 (Targets)

| Method | Path | 说明 |
| --- | --- | --- |
| `GET` | `/api/acme/deploy/targets` | 列目标 |
| `POST` | `/api/acme/deploy/targets` | 新建（按 `kind` 校验 schema） |
| `PUT` | `/api/acme/deploy/targets/:id` | 更新 |
| `DELETE` | `/api/acme/deploy/targets/:id` | 删除 |
| `POST` | `/api/acme/deploy/targets/:id/test` | 连通性测试（SSH 建立连接、雷池 `ListCerts`、CAS `ListUserCertificateOrder` 等） |

### 部署配置 (Configs，对应「域名 × 目标」绑定)

| Method | Path | 说明 |
| --- | --- | --- |
| `PUT` | `/api/acme/deploy/configs/:id` | 更新 |
| `DELETE` | `/api/acme/deploy/configs/:id` | 删除 |
| `POST` | `/api/acme/deploy/configs/:id/deploy` | 仅部署这一条 |

### 域名

| Method | Path | 说明 |
| --- | --- | --- |
| `GET` | `/api/acme/domains` | 列域名 |
| `POST` | `/api/acme/domains` | 新建（选 CA 账号 + DNS provider + 密钥类型） |
| `PUT` | `/api/acme/domains/:id` | 更新 |
| `DELETE` | `/api/acme/domains/:id` | 删除 |
| `GET` | `/api/acme/domains/:id/cert` | 查当前证书（链信息 + SAN + 剩余天数） |
| `GET` | `/api/acme/domains/:id/cert/download` | 下载证书包（zip） |
| `POST` | `/api/acme/domains/:id/issue` | 手动签发一次（异步：生成 `acme_issue_task`） |
| `POST` | `/api/acme/domains/:id/revoke` | 吊销证书 |
| `GET` | `/api/acme/domains/:id/deploy-configs` | 列出该域名下所有部署配置 |
| `POST` | `/api/acme/domains/:id/deploy-configs` | 新建一条「域名 × 目标」绑定 |
| `POST` | `/api/acme/domains/:id/deploy-configs/deploy` | 批量触发该域名下所有配置 |

### 任务（含 SSE）

| Method | Path | 说明 |
| --- | --- | --- |
| `GET` | `/api/acme/tasks` | 任务历史，query：`page`, `page_size`, `status=pending/running/success/failed/retrying` |
| `GET` | `/api/acme/tasks/:id` | 任务详情 |
| `POST` | `/api/acme/tasks/:id/retry` | 手动重试 |
| `GET` | `/api/acme/tasks/:id/stream` | **SSE** 实时日志（见下） |

### SSE 日志

`/api/acme/tasks/:id/stream` 使用 `text/event-stream`，断线不自动重连，需客户端自行处理。事件类型（data 均为纯字符串，非 JSON）：

| event | data | 说明 |
| --- | --- | --- |
| `log` | 一行日志文本 | 任务执行过程中追加 |
| `done` | 终态字符串：`success` / `failed` / `retrying` | 收到后即可关闭流 |

> 服务端已带 `X-Accel-Buffering: no` 头，nginx 默认会识别并关闭缓冲；为保险起见，反代仍建议显式 `proxy_buffering off` + 放宽 `proxy_read_timeout`（[deployment.md](deployment.md#nginx-反向代理) 中的示例已包含相应配置）。

## ACME 部署 driver 详解

ACME 内部目前注册了 4 个 driver，新增 driver 在 `wire.go::buildACMEService` 的 `NewDeployRegistry` 中添加一行即可。

### `ssh` — SSH 机器

`auth_json`（继承自 `sshlike.TargetAuth`）：

| 字段 | 说明 |
| --- | --- |
| `auth_source` | `inline`（默认）或 `credential`（引用 `ssh-credentials`） |
| `credential_id` | `credential` 模式必填 |
| `username` | |
| `auth_type` | `password` 或 `key` |
| `password` | password 模式 |
| `private_key` / `passphrase` | key 模式 |

`endpoint`：`host:port`，无端口默认 22。

`target.config_json`：

| 字段 | 说明 |
| --- | --- |
| `bastion_target_id` | 单跳跳板机，指向另一条 `ssh`/`fnos` 目标；不允许自指 |

`deploy.config_json`：

| 字段 | 说明 |
| --- | --- |
| `cert_path` / `chain_path` / `fullchain_path` | 三者至少 1 个 |
| `key_path` | 必填 |
| `deploy_command` | 可选；上传完成后执行（支持 `{domain}` 占位符） |

行为：SFTP 写入 → 执行 `deploy_command` → `TestTarget` 仅校验连通性。

### `safeline` — 雷池 WAF

`auth_json`：

| 字段 | 说明 |
| --- | --- |
| `api_token` | 雷池后台生成的 API token |

`endpoint`：雷池 BaseURL，必须 `http(s)://` 开头。

`target.config_json`：

| 字段 | 说明 |
| --- | --- |
| `skip_tls_verify` | 自签证书场景可设为 true |

`deploy.config_json`：

| 字段 | 说明 |
| --- | --- |
| `cert_type` | 雷池侧证书类型 ID，默认 2 |

`deploy.state_json`：`{cert_id, cert_ids[]}`，记录上次落地的雷池 cert ID，用于下次原地更新。

行为：若已有 `cert_id` 则直接 `UpsertCert(cert_id, …)`；否则 `ListCerts` → 按 SAN 匹配 → 命中的逐个更新，未命中则新增。

### `upload_cas` — 阿里云 CAS

`auth_json`：

| 字段 | 说明 |
| --- | --- |
| `access_key_id` | per-target，独立于全局 `ALIYUN_CAS_*` |
| `access_key_secret` | |

`endpoint` / `target.config_json` / `deploy.config_json`：均未使用。

`deploy.state_json`：`{cert_id}`，记录最近一次上传的 CAS 证书 ID。

行为：`UploadUserCertificate(Name=时间戳, Cert=fullchain, Key=key)`，**每次均新增**（CAS API 不支持原地更新）。`TestTarget` 调用 `ListUserCertificateOrder` 验证 AK/SK。

### `fnos` — 飞牛 OS

`auth_json` / `endpoint` / `target.config_json`：与 `ssh` driver 完全一致（共享 `sshlike`）；`config.bastion_target_id` 同样支持单跳，且不展开跳板机自身的 bastion 链。

`deploy.config_json`：

| 字段 | 说明 |
| --- | --- |
| `domain_override` | 可选；留空时取 `domain.main_domain` |

行为（脚本固化，路径/库名/服务名内置）：

1. SFTP 写入 `/tmp/homer-fnos-<certID>.{crt,key}`
2. 远端 bash：`install -m 0644/0600` 覆盖至 `/usr/trim/var/trim_connect/ssls/<domain>/<最新时间戳目录>/{<domain>.crt, <domain>.key}`
3. `sudo -u postgres psql -d trim_connect`：CTE `UPDATE cert SET valid_from/valid_to/last_renew_time/updated_time/issued_by/encrypt_type/status='suc' WHERE domain=? AND source='upload'`，要求且仅命中 1 行
4. `sudo systemctl restart trim_nginx`

`TestTarget`：连通性 + `command -v psql` + `test -d /usr/trim/var/trim_connect/ssls`。

## ACME 使用流程要点

1. **CA 账号**：指定 Homer 向何处申请证书。支持 Let's Encrypt、ZeroSSL（EAB 存于数据库）或自定义 ACME directory。
2. **DNS provider 凭证**：用于完成 DNS-01 校验。已实现深度校验的 provider：`cloudflare` / `dnspod` / `alidns` / `tencentcloud` / `huaweicloud`，其余通过 lego 通用 provider 实现。
3. **域名配置**：绑定 CA + provider；密钥类型默认沿用全局 `ACME_KEY_TYPE`，也可按单域名覆盖。
4. **手动签发一次** 验证链路通畅：`POST /api/acme/domains/:id/issue` → 通过 `tasks/:id/stream` 查看实时日志。
5. **续期**：`ACME_RENEW_CRON` 定时扫描，剩余天数 ≤ `ACME_RENEW_BEFORE_DAYS` 时触发续期。亦可在 UI 中手动签发覆盖。
6. **部署**：先创建一条 deploy target（4 种 kind 任选其一），再在域名详情中添加 deploy config（域名 × target 多对多）。签发完成后会自动按所有启用配置执行一次部署。
7. **失败重试**：单次部署 → 同任务内按 `ACME_DEPLOY_RETRY` 立即重试 → 仍失败则标记为 `retrying` → `acme-deploy-retry` cron 按 `ACME_DEPLOY_RETRY_BACKOFF_SEC * 已执行次数` 退避触发；UI 任务历史中亦可手动重试。

ACME 签发产物双路落地：数据库 `acme_cert` 表 + `ACME_DATA_DIR` 本地工作目录（lego 私钥、acme.json 等）。生产部署务必持久化该目录。

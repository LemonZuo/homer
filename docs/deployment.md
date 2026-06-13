# 部署

本文涵盖 Homer 的部署步骤：数据库初始化、Docker compose、Nginx 反向代理（含 tinyauth）、发布流程、升级与备份、故障排查。

回到[项目首页](../README.md)。

> Homer 设计为「单进程单二进制」，部署模型有两种主路径：
>
> - **Docker compose**（推荐）：拉取镜像、挂载 `.env` 与 `data/` 卷，配合 tinyauth 做前置登录；适合家用 NAS 与小型 VPS。
> - **裸机二进制**：直接运行 `bin/server`，由 systemd 托管；适合已有反代体系的环境。
>
> 两种路径下数据库初始化、Nginx 反代、备份注意事项都通用。

## 数据库初始化

首次部署需先完成数据库与表的初始化，再启动 Homer。

### 全新部署

```sh
mysql -uroot -p -e "CREATE DATABASE IF NOT EXISTS homer DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;"
mysql -uroot -p homer < sql/00_schema.sql
```

`sql/00_schema.sql` 大部分建表语句以 `DROP TABLE IF EXISTS` 开头，**仅适用于空库**。已有数据的库重新执行会清空表，须改走「增量」路径。

### 已有数据的升级

按编号顺序执行增量脚本：

```sh
for f in sql/0[1-9]_*.sql; do
  echo ">>> $f"
  mysql -uroot -p homer < "$f"
done
```

所有 `0X_*.sql` 都用存储过程做幂等保护，可重复执行无副作用。当前增量脚本清单见 [development.md#sql-与-automigrate双轨](development.md#sql-与-automigrate双轨)。

### AutoMigrate

后端启动时会对 22 张业务表执行 `AutoMigrate`（含 ACME / 通知 / 调度 / SMS / UPS / ESXi 全系列），仅做**追加式**变更（加列、加索引），不会执行 drop。此层为兜底，避免因漏执行迁移导致启动失败。**真正的 schema 变更仍须落到 `sql/0X_*.sql`**，不可仅依赖 AutoMigrate。

## Docker Compose

仓库 `docker-compose.yml` 包含两个服务：

- `homer`：业务本体，监听 `8081`。配置来自 `.env`（以只读方式挂载到容器 `/app/.env`），由 `godotenv.Load()` 自动加载。
- `tinyauth`：前置登录网关，监听 `3000`。Homer 本体不内置鉴权，公网入口建议置于反代之后，由 tinyauth 拦截鉴权后回源。tinyauth 配置直接写在 compose 文件中，不读取 `.env`。

### 1. 准备 `.env`

```sh
cp .env.example .env
```

最少要填的字段（其它走默认即可，完整清单见 [development.md#配置env](development.md#配置env)）：

```dotenv
IMAGE=ghcr.io/lemonzuo/homer:latest
HOST_PORT=8081
TZ=Asia/Shanghai

SERVER_PORT=8081
DB_HOST=192.168.1.10
DB_PORT=3306
DB_USER=root
DB_PASSWORD=your-password
DB_NAME=homer
DB_CHARSET=utf8mb4
```

> 容器内的 `127.0.0.1` 指向容器自身而非宿主机。MySQL 部署于宿主机或其他主机时，`DB_HOST` 必须填写宿主机 IP、局域网 IP 或 Docker 网络服务名等容器可访问的地址。

### 2. 配置 tinyauth

编辑 `docker-compose.yml` 的 `tinyauth.environment`：

| 字段 | 生成方式 |
| --- | --- |
| `SECRET` | `openssl rand -hex 16`（32 字节随机串） |
| `USERS` | `docker run --rm httpd:alpine htpasswd -nbB admin '你的密码'`，输出原样填，**每个 `$` 改写成 `$$`**（compose 双写转义） |
| `APP_URL` | tinyauth 自身对外访问入口，反代后通常是 `https://auth.example.com`，用于登录回跳 |

### 3. 数据库初始化

```sh
mysql -h 192.168.1.10 -P 3306 -u root -p homer < sql/00_schema.sql
```

`-p` 触发交互式询问密码。注意：`00_schema.sql` 仅用于空库。

### 4. 启动

```sh
mkdir -p data data/tinyauth
docker compose pull
docker compose up -d
```

启动后访问：

```text
http://localhost:8081   # homer（直连，绕开鉴权）
http://localhost:3000   # tinyauth 登录页
```

公网部署时 **homer 的 8081 端口必须收回内网**，由反代单独从 `127.0.0.1` 回源（见下方 Nginx 段）。

### 5. 状态与日志

```sh
docker compose ps
docker compose logs -f homer
docker compose logs -f tinyauth
```

### 6. 持久化卷

| 宿主机路径 | 容器路径 | 用途 |
| --- | --- | --- |
| `./.env` | `/app/.env` | homer 应用配置（只读） |
| `./data` | `/app/data` | homer ACME 工作目录、账号私钥、签发产物（**必须持久化**） |
| `./data/tinyauth` | `/data` | tinyauth 用户会话 |

> 业务数据本身（生日、事项、ACME 元信息等）存储于 MySQL，备份策略以数据库为主；`./data` 主要是 ACME 的 lego 工作目录，丢失会导致下一次签发时重新注册账号（功能仍可正常工作，但 CA 侧会产生新的账号记录）。

### 7. 升级镜像

```sh
docker compose pull
docker compose up -d
```

`up -d` 检测到镜像变化会重建容器。建议升级前：

1. **备份数据库**（`mysqldump`），跨 minor 版本升级前尤其重要。
2. **检查 `sql/` 目录是否新增 0X 增量脚本**：升级到新版本后，对照 `git diff` 查看 `sql/` 是否新增 `0X_*.sql`；若有，先在数据库上执行完再 `up -d`。AutoMigrate 仅为兜底，schema 调整（例如索引去重）单靠它会出现不一致。
3. 重启后通过 `docker compose logs -f homer | head -50` 确认 `homer starting → connect db → migrate → register scheduler job → http listening` 序列完整。

### 8. 镜像

默认镜像：

```text
ghcr.io/lemonzuo/homer:latest
```

本地构建（`docker build`）依赖 release 流程已经产出的 `dist/server-linux-amd64` 与 `dist/server-linux-arm64`。Dockerfile 不从源码编译，而是基于这两个二进制构建多架构 manifest，日常无须自行构建。

## 裸机二进制部署（可选）

若不引入 Docker，亦可直接运行 `bin/server`：

1. 在本地或 CI 构建好二进制（见 [development.md#常用命令](development.md#常用命令)）。
2. scp 到目标机：

   ```sh
   scp bin/server user@host:/opt/homer/bin/server
   scp .env       user@host:/opt/homer/.env
   ```

3. 一份最简 systemd unit（`/etc/systemd/system/homer.service`）：

   ```ini
   [Unit]
   Description=Homer
   After=network-online.target mysql.service
   Wants=network-online.target

   [Service]
   Type=simple
   User=homer
   WorkingDirectory=/opt/homer
   EnvironmentFile=/opt/homer/.env
   ExecStart=/opt/homer/bin/server
   Restart=on-failure
   RestartSec=5s
   # 让 ACME_DATA_DIR 走绝对路径或 WorkingDirectory 下，避免 systemd 启动目录不一致
   AmbientCapabilities=CAP_NET_BIND_SERVICE

   [Install]
   WantedBy=multi-user.target
   ```

4. 启用：

   ```sh
   systemctl daemon-reload
   systemctl enable --now homer
   journalctl -u homer -f
   ```

升级时停服 → 替换 `bin/server` → 跑必要的 `sql/0X_*.sql` → `systemctl restart homer`。

## Nginx 反向代理

将 homer 与 tinyauth 置于 nginx 之后、由 tinyauth 拦截 homer 入口的最小模板。两个域名各一个 server 块；homer 这一段通过 `auth_request` 调用 tinyauth 的鉴权接口，401 时重定向至 tinyauth 登录页。SSL 证书路径与上游端口（与 compose 保持一致：homer `8081` / tinyauth `3000`）按实际调整。

```nginx
# 1. tinyauth 登录页本身
server {
    listen 443 ssl;
    server_name auth.example.com;

    ssl_certificate     /etc/nginx/certs/auth.example.com.pem;
    ssl_certificate_key /etc/nginx/certs/auth.example.com.key;

    location / {
        proxy_pass http://127.0.0.1:3000;
        proxy_set_header Host              $host;
        proxy_set_header X-Real-IP         $remote_addr;
        proxy_set_header X-Forwarded-For   $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}

# 2. homer 主入口,经 tinyauth 鉴权后转发
server {
    listen 443 ssl;
    server_name homer.example.com;

    ssl_certificate     /etc/nginx/certs/homer.example.com.pem;
    ssl_certificate_key /etc/nginx/certs/homer.example.com.key;

    location / {
        auth_request /auth;
        error_page 401 = @login;

        proxy_pass http://127.0.0.1:8081;
        proxy_set_header Host              $host;
        proxy_set_header X-Real-IP         $remote_addr;
        proxy_set_header X-Forwarded-For   $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

        # ACME 任务详情用了 SSE,需要关掉缓冲并放宽超时
        proxy_buffering    off;
        proxy_read_timeout 1h;
    }

    # 内部 auth 子请求,转发给 tinyauth
    location = /auth {
        internal;
        proxy_pass http://127.0.0.1:3000/api/auth/nginx;
        proxy_set_header Host              $host;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header X-Forwarded-Host  $host;
        proxy_set_header X-Forwarded-Uri   $request_uri;
        proxy_pass_request_body off;
        proxy_set_header Content-Length "";
    }

    # 未登录跳到 tinyauth 登录页,登录成功后回到原 URL
    location @login {
        return 302 https://auth.example.com/login?redirect_uri=$scheme://$host$request_uri;
    }
}
```

> tinyauth 的鉴权 endpoint（示例中写的 `/api/auth/nginx`）与登录回跳参数名以 tinyauth 当前版本文档为准，存在差异时按其文档替换即可。
> 启用反代后，宿主机不应将 homer 的 `8081` 端口对公网开放；由 nginx 经环回地址（`127.0.0.1`）单独回源，tinyauth 的 `3000` 同理。

### SSE 相关的反代细节

`/api/acme/tasks/:id/stream` 是 SSE 长连接，需要：

- `proxy_buffering off`：nginx 默认会缓存响应至一定字节后再下发，会使实时日志延迟到任务结束后才一次性返回。
- `proxy_read_timeout 1h`：默认 60s，长签发任务（DNS 传播 + 部署）会被中断。
- 该路径不应启用 gzip（多数 nginx 默认不会对 `text/event-stream` 压缩，但若存在全局压缩配置需将其排除）。

## 健康检查

| 路径 | 用途 |
| --- | --- |
| `GET /healthz` | DB ping + scheduler 概览。Body：`{"status":"ok\|degraded","db":"ok\|<err>","scheduler":{"jobs":N,"running":M,"failing":K}}`；DB 不通时 503，**调度器状态仅展示，不影响存活判定**（任一 job 失败不会让 healthz 红） |
| `GET /api/version` | 当前 `version / commit / build_id`，可用于「滚动升级是否生效」检查 |
| `GET /api/scheduler/jobs` | 调度任务详情（cron、上次执行时间、连续失败次数）；监控任务失败应使用此接口而非 `/healthz` |

接入 uptime 监控时建议同时监测两条：`/healthz` 用于探活，`/api/version` 用于验证版本回滚是否生效。

## 发布

仓库包含 GitHub Actions release 工作流：

- **`v*` tag 推送**：构建前端 → Linux amd64 / arm64 二进制 → 多架构 Docker 镜像 → 创建 GitHub Release。
- **手动触发 workflow**：构建 `dev-<sha>` 版本镜像，不发 release。
- release 二进制通过 ldflags 注入 3 个字段（前端 `/api/version` 展示）：
  - `Version`：tag 名（如 `v0.4.4`）
  - `Commit`：短 git hash
  - `BuildID`：每次构建唯一短 hash，区分同 commit 的多次构建

### Tag 规则

patch 号不进位到两位：保持个位（0~9），满 9 后进位到 minor 并归零 patch。  
即 `v0.1.9` 的下一个 tag 是 `v0.2.0`，不是 `v0.1.10`。

## 备份建议

| 对象 | 优先级 | 方式 |
| --- | --- | --- |
| MySQL `homer` 库 | ★★★ | `mysqldump --single-transaction --routines homer > homer-$(date +%Y%m%d).sql`，至少每日 |
| `./data/acme/` | ★★ | rsync / tar；丢失会让下次签发重新注册 ACME 账号，但不影响功能 |
| `./data/tinyauth/` | ★ | 仅会话数据，丢失就强制重登 |
| `.env` | ★★★ | 含 AK/SK 和 DB 密码，单独存密码管理器，**不入 git** |

升级前的「最小回滚集」：DB dump 与 `.env` 即可重建实例。

## 部署注意事项

- 本项目默认面向可信网络环境，**没有内置登录鉴权**。暴露到公网前必须放在反向代理、VPN、内网网关或 tinyauth/Authelia 等鉴权层之后。
- `.env`、数据库、`ACME_DATA_DIR` 中包含访问密钥、证书私钥、SSH 凭证，不应提交到仓库（已被 `.gitignore` 覆盖，push 前仍需复核 `git status`）。
- `frontend/dist` 被 Go embed 引用。新克隆时该目录必须存在；`npm run build` 会在构建后补回 `dist/.gitkeep`，确保下次克隆时 embed 不会因目录为空失败。
- SPA 路由由后端 `NoRoute` 兜底处理，非 `/api/*` 请求回退至前端页面；nginx 无须为前端单独配置 `try_files`。

## 故障排查

| 现象 | 检查点 |
| --- | --- |
| 启动直接 fatal `connect db` | 检查 `.env` MySQL 连接五项、防火墙、`utf8mb4` collation、`DB_HOST` 在容器内是否可达 |
| 启动 fatal `migrate` | 老库字段冲突，先执行对应 `sql/0X_*.sql` 将表结构对齐 |
| `bind: address already in use` | `SERVER_PORT` / `HOST_PORT` 被占用，使用 `lsof -i :8081` 排查或更换端口 |
| `/api/cdnops/domains` / `/api/certstore/*` 返回 503 | 对应 `ALIYUN_*` AK/SK 未配置 |
| 通知 API 报「企业微信未配置」 | 通道未创建或未绑定对应模块，至 UI 通知页配置 |
| SSE 日志停滞 / 任务结束后才一次性返回 | nginx 缺少 `proxy_buffering off`；或前端为 `https://` 但反代回源使用 `http://` 触发 mixed content |
| 调度任务连续失败但无告警 | `SCHEDULER_ALERT_FAIL_THRESHOLD` 设置过大；或 `scheduler_alert` 模块未绑定通道 |
| 升级后界面未更新 | 浏览器强制刷新（`Cmd+Shift+R`）；或检查 `/api/version` 的 `build_id` 是否更新 |
| 容器内 `DB_HOST=127.0.0.1` 连不上 | 容器自身并非宿主机，应改为宿主机 IP / `host.docker.internal` / Docker 网络服务名 |

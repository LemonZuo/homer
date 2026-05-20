# 部署

这里收集 Homer 的部署相关步骤：数据库初始化、Docker compose、Nginx 反向代理（含 tinyauth）、发布流程，以及一些值得提醒的事。

回到[项目首页](../README.md)。

## 数据库初始化

首次部署时先准备好数据库，再导入 SQL。这里相当于把值班台、档案柜和任务清单先摆好：

```sh
mysql -uroot -p -e "CREATE DATABASE IF NOT EXISTS homer DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;"
mysql -uroot -p homer < sql/00_schema.sql
```

注意：`sql/00_schema.sql` 大部分表使用 `DROP TABLE IF EXISTS`，适合全新初始化。已有数据的环境不要直接重跑，应先按 SQL 内容整理增量迁移。这条提醒不有趣，但很重要。

`sql/0X_*.sql` 是增量迁移脚本（命名按时间顺序），全部用存储过程做幂等保护，可重复执行；新装库时按编号依次跑一遍即可。

当前后端启动时只会自动迁移 `sms_forwarder`，其他业务表依赖 `sql/` 里的建表语句。

## Docker

想少敲几行命令，可以直接用 `docker compose` 部署。仓库里的 `docker-compose.yml` 包含两个服务：

- `homer`：业务本体，监听 `8081`。配置走 `.env`（只读挂载到容器 `/app/.env`），`godotenv.Load()` 会自动读到。
- `tinyauth`：前置登录网关，监听 `3000`。Homer 不内置鉴权，公网入口建议挂在反代后面、由 tinyauth 做登录拦截，再回源到 homer。配置直接写在 compose 里，不读 `.env`。

1. 准备 homer 配置：

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

2. 配置 tinyauth：直接编辑 `docker-compose.yml` 中 `tinyauth.environment` 三个占位值：

- `SECRET`：32 字节随机串，例如 `openssl rand -hex 16`。
- `USERS`：`user:bcrypt-hash` 形式。可用下面的 docker 命令生成：

  ```sh
  docker run --rm httpd:alpine htpasswd -nbB admin '你的密码'
  ```

  把输出原样填进 `USERS`，**每个 `$` 都改写成 `$$`**——compose 会把 `$xxx` 当变量插值，双写后才是字面 `$`，容器内仍是单 `$`。

- `APP_URL`：tinyauth 自身对外访问入口（反代后通常是 `https://auth.example.com`），用于登录回跳。

3. 初始化数据库：

```sh
mysql -h 192.168.1.10 -P 3306 -u root -p homer < sql/00_schema.sql
```

把示例里的 host、port、user 和库名替换成你的实际值；`-p` 会让 MySQL 客户端交互式询问密码。再次提醒：`sql/00_schema.sql` 大部分表使用 `DROP TABLE IF EXISTS`，只适合全新库初始化。

4. 启动服务：

```sh
mkdir -p data data/tinyauth
docker compose pull
docker compose up -d
```

启动后访问：

```text
http://localhost:8081   # homer（直接访问，绕开鉴权）
http://localhost:3000   # tinyauth 登录页
```

公网部署时应由反代仅暴露 tinyauth，homer 端口收回内网，由反代鉴权通过后回源到 homer 容器。

5. 查看状态和日志：

```sh
docker compose ps
docker compose logs -f homer
docker compose logs -f tinyauth
```

6. 更新镜像：

```sh
docker compose pull
docker compose up -d
```

当前 compose 会持久化这些内容：

| 宿主机路径 | 容器路径 | 用途 |
| --- | --- | --- |
| `./.env` | `/app/.env` | homer 应用配置，只读挂载 |
| `./data` | `/app/data` | homer ACME 工作目录、账号私钥、签发相关落盘数据 |
| `./data/tinyauth` | `/data` | tinyauth 用户会话等持久化数据 |

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

## Nginx 反向代理

下面是把 homer 和 tinyauth 放在 nginx 后面、用 tinyauth 拦截 homer 入口的最小模板。两个域名各一个 server 块，homer 这块通过 `auth_request` 调 tinyauth 的鉴权接口，401 时重定向到 tinyauth 登录页。SSL 证书路径、上游端口（与 compose 一致：homer `8081` / tinyauth `3000`）按实际改。

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

> tinyauth 的鉴权 endpoint(示例里写的 `/api/auth/nginx`)和登录回跳参数名以 tinyauth 当前版本文档为准,有差异时按其文档替换即可。
> 启用反代后，宿主机不应把 homer 的 `8081` 端口对公网开放，由 nginx 走环回(`127.0.0.1`)单独回源；tinyauth 的 `3000` 同理。

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

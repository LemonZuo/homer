# Homer 🏠

把容易忘的事、容易过期的证书、容易散落在各处的设备操作，交给一个小管家盯着。

Homer 是一个自用的小管家工具集，用来跑主动型、轻量级的后台任务和外部服务联动。它像一张常驻值班台：该提醒的时候提醒，该续期的时候续期，该转发的时候转发。

适合放进 Homer 的东西通常有三个特征：会重复、会忘、忘了会有点麻烦。

## 功能菜单 ✨

- 生日提醒：记住谁快过生日，顺手算好农历生日和生肖，到点通过企业微信提醒。
- 事项提醒：体检、续费、办事截止日这类一次性日期，提前几天开始提醒，同一天不重复吵你。
- 调度面板：看每个任务几点跑、上次跑得怎样；需要时也可以点一下"现在就跑"。
- ACME 证书管理：维护 ACME 账号、DNS provider 凭证、域名、签发任务、证书产物和续期任务。
- 证书部署：证书签好以后，可以继续送到 SSH 机器、雷池 SafeLine、阿里云 CAS 或飞牛 fnOS，不用手动复制粘贴；失败的部署任务有定时重试和手动重试。
- 阿里云 CAS / CDN：查看 CAS 证书库存、删除证书、把证书一键部署到 CDN，加速域名只读查看。
- 短信转发器：对接 SmsForwarder Android，查询配置、发送短信、查短信记录都走一个页面。
- 12306Bypass webhook 转发：接收分流抢票助手 webhook，再转发到企业微信和 Resend 邮件。
- UPS 监控：SSH 拉取 NUT `upsc`，市电/电池/低电状态机告警，SSE 实时推送，历史曲线。
- ESXi 监控：SSH 跑 esxcli/vsish/vim-cmd 采平台、CPU/内存使用率、CPU 温度、磁盘 SMART、容量用量、MCE、USB、虚拟机、网络拓扑（vSwitch / uplink / portgroup / VM-NIC，用 React Flow 在前端画机箱式拓扑图）；阈值告警（去抖：只在连续超阈值时推送）。

## 不做什么

- 不做通用账号保险柜：Homer 更关心“什么时候该主动做点事”。
- 不做复杂工作流平台：Homer 更偏“一台机器上跑几个靠谱的小任务”。
- 不内置公网鉴权：默认跑在可信网络里，公网访问应放在反向代理、VPN、网关或其他鉴权层之后。

## 技术栈

- 后端：Go 1.25+、Gin、GORM、MySQL、robfig/cron。
- 前端：React 19、Vite、TypeScript、Tailwind CSS v4、react-router-dom、axios、lucide-react。
- 部署：前端构建产物通过 `//go:embed all:frontend/dist` 嵌入 Go 二进制，最终可单二进制运行。

## 文档导航

按场景拆成了三份，按需查阅：

- [docs/deployment.md](docs/deployment.md) — 部署：数据库初始化、Docker compose、Nginx + tinyauth 反代、发布流程、部署注意事项。
- [docs/development.md](docs/development.md) — 配置与开发：环境要求、`.env` 配置项详表、目录结构、本地开发、常用命令、单二进制构建。
- [docs/api.md](docs/api.md) — API 与 ACME：HTTP API 概览、ACME 模块使用流程要点。

## Quick start

最小可跑起来的路径，详细说明见上方文档。

```sh
# 1. 准备 MySQL 和 .env
cp .env.example .env
mysql -uroot -p -e "CREATE DATABASE IF NOT EXISTS homer DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;"
mysql -uroot -p homer < sql/00_schema.sql

# 2. 启动后端（默认监听 :8081）
export PATH=/opt/module/go/go1.25.0/bin:$PATH
go run .

# 3. 启动前端（:5173，/api 已代理到 :8081）
cd frontend && npm install && npm run dev
```

容器化部署直接看 [docs/deployment.md](docs/deployment.md) 里的 Docker 段。

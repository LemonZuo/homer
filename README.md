# Homer

Homer 是一个自用的小管家工具集，承担主动型、轻量级的后台任务与外部服务联动：到期事项的提醒、证书的自动续期与部署、设备的状态采集与告警。

它面向的是三类容易被遗忘的工作：周期性任务、临期一次性事项、需要主动巡检的设备。

## 功能列表

- 生日提醒：维护生日名单，自动计算农历生日与生肖，到期通过企业微信推送提醒。
- 事项提醒：体检、续费、办事截止等一次性日期，提前若干天开始推送，同一天去重。
- 调度面板：查看各调度任务的执行时间、最近一次执行结果，支持手动触发。
- ACME 证书管理：管理 ACME 账号、DNS provider 凭证、域名、签发任务、证书产物与续期任务。
- 证书部署：证书签发后自动分发至 SSH 主机、雷池 SafeLine、阿里云 CAS、飞牛 fnOS；部署失败支持定时重试与手动重试。
- 阿里云 CAS / CDN：查看 CAS 证书库存、删除证书、一键部署至 CDN，加速域名只读查询。
- 短信转发：对接 SmsForwarder Android 端，统一管理配置、发送短信、查阅短信记录。
- 12306Bypass webhook 中转：接收分流抢票助手 webhook，转发至企业微信与 Resend 邮件。
- UPS 监控：通过 SSH 拉取 NUT `upsc`，按市电 / 电池 / 低电状态机产生告警，SSE 实时推送，提供历史曲线。
- ESXi 监控：通过 SSH 执行 esxcli / vsish / vim-cmd，采集平台信息、CPU / 内存使用率、CPU 温度、磁盘 SMART、容量用量、MCE、USB 直通、虚拟机及网络拓扑（vSwitch / uplink / portgroup / VM-NIC，前端使用 React Flow 渲染机箱式拓扑图）；历史曲线覆盖 CPU 各核温度、各盘温度、CPU 使用率、内存已用 GiB、各盘已用 GiB、运行 VM 数、MCE 累计；阈值告警在连续 N 次超阈值后触发，仅对新增超阈项推送，已通知项不重复。

## 不在范围

- 不做通用账号保险柜：Homer 聚焦于主动型任务，被动凭据存取不在范围。
- 不做复杂工作流平台：Homer 倾向于在单机内运行少量可靠的轻量任务。
- 不内置公网鉴权：默认运行于可信内网，公网访问应由反向代理、VPN、网关等鉴权层前置。

## 技术栈

- 后端：Go 1.25+、Gin、GORM、MySQL、robfig/cron。
- 前端：React 19、Vite、TypeScript、Tailwind CSS v4、react-router-dom、axios、lucide-react。
- 部署：前端构建产物通过 `//go:embed all:frontend/dist` 嵌入 Go 二进制，最终以单二进制方式运行。

## 文档导航

按场景拆分，按需查阅：

- [docs/deployment.md](docs/deployment.md) — 部署：数据库初始化、Docker compose、Nginx + tinyauth 反代、发布流程、部署注意事项。
- [docs/development.md](docs/development.md) — 配置与开发：环境要求、`.env` 配置项详表、目录结构、本地开发、常用命令、单二进制构建。
- [docs/api.md](docs/api.md) — API 与 ACME：HTTP API 概览、ACME 模块使用流程要点。
- [docs/ESXI.md](docs/ESXI.md) — ESXi 数据采集详解：`internal/esximon` 通过 SSH 执行的命令、真实输出样例、解析方式、重试与超时策略。
- [docs/UT1050EGC.md](docs/UT1050EGC.md) — NUT `upsc` 字段含义对照表（以 CyberPower UT1050EGC 为样本），排查 UPS 状态时备查。

## Quick start

最小启动路径，详细说明参见上方文档。

```sh
# 1. 准备 MySQL 和 .env
cp .env.example .env
mysql -uroot -p -e "CREATE DATABASE IF NOT EXISTS homer DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;"
mysql -uroot -p homer < sql/00_schema.sql

# 2. 启动后端（默认监听 :8081）
export PATH=/opt/module/go/go1.25.0/bin:$PATH
go run .

# 3. 启动前端（:5173，/api 默认代理至 :8081）
cd frontend && npm install && npm run dev
```

容器化部署参见 [docs/deployment.md](docs/deployment.md) 中的 Docker 章节。

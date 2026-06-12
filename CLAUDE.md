# CLAUDE.md

本文件为 Claude Code 提供本仓库的上下文，便于后续协作时快速进入状态。

## 项目定位

个人「小管家」型工具集，跑一些**主动型**的轻量任务，例如：

- 证书管理（ACME 自动续期、部署到 SSH / SafeLine / 阿里云 CAS / 飞牛 fnOS、失败重试；阿里云 CAS 证书归档、CDN 加速域名只读查看）
- 生日 / 纪念日提醒
- SmsForwarder 短信转发、12306Bypass webhook 中转
- UPS 状态监控（SSH 拉取 NUT `upsc`，状态机告警 + SSE 推送 + 历史曲线；机器/凭证独立于 ACME）
- ESXi 监控（SSH 跑 esxcli/vsish/vim-cmd 采集硬件/温度/磁盘 SMART/容量用量/MCE/USB/VM；阈值告警走"新增超阈值"语义不重复推送）
- 其他偶尔会扩进来的自用小功能

## 技术栈

- **后端**: Go 1.25+，Gin + GORM + MySQL 驱动；通过 `.env`（godotenv）加载配置
- **前端**: React 19 + Vite + TypeScript + Tailwind CSS v4 + react-router-dom + axios + lucide-react + @xyflow/react(ESXi 网络拓扑)
- **风格**: 卡片式现代极简（浅色为主，深色模式自动跟随系统），桌面 + 移动响应式
- **部署**: 单二进制 —— `//go:embed all:frontend/dist` 把前端产物打入 Go binary

## 目录结构

```
homer/
├── main.go
├── wire.go          # ACME / scheduler 等依赖组装
├── go.mod / go.sum
├── .env / .env.example
├── internal/
│   ├── acme/        # 签发、续期、SSE；子包 deployer/{ssh,safeline}、providers/
│   ├── aliyun/      # 阿里云 SDK 客户端封装
│   ├── birthday/、event/   # 生日 / 事项提醒任务
│   ├── buildinfo/   # 版本/commit 注入
│   ├── certstore/、cdnops/  # 阿里云 CAS 证书库存、CDN 加速域名查询与 CAS→CDN 部署
│   ├── chinesedate/ # 农历/生肖
│   ├── config/、db/
│   ├── handler/     # 业务专用 handler 单独文件（含 acme/ 子包；通用 CRUD 已删除）
│   ├── jobmonitor/  # 任务失败计数 + 告警门槛
│   ├── logx/        # slog 结构化日志封装
│   ├── model/       # 业务模型（按模块拆分文件即可）
│   ├── notify/、sms/
│   ├── router/、scheduler/
│   ├── sshlike/   # 公共 SSH 拨号(acme/upsmon/esximon 共用，原 acme/deployer/sshlike）
│   ├── sshx/      # SSH 跳板/中转工具（原 acme/deployer/sshx，提升到顶层）
│   ├── upsmon/    # UPS：sampler(SSH+upsc) + hoststore/credstore（独立表）+ service(状态机/SSE) + handler
│   ├── esximon/   # ESXi：client(esxcli/vsish/vim-cmd 重试+合并) + sampler + alert(阈值差集) + service(SSE) + hoststore/credstore + handler；网络拓扑(vSwitch/uplink/portgroup/VM-NIC) 也在 sampler 合并
│   └── web/
├── sql/             # 00 全量建表 + 0X_* 增量迁移（幂等存储过程，当前到 18）
└── frontend/
    ├── dist/        # vite 产物，被 go:embed 引用
    └── src/
        ├── main.tsx / App.tsx   # 入口；路由 + 懒加载页面映射
        ├── api.ts / colors.ts
        ├── pages.ts     # 页面/导航登记表（每页自包含，无声明式 CRUD）
        ├── components/  # 顶层放各页面入口（Esxi.tsx/Ups.tsx 现已瘦身到 ~50 行），按模块拆子目录：
        │                #   acme/ sms/ ui/ icons/
        │                #   esxi/{host-cards,history,topology,shared 等}（NetTopologyFlow 用 @xyflow/react）
        │                #   ups/{history,cards,management 等}
        ├── lib/、assets/
        └── index.css
```

## 开发

```sh
# 后端（端口默认 8080，可在 .env 的 SERVER_PORT 改）
export PATH=/opt/module/go/go1.25.0/bin:$PATH
cp .env.example .env   # 填好 MySQL 连接
go run .

# 前端（端口默认 :5173；Vite 已把 /api 代理到 :8080）
cd frontend && npm install && npm run dev
```

开发态前端走 Vite dev server，后端 SPA handler 在 `frontend/dist` 为空时只返回占位页 —— 调试 UI 始终用 :5173。

## 打包（单二进制部署）

`main.go` 顶部的 `//go:embed all:frontend/dist` 把 vite 产物嵌入 Go 二进制，`router.NoRoute` 把非 `/api/*` 的请求交给 SPA handler，未命中文件时回退 `index.html` 给 React Router。

```sh
# 1. 前端构建（输出到 frontend/dist，vite 默认位置）
cd frontend && npm install && npm run build

# 2. 后端构建（embed 进 bin/server，单文件分发）
cd ..
export PATH=/opt/module/go/go1.25.0/bin:$PATH
go build -ldflags="-s -w" -o bin/server .   # -s -w 去掉符号表/调试信息，砍 30%+ 体积

# 运行
./bin/server   # 同时提供 /api 和 / （前端）
```

`npm run build` 末尾会自动补回 `frontend/dist/.gitkeep`（vite `emptyOutDir` 会清掉），保证下次 fresh clone 时 `//go:embed` 不至于因目录为空报错。

## 版本 tag 规则

- patch 号不进位到两位：保持个位（0~9），满 9 后进位到 minor 并归零 patch。
- 即 `v0.1.9` 的下一个 tag 是 `v0.2.0`，不是 `v0.1.10`。

## 注意事项

- go.mod 要求 Go ≥ 1.25，开发用 `/opt/module/go/go1.25.0`
- 默认面向可信网络环境；如证书等模块需要回连外网，应明确文档化端口暴露范围
- 后台任务（cron / 续期 worker / 部署重试等）集中在 `internal/scheduler/`，主进程内启动，单 binary 不引入额外组件；任务失败计数走 `internal/jobmonitor/`
- 日志统一走 `internal/logx`（slog 结构化），级别由 `LOG_LEVEL` env 控制
- schema 变更走「`sql/0X_*.sql` 增量脚本 + GORM AutoMigrate」双轨：两边都要补，迁移脚本用存储过程做幂等
- Tailwind 用的是 v4（`@import "tailwindcss";`），没有 `tailwind.config.js`，配置通过 CSS 变量即可

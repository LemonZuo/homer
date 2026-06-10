# AGENTS.md

## Response Language

除非用户另有说明，请使用中文回答。

## Project Overview

这是一个个人「小管家」型工具集，用于运行主动型轻量任务，包括：

- 证书管理：ACME 自动续期、部署到 SSH / SafeLine / 阿里云 CAS / 飞牛 fnOS、失败重试；阿里云 CAS 证书归档；CDN 加速域名只读查看
- 生日 / 纪念日提醒
- SmsForwarder 短信转发、12306Bypass webhook 中转
- UPS 状态监控：SSH 调用 NUT(`upsc`)采样，市电/电池/低电状态转换告警，SSE 实时推送，历史曲线；机器与凭证表独立于 ACME
- ESXi 状态监控：SSH 跑 esxcli/vsish/vim-cmd 采平台/CPU/内存/温度/磁盘 SMART 与容量用量/MCE/USB/VM；阈值告警仅在新增超阈值时推送
- 其他自用小功能

## Tech Stack

- 后端：Go 1.25+，Gin，GORM，MySQL driver，`.env` 通过 godotenv 加载
- 前端：React 19，Vite，TypeScript，Tailwind CSS v4，react-router-dom，axios，lucide-react
- UI 风格：卡片式现代极简，浅色为主，深色模式自动跟随系统，桌面和移动端响应式
- 部署：单二进制，通过 `//go:embed all:frontend/dist` 将前端产物嵌入 Go binary

## Repository Layout

```text
homer/
├── main.go
├── wire.go
├── go.mod / go.sum
├── .env / .env.example
├── internal/
│   ├── acme/
│   ├── aliyun/
│   ├── birthday/、event/
│   ├── buildinfo/
│   ├── certstore/、cdnops/
│   ├── chinesedate/
│   ├── config/、db/
│   ├── handler/
│   ├── jobmonitor/
│   ├── logx/
│   ├── model/
│   ├── notify/、sms/
│   ├── router/、scheduler/
│   ├── upsmon/
│   ├── esximon/
│   └── web/
├── sql/
└── frontend/
    ├── dist/
    └── src/
        ├── main.tsx / App.tsx
        ├── api.ts / colors.ts
        ├── pages.ts
        ├── components/
        ├── lib/
        └── index.css
```

## Development

后端开发：

```sh
export PATH=/opt/module/go/go1.25.0/bin:$PATH
cp .env.example .env
go run .
```

前端开发：

```sh
cd frontend
npm install
npm run dev
```

开发态前端使用 Vite dev server，默认端口 `:5173`，并代理 `/api` 到后端 `:8080`。后端 SPA handler 在 `frontend/dist` 为空时只返回占位页，因此调试 UI 时优先使用 Vite dev server。

## Build

单二进制构建流程：

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

`npm run build` 末尾会补回 `frontend/dist/.gitkeep`，避免 fresh clone 时 `//go:embed` 因目录为空失败。

## Implementation Notes

- go.mod 要求 Go 1.25+，本机开发优先使用 `/opt/module/go/go1.25.0`
- 默认面向可信网络环境；证书等模块如果需要回连外网，应明确文档化端口暴露范围
- 后台任务集中在 `internal/scheduler/`，在主进程内启动，不额外引入组件
- 任务失败计数和告警门槛走 `internal/jobmonitor/`
- 日志统一使用 `internal/logx` 的 slog 结构化日志封装，级别由 `LOG_LEVEL` 控制
- schema 变更必须同时补：
  - `sql/0X_*.sql` 增量迁移脚本，使用存储过程保持幂等
  - GORM AutoMigrate 对应模型
- Tailwind 使用 v4，通过 `@import "tailwindcss";` 和 CSS 变量配置；不要新增 `tailwind.config.js`，除非确有必要
- 前端页面登记在 `frontend/src/pages.ts`，业务页面保持自包含，不恢复声明式 CRUD 模式
- Go 业务 handler 放在 `internal/handler/`，按业务拆分文件；ACME 相关逻辑可放在 `internal/handler/acme/`
- 新增共享逻辑时优先遵循现有包边界，不做无关重构

## Version Tags

版本 tag 的 patch 号保持个位，满 9 后进位到 minor 并归零 patch。

示例：`v0.1.9` 的下一个 tag 是 `v0.2.0`，不是 `v0.1.10`。

## Agent Workflow

- 改动前先查看相关文件和 `git status --short`，不要覆盖用户已有改动
- 搜索文件或文本优先使用 `rg` / `rg --files`
- 手写文件改动优先使用 `apply_patch`
- 不要执行 `git reset --hard`、`git checkout --` 等破坏性命令，除非用户明确要求
- 后端改动完成后尽量运行相关 Go 测试或至少 `go test ./...`
- 前端改动完成后尽量运行 `npm run build` 或更小范围的类型检查 / lint
- 如果测试因环境、数据库、网络或缺少依赖失败，需要在最终回复中说明具体原因

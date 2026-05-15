# CLAUDE.md

本文件为 Claude Code 提供本仓库的上下文，便于后续协作时快速进入状态。

## 项目定位

个人「小管家」型工具集，跑一些**主动型**的轻量任务，例如：

- 证书管理（ACME 自动续期、CAS 证书归档、加速域名管理）
- 生日 / 纪念日提醒
- 其他偶尔会扩进来的自用小功能

姊妹项目 `account-vault` 负责**被动型**的凭证 CRUD；这里专门放需要后台调度、外部交互的功能，二者分仓维护以避免概念混淆。

## 技术栈

- **后端**: Go 1.25+，Gin + GORM + MySQL 驱动；通过 `.env`（godotenv）加载配置
- **前端**: React 19 + Vite + TypeScript + Tailwind CSS v4 + react-router-dom + axios + lucide-react
- **风格**: 卡片式现代极简（浅色为主，深色模式自动跟随系统），桌面 + 移动响应式
- **部署**: 单二进制 —— `//go:embed all:frontend/dist` 把前端产物打入 Go binary

## 目录结构

```
homer/
├── main.go
├── go.mod / go.sum
├── .env / .env.example
├── internal/
│   ├── buildinfo/   # 版本/commit 注入
│   ├── config/
│   ├── db/
│   ├── model/       # 业务模型（按模块拆分文件即可）
│   ├── handler/     # crud.go 通用 CRUD；业务专用 handler 单独文件
│   ├── router/
│   └── web/
└── frontend/
    ├── dist/        # vite 产物，被 go:embed 引用
    └── src/
        ├── App.tsx
        ├── api.ts
        ├── tables.ts
        ├── components/
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

## 注意事项

- go.mod 要求 Go ≥ 1.25，开发用 `/opt/module/go/go1.25.0`
- 默认面向可信网络环境；如证书等模块需要回连外网，应明确文档化端口暴露范围
- 后台任务（cron / 续期 worker 等）建议集中在 `internal/scheduler/`（待建），主进程内启动，单 binary 不引入额外组件
- Tailwind 用的是 v4（`@import "tailwindcss";`），没有 `tailwind.config.js`，配置通过 CSS 变量即可

package web

import (
	"io"
	"io/fs"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

const placeholder = `<!DOCTYPE html>
<html lang="zh-CN"><head><meta charset="utf-8"><title>Account Vault</title>
<style>body{font-family:system-ui,sans-serif;max-width:520px;margin:4rem auto;padding:0 1.5rem;color:#222;line-height:1.55}code{background:#f4f4f5;padding:2px 6px;border-radius:4px;font-family:ui-monospace,Menlo,monospace}</style>
</head><body>
<h2>前端尚未构建</h2>
<p>请在 <code>frontend/</code> 目录执行 <code>npm install &amp;&amp; npm run build</code>，然后重新启动后端。</p>
</body></html>`

// SPAHandler 把传入的 fs.FS（通常是 main.go 里 //go:embed frontend/dist 后 fs.Sub 出来的）做成 Gin handler：
//   - 命中真实文件 → 按 MIME 返回（走 http.FileServer）
//   - 未命中且不是 /api/* → 直接返回 index.html 字节（前端路由由 React Router 接管）
//   - dist 为空（未构建）→ 返回友好占位页
func SPAHandler(dist fs.FS) gin.HandlerFunc {
	fileServer := http.FileServer(http.FS(dist))

	var indexBytes []byte
	if f, err := dist.Open("index.html"); err == nil {
		indexBytes, _ = io.ReadAll(f)
		_ = f.Close()
	}

	return func(c *gin.Context) {
		if len(indexBytes) == 0 {
			c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(placeholder))
			return
		}

		p := strings.TrimPrefix(c.Request.URL.Path, "/")
		if p == "" {
			c.Data(http.StatusOK, "text/html; charset=utf-8", indexBytes)
			return
		}

		if f, err := dist.Open(p); err == nil {
			_ = f.Close()
			fileServer.ServeHTTP(c.Writer, c.Request)
			return
		}

		// SPA 回退：路径不存在，直接返回 index.html（避免 FileServer 对 /index.html 做 301 → /）
		c.Data(http.StatusOK, "text/html; charset=utf-8", indexBytes)
	}
}

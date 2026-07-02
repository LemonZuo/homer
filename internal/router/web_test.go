package router

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/gin-gonic/gin"
)

func newSPARouter(dist fstest.MapFS) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.NoRoute(SPAHandler(dist))
	return r
}

func get(t *testing.T, r *gin.Engine, path string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	r.ServeHTTP(w, req)
	return w
}

func TestSPAHandlerServesIndexAndFallback(t *testing.T) {
	dist := fstest.MapFS{
		"index.html":  {Data: []byte("<html>app</html>")},
		"assets/a.js": {Data: []byte("console.log(1)")},
	}
	r := newSPARouter(dist)

	// 根路径 → index.html
	if w := get(t, r, "/"); w.Code != 200 || w.Body.String() != "<html>app</html>" {
		t.Fatalf("/ = %d %q", w.Code, w.Body.String())
	}
	// 真实文件 → 文件内容
	if w := get(t, r, "/assets/a.js"); w.Code != 200 || w.Body.String() != "console.log(1)" {
		t.Fatalf("/assets/a.js = %d %q", w.Code, w.Body.String())
	}
	// 未命中路径 → SPA 回退 index.html(前端路由)
	if w := get(t, r, "/ups/history"); w.Code != 200 || w.Body.String() != "<html>app</html>" {
		t.Fatalf("spa fallback = %d %q", w.Code, w.Body.String())
	}
	// /index.html 命中真实文件走 FileServer,标准行为是 301 → ./,浏览器跟随后仍到 /
	if w := get(t, r, "/index.html"); w.Code != 301 {
		t.Fatalf("/index.html = %d", w.Code)
	}
}

func TestSPAHandlerPlaceholderWhenNotBuilt(t *testing.T) {
	r := newSPARouter(fstest.MapFS{})
	w := get(t, r, "/")
	if w.Code != 200 || !strings.Contains(w.Body.String(), "前端尚未构建") {
		t.Fatalf("placeholder = %d %q", w.Code, w.Body.String())
	}
}

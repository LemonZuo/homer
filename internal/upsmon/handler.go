package upsmon

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/LemonZuo/homer/internal/model"
	"github.com/LemonZuo/homer/internal/upsmon/sshhost"
	"github.com/gin-gonic/gin"
)

// Handler 暴露 UPS 监控的 HTTP 接口。
type Handler struct {
	svc     *Service
	sampler *Sampler
	hosts   *HostStore
	creds   *CredentialStore
}

func NewHandler(svc *Service, sampler *Sampler, hosts *HostStore, creds *CredentialStore) *Handler {
	return &Handler{svc: svc, sampler: sampler, hosts: hosts, creds: creds}
}

func (h *Handler) Register(rg *gin.RouterGroup) {
	g := rg.Group("/ups")
	g.GET("/snapshot", h.snapshot)
	g.GET("/stream", h.stream)
	g.GET("/series", h.series)
	g.POST("/refresh", h.refresh)

	g.GET("/hosts", h.listHosts)
	g.POST("/hosts", h.upsertHost)
	g.PUT("/hosts/:id", h.upsertHost)
	g.DELETE("/hosts/:id", h.deleteHost)
	g.POST("/hosts/:id/toggle", h.toggleHost)
	g.POST("/hosts/:id/test", h.testHost)

	g.GET("/credentials", h.listCredentials)
	g.POST("/credentials", h.upsertCredential)
	g.PUT("/credentials/:id", h.upsertCredential)
	g.DELETE("/credentials/:id", h.deleteCredential)
}

// stream SSE 推送 snapshot,替代前端 30 秒一次的 HTTP 轮询。
// 协议:订阅时立即发一帧当前 snapshot(避免空白等下一轮),之后每轮采样完推一帧。
// 25 秒一次的注释行心跳(`: ping`)防止反代/浏览器把 idle 连接掐掉。
func (h *Handler) stream(c *gin.Context) {
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "ResponseWriter 不支持流式输出"})
		return
	}

	send := func(snap []Snapshot) bool {
		buf, err := json.Marshal(snap)
		if err != nil {
			return false
		}
		if _, err := c.Writer.WriteString("event: snapshot\ndata: "); err != nil {
			return false
		}
		if _, err := c.Writer.Write(buf); err != nil {
			return false
		}
		if _, err := c.Writer.WriteString("\n\n"); err != nil {
			return false
		}
		flusher.Flush()
		return true
	}

	ch, unsub := h.svc.Subscribe()
	defer unsub()

	if snap, err := h.svc.BuildSnapshot(); err == nil {
		if !send(snap) {
			return
		}
	}

	ping := time.NewTicker(25 * time.Second)
	defer ping.Stop()
	ctxDone := c.Request.Context().Done()
	for {
		select {
		case snap, ok := <-ch:
			if !ok {
				return
			}
			if !send(snap) {
				return
			}
		case <-ping.C:
			if _, err := c.Writer.WriteString(": ping\n\n"); err != nil {
				return
			}
			flusher.Flush()
		case <-ctxDone:
			return
		}
	}
}

// snapshot 返回每台已订阅机器的最新 UPS 状态(不触发采样)。
func (h *Handler) snapshot(c *gin.Context) {
	data, err := h.svc.BuildSnapshot()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": data})
}

// series 返回指定 UPS 的历史曲线(已聚合)。
// 必填:host_kind / host_id / ups_name,可选 range=24h|7d(默认 24h)。
func (h *Handler) series(c *gin.Context) {
	hostKind := strings.TrimSpace(c.Query("host_kind"))
	upsName := strings.TrimSpace(c.Query("ups_name"))
	hostIDStr := strings.TrimSpace(c.Query("host_id"))
	if hostKind == "" || upsName == "" || hostIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "host_kind / host_id / ups_name 必填"})
		return
	}
	hostID, err := strconv.ParseInt(hostIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "host_id 无效"})
		return
	}
	window := parseRange(c.Query("range"))
	pts, err := h.svc.Series(hostKind, hostID, upsName, window)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"data":   pts,
		"range":  window.String(),
		"bucket": pickBucket(window),
	})
}

// refresh 手动触发一次采样(同步等待),供前端"立即刷新"按钮用。
func (h *Handler) refresh(c *gin.Context) {
	if err := h.svc.TriggerSample(); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	data, err := h.svc.BuildSnapshot()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": data})
}

// hostDTO 列表返回形态:把 auth_json/config_json 展平,前端直接展示。
type hostDTO struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	Endpoint      string `json:"endpoint"`
	AuthSource    string `json:"auth_source"`
	CredentialID  int64  `json:"credential_id"`
	Username      string `json:"username"`
	AuthType      string `json:"auth_type"`
	BastionHostID int64  `json:"bastion_host_id"`
	Enabled       bool   `json:"enabled"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
}

func toHostDTO(row model.UPSHost) hostDTO {
	t, _ := sshhost.ParseTarget(row)
	dto := hostDTO{
		ID:        row.ID,
		Name:      row.Name,
		Endpoint:  row.Endpoint,
		Enabled:   bool(row.Enabled),
		CreatedAt: row.CreatedAt.Format(time.RFC3339),
		UpdatedAt: row.UpdatedAt.Format(time.RFC3339),
	}
	if t != nil {
		dto.AuthSource = t.AuthSource
		dto.CredentialID = t.CredentialID
		dto.Username = t.Username
		dto.AuthType = t.AuthType
		dto.BastionHostID = t.BastionHostID
	}
	return dto
}

func (h *Handler) listHosts(c *gin.Context) {
	rows, err := h.hosts.List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	out := make([]hostDTO, 0, len(rows))
	for _, r := range rows {
		out = append(out, toHostDTO(r))
	}
	c.JSON(http.StatusOK, gin.H{"data": out})
}

func (h *Handler) upsertHost(c *gin.Context) {
	var in HostInput
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求体无效"})
		return
	}
	if idStr := c.Param("id"); idStr != "" {
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "id 无效"})
			return
		}
		in.ID = id
	}
	row, err := h.hosts.Upsert(in)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": toHostDTO(*row)})
}

func (h *Handler) deleteHost(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id 无效"})
		return
	}
	if err := h.hosts.Delete(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"id": id}})
}

func (h *Handler) toggleHost(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id 无效"})
		return
	}
	var body struct {
		Enable bool `json:"enable"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求体无效"})
		return
	}
	if err := h.hosts.SetEnabled(id, body.Enable); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"id": id, "enabled": body.Enable}})
}

// testHost 立即拨号一次,跑 upsc -l 探测,前端用于"测试连通性"按钮。
func (h *Handler) testHost(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id 无效"})
		return
	}
	res, err := h.sampler.ProbeByHostID(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	// HostResult.UPSes 标了 json:"-",直接返回会丢失名字,这里平铺出来给前端用。
	names := make([]string, 0, len(res.UPSes))
	for _, u := range res.UPSes {
		names = append(names, u.Name)
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{
		"ok":        res.OK,
		"error":     res.Error,
		"has_ups":   res.HasUPS,
		"ups_names": names,
		"diag":      res.Diag,
	}})
}

// credentialDTO 列表返回形态:不回吐密码 / 私钥明文,只暴露元数据。
type credentialDTO struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Username  string `json:"username"`
	AuthType  string `json:"auth_type"`
	RefCount  int64  `json:"ref_count"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

func toCredentialDTO(c model.UPSSSHCredential) credentialDTO {
	return credentialDTO{
		ID:        c.ID,
		Name:      c.Name,
		Username:  c.Username,
		AuthType:  c.AuthType,
		RefCount:  c.RefCount,
		CreatedAt: c.CreatedAt.Format(time.RFC3339),
		UpdatedAt: c.UpdatedAt.Format(time.RFC3339),
	}
}

func (h *Handler) listCredentials(c *gin.Context) {
	rows, err := h.creds.List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	out := make([]credentialDTO, 0, len(rows))
	for _, r := range rows {
		out = append(out, toCredentialDTO(r))
	}
	c.JSON(http.StatusOK, gin.H{"data": out})
}

func (h *Handler) upsertCredential(c *gin.Context) {
	var in model.UPSSSHCredential
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求体无效"})
		return
	}
	if idStr := c.Param("id"); idStr != "" {
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "id 无效"})
			return
		}
		in.ID = id
	}
	row, err := h.creds.Upsert(&in)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": toCredentialDTO(*row)})
}

func (h *Handler) deleteCredential(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id 无效"})
		return
	}
	if err := h.creds.Delete(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"id": id}})
}

// parseRange 把人类时间窗映射到 time.Duration。未知值回落 24h。
func parseRange(s string) time.Duration {
	switch strings.TrimSpace(s) {
	case "1h":
		return 1 * time.Hour
	case "6h":
		return 6 * time.Hour
	case "24h", "":
		return 24 * time.Hour
	case "3d":
		return 3 * 24 * time.Hour
	case "7d":
		return 7 * 24 * time.Hour
	default:
		return 24 * time.Hour
	}
}

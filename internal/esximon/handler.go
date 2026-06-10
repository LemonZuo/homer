package esximon

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/LemonZuo/homer/internal/esximon/sshhost"
	"github.com/LemonZuo/homer/internal/model"
	"github.com/gin-gonic/gin"
)

// Handler 暴露 ESXi 监控的 HTTP 接口。
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
	g := rg.Group("/esxi")
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

// stream SSE 推送 snapshot,协议同 UPS 模块。
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

func (h *Handler) snapshot(c *gin.Context) {
	data, err := h.svc.BuildSnapshot()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": data})
}

// series 历史曲线接口。必填:host_kind / host_id,可选 range=24h|7d。
func (h *Handler) series(c *gin.Context) {
	hostKind := strings.TrimSpace(c.Query("host_kind"))
	hostIDStr := strings.TrimSpace(c.Query("host_id"))
	if hostKind == "" || hostIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "host_kind / host_id 必填"})
		return
	}
	hostID, err := strconv.ParseInt(hostIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "host_id 无效"})
		return
	}
	window := parseRange(c.Query("range"))
	pts, err := h.svc.Series(hostKind, hostID, window)
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

// --- Host CRUD ---

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

func toHostDTO(row model.EsxiHost) hostDTO {
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

// testHost 立即拨号一次,跑全套采集命令,前端用于"测试连通性"按钮。
// 返回采到的平台信息 + 简短摘要,方便用户判断"对方真的是 ESXi 主机吗"。
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
	summary := buildTestSummary(res.Metrics)
	c.JSON(http.StatusOK, gin.H{"data": gin.H{
		"ok":       res.OK,
		"error":    res.Error,
		"platform": res.Metrics.Platform,
		"cpu_max":  res.Metrics.CPUTemp.MaxC,
		"vm_total": len(res.Metrics.VMs),
		"summary":  summary,
	}})
}

// buildTestSummary 把 metrics 关键信息凝成一句话给前端 toast。
func buildTestSummary(m HostMetrics) string {
	parts := []string{}
	if m.Platform.Product != "" {
		parts = append(parts, m.Platform.Vendor+" "+m.Platform.Product)
	}
	if m.Platform.ESXiVersion != "" {
		parts = append(parts, "ESXi "+m.Platform.ESXiVersion)
	}
	if m.CPUTemp.MaxC > 0 {
		parts = append(parts, "CPU 最高 "+strconv.Itoa(m.CPUTemp.MaxC)+"°C")
	}
	if len(m.VMs) > 0 {
		on := 0
		for _, v := range m.VMs {
			if v.State == "powered_on" {
				on++
			}
		}
		parts = append(parts, "VM "+strconv.Itoa(on)+"/"+strconv.Itoa(len(m.VMs)))
	}
	return strings.Join(parts, " · ")
}

// --- Credential CRUD ---

type credentialDTO struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Username  string `json:"username"`
	AuthType  string `json:"auth_type"`
	RefCount  int64  `json:"ref_count"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

func toCredentialDTO(c model.EsxiSSHCredential) credentialDTO {
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
	var in model.EsxiSSHCredential
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

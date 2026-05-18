package acme

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	acmesvc "github.com/LemonZuo/homer/internal/acme"
	acmealicas "github.com/LemonZuo/homer/internal/acme/deployer/alicas"
	acmefnos "github.com/LemonZuo/homer/internal/acme/deployer/fnos"
	acmesafeline "github.com/LemonZuo/homer/internal/acme/deployer/safeline"
	acmessh "github.com/LemonZuo/homer/internal/acme/deployer/ssh"
	acmeproviders "github.com/LemonZuo/homer/internal/acme/providers"
	"github.com/LemonZuo/homer/internal/model"

	"github.com/gin-gonic/gin"
)

// Handler ACME 自动签发接口的 HTTP handler。
type Handler struct {
	svc             *acmesvc.Service
	sshTargets      *acmessh.TargetStore
	sshDeploys      *acmessh.DeployConfigStore
	safelineTargets *acmesafeline.TargetStore
	safelineDeploys *acmesafeline.DeployConfigStore
	casTargets      *acmealicas.TargetStore
	casDeploys      *acmealicas.DeployConfigStore
	fnosTargets     *acmefnos.TargetStore
	fnosDeploys     *acmefnos.DeployConfigStore
}

func New(svc *acmesvc.Service) *Handler {
	return &Handler{
		svc:             svc,
		sshTargets:      acmessh.NewTargetStore(svc.DeployTargets()),
		sshDeploys:      acmessh.NewDeployConfigStore(svc.DeployConfigs()),
		safelineTargets: acmesafeline.NewTargetStore(svc.DeployTargets()),
		safelineDeploys: acmesafeline.NewDeployConfigStore(svc.DeployConfigs()),
		casTargets:      acmealicas.NewTargetStore(svc.DeployTargets()),
		casDeploys:      acmealicas.NewDeployConfigStore(svc.DeployConfigs()),
		fnosTargets:     acmefnos.NewTargetStore(svc.DeployTargets()),
		fnosDeploys:     acmefnos.NewDeployConfigStore(svc.DeployConfigs()),
	}
}

type deployTargetPayload struct {
	Name       string         `json:"name"`
	Kind       string         `json:"kind"`
	Endpoint   string         `json:"endpoint"`
	AuthJSON   string         `json:"auth_json"`
	ConfigJSON string         `json:"config_json"`
	Enabled    model.BoolFlag `json:"enabled"`
}

func (p deployTargetPayload) toModel(id int64) model.ACMEDeployTarget {
	return model.ACMEDeployTarget{
		ID:         id,
		Name:       p.Name,
		Kind:       p.Kind,
		Endpoint:   p.Endpoint,
		AuthJSON:   p.AuthJSON,
		ConfigJSON: p.ConfigJSON,
		Enabled:    p.Enabled,
	}
}

type deployConfigPayload struct {
	DomainID   int64          `json:"domain_id"`
	TargetID   int64          `json:"target_id"`
	Kind       string         `json:"kind"`
	Name       string         `json:"name"`
	ConfigJSON string         `json:"config_json"`
	StateJSON  string         `json:"state_json"`
	AutoDeploy model.BoolFlag `json:"auto_deploy"`
	Enabled    model.BoolFlag `json:"enabled"`
}

func (p deployConfigPayload) toModel(id int64) model.ACMEDeployConfig {
	return model.ACMEDeployConfig{
		ID:         id,
		DomainID:   p.DomainID,
		TargetID:   p.TargetID,
		Kind:       p.Kind,
		Name:       p.Name,
		ConfigJSON: p.ConfigJSON,
		StateJSON:  p.StateJSON,
		AutoDeploy: p.AutoDeploy,
		Enabled:    p.Enabled,
	}
}

func (h *Handler) Register(rg *gin.RouterGroup) {
	g := rg.Group("/acme")
	g.GET("/providers", h.providers)
	g.GET("/accounts", h.listAccounts)
	g.POST("/accounts", h.upsertAccount)
	g.PUT("/accounts/:id", h.updateAccount)
	g.DELETE("/accounts/:id", h.deleteAccount)
	g.GET("/ssh-targets", h.listSSHTargets)
	g.POST("/ssh-targets", h.upsertSSHTarget)
	g.PUT("/ssh-targets/:id", h.updateSSHTarget)
	g.DELETE("/ssh-targets/:id", h.deleteSSHTarget)
	g.POST("/ssh-targets/:id/test", h.testSSHTarget)
	g.GET("/safeline-targets", h.listSafelineTargets)
	g.POST("/safeline-targets", h.upsertSafelineTarget)
	g.PUT("/safeline-targets/:id", h.updateSafelineTarget)
	g.DELETE("/safeline-targets/:id", h.deleteSafelineTarget)
	g.POST("/safeline-targets/:id/test", h.testSafelineTarget)
	g.GET("/cas-targets", h.listCASTargets)
	g.POST("/cas-targets", h.upsertCASTarget)
	g.PUT("/cas-targets/:id", h.updateCASTarget)
	g.DELETE("/cas-targets/:id", h.deleteCASTarget)
	g.POST("/cas-targets/:id/test", h.testCASTarget)
	g.GET("/fnos-targets", h.listFnOSTargets)
	g.POST("/fnos-targets", h.upsertFnOSTarget)
	g.PUT("/fnos-targets/:id", h.updateFnOSTarget)
	g.DELETE("/fnos-targets/:id", h.deleteFnOSTarget)
	g.POST("/fnos-targets/:id/test", h.testFnOSTarget)
	g.GET("/deploy/targets", h.listDeployTargets)
	g.POST("/deploy/targets", h.upsertDeployTarget)
	g.PUT("/deploy/targets/:id", h.updateDeployTarget)
	g.DELETE("/deploy/targets/:id", h.deleteDeployTarget)
	g.POST("/deploy/targets/:id/test", h.testDeployTarget)
	g.PUT("/deploy/configs/:id", h.updateDeployConfig)
	g.DELETE("/deploy/configs/:id", h.deleteDeployConfig)
	g.POST("/deploy/configs/:id/deploy", h.deployConfig)
	g.PUT("/ssh-deploy-configs/:id", h.updateSSHDeployConfig)
	g.DELETE("/ssh-deploy-configs/:id", h.deleteSSHDeployConfig)
	g.POST("/ssh-deploy-configs/:id/deploy", h.deploySSHConfig)
	g.PUT("/safeline-deploy-configs/:id", h.updateSafelineDeployConfig)
	g.DELETE("/safeline-deploy-configs/:id", h.deleteSafelineDeployConfig)
	g.POST("/safeline-deploy-configs/:id/deploy", h.deploySafelineConfig)
	g.PUT("/cas-deploy-configs/:id", h.updateCASDeployConfig)
	g.DELETE("/cas-deploy-configs/:id", h.deleteCASDeployConfig)
	g.POST("/cas-deploy-configs/:id/deploy", h.deployCASConfig)
	g.PUT("/fnos-deploy-configs/:id", h.updateFnOSDeployConfig)
	g.DELETE("/fnos-deploy-configs/:id", h.deleteFnOSDeployConfig)
	g.POST("/fnos-deploy-configs/:id/deploy", h.deployFnOSConfig)
	g.GET("/credentials", h.listCredentials)
	g.POST("/credentials", h.upsertCredential)
	g.DELETE("/credentials/:id", h.deleteCredential)
	g.GET("/ssh-credentials", h.listSSHCredentials)
	g.POST("/ssh-credentials", h.upsertSSHCredential)
	g.PUT("/ssh-credentials/:id", h.updateSSHCredential)
	g.DELETE("/ssh-credentials/:id", h.deleteSSHCredential)
	g.GET("/domains", h.listDomains)
	g.POST("/domains", h.createDomain)
	g.PUT("/domains/:id", h.updateDomain)
	g.DELETE("/domains/:id", h.deleteDomain)
	g.GET("/domains/:id/cert", h.domainCert)
	g.GET("/domains/:id/cert/download", h.downloadCert)
	g.POST("/domains/:id/issue", h.issue)
	g.POST("/domains/:id/revoke", h.revoke)
	g.POST("/domains/:id/deploy-ssh", h.deploySSH)
	g.GET("/domains/:id/ssh-deploy-configs", h.listSSHDeployConfigs)
	g.POST("/domains/:id/ssh-deploy-configs", h.upsertSSHDeployConfig)
	g.POST("/domains/:id/ssh-deploy-configs/deploy", h.deploySSHConfigsByDomain)
	g.GET("/domains/:id/deploy-configs", h.listDeployConfigs)
	g.POST("/domains/:id/deploy-configs", h.upsertDeployConfig)
	g.POST("/domains/:id/deploy-configs/deploy", h.deployConfigsByDomain)
	g.GET("/domains/:id/safeline-deploy-configs", h.listSafelineDeployConfigs)
	g.POST("/domains/:id/safeline-deploy-configs", h.upsertSafelineDeployConfig)
	g.POST("/domains/:id/safeline-deploy-configs/deploy", h.deploySafelineConfigsByDomain)
	g.GET("/domains/:id/cas-deploy-configs", h.listCASDeployConfigs)
	g.POST("/domains/:id/cas-deploy-configs", h.upsertCASDeployConfig)
	g.POST("/domains/:id/cas-deploy-configs/deploy", h.deployCASConfigsByDomain)
	g.GET("/domains/:id/fnos-deploy-configs", h.listFnOSDeployConfigs)
	g.POST("/domains/:id/fnos-deploy-configs", h.upsertFnOSDeployConfig)
	g.POST("/domains/:id/fnos-deploy-configs/deploy", h.deployFnOSConfigsByDomain)
	g.GET("/tasks", h.listTasks)
	g.GET("/tasks/:id", h.getTask)
	g.POST("/tasks/:id/retry", h.retryTask)
	g.GET("/tasks/:id/stream", h.streamTask)
}

func (h *Handler) providers(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"data": h.svc.Credentials().Providers()})
}

func (h *Handler) listAccounts(c *gin.Context) {
	items, err := h.svc.Accounts().List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

func (h *Handler) upsertAccount(c *gin.Context) {
	var a model.ACMEAccount
	if err := c.ShouldBindJSON(&a); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	a.ID = 0
	row, err := h.svc.Accounts().Upsert(&a)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": row})
}

func (h *Handler) updateAccount(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 id"})
		return
	}
	var a model.ACMEAccount
	if err := c.ShouldBindJSON(&a); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	a.ID = id
	row, err := h.svc.Accounts().Upsert(&a)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": row})
}

func (h *Handler) deleteAccount(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 id"})
		return
	}
	if err := h.svc.Accounts().Delete(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "已删除"})
}

func (h *Handler) listCredentials(c *gin.Context) {
	items, err := h.svc.Credentials().List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

func (h *Handler) upsertCredential(c *gin.Context) {
	var body struct {
		Provider  string `json:"provider"`
		EnvsJSON  string `json:"envs_json"`
		SkipCheck bool   `json:"skip_check"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	warn := ""
	if !body.SkipCheck {
		envs := map[string]string{}
		_ = json.Unmarshal([]byte(body.EnvsJSON), &envs)
		switch err := acmeproviders.Validate(body.Provider, envs); {
		case err == nil:
			// 校验通过，继续保存
		case errors.Is(err, acmeproviders.ErrNoValidator):
			// 未注册深度校验的 provider，允许保存，但带提示给前端
			warn = err.Error()
		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	}
	row, err := h.svc.Credentials().Upsert(body.Provider, body.EnvsJSON)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	resp := gin.H{"data": row}
	if warn != "" {
		resp["warning"] = warn
	}
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) deleteCredential(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 id"})
		return
	}
	if err := h.svc.Credentials().Delete(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "已删除"})
}

func (h *Handler) listDomains(c *gin.Context) {
	items, err := h.svc.ListDomains()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

func (h *Handler) createDomain(c *gin.Context) {
	var d model.ACMEDomain
	if err := c.ShouldBindJSON(&d); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	d.ID = 0
	if err := h.svc.CreateDomain(&d); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": d})
}

func (h *Handler) updateDomain(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 id"})
		return
	}
	var d model.ACMEDomain
	if err := c.ShouldBindJSON(&d); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	d.ID = id
	if err := h.svc.UpdateDomain(&d); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": d})
}

func (h *Handler) deleteDomain(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 id"})
		return
	}
	if err := h.svc.DeleteDomain(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "已删除"})
}

func (h *Handler) domainCert(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 id"})
		return
	}
	cert, err := h.svc.GetCertByDomain(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": cert})
}

// downloadCert 把当前证书 4 个 PEM 文件打成 ZIP 流式返回。
func (h *Handler) downloadCert(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 id"})
		return
	}
	d, err := h.svc.DomainByID(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if d == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "域名不存在"})
		return
	}
	cert, err := h.svc.GetCertByDomain(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if cert == nil || strings.TrimSpace(cert.CertPEM) == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": "当前域名还没有证书"})
		return
	}
	files := []struct {
		name string
		data string
	}{
		{"cert.pem", cert.CertPEM},
		{"chain.pem", cert.ChainPEM},
		{"fullchain.pem", cert.FullchainPEM},
		{"key.pem", cert.KeyPEM},
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, f := range files {
		w, err := zw.CreateHeader(&zip.FileHeader{
			Name:     f.name,
			Method:   zip.Deflate,
			Modified: cert.IssuedAt,
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if _, err := io.WriteString(w, f.data); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}
	if err := zw.Close(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	filename := d.MainDomain + ".zip"
	c.Header("Content-Type", "application/zip")
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	c.Header("Content-Length", strconv.Itoa(buf.Len()))
	c.Data(http.StatusOK, "application/zip", buf.Bytes())
}

func (h *Handler) issue(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 id"})
		return
	}
	kind := c.Query("kind")
	if kind == "" {
		kind = "issue"
	}
	taskID, err := h.svc.IssueAsync(id, kind)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"task_id": taskID}})
}

func (h *Handler) revoke(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 id"})
		return
	}
	taskID, err := h.svc.RevokeAsync(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"task_id": taskID}})
}

func (h *Handler) listTasks(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))
	status := c.Query("status")
	switch status {
	case "", "pending", "running", "success", "failed", "retrying":
	default:
		status = ""
	}
	items, total, err := h.svc.ListTasks(page, pageSize, status)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	c.JSON(http.StatusOK, gin.H{"data": items, "total": total, "page": page, "page_size": pageSize})
}

func (h *Handler) getTask(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 id"})
		return
	}
	t, err := h.svc.GetTask(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if t == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "任务不存在"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": t})
}

func (h *Handler) retryTask(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 id"})
		return
	}
	if err := h.svc.RetryDeployTaskNow(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"task_id": id}})
}

// streamTask SSE 推送任务日志。若任务已结束（FinishedAt 非空），
// 直接一次性发完 log_text 并关闭；运行中则订阅 hub。
func (h *Handler) streamTask(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 id"})
		return
	}
	t, err := h.svc.GetTask(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if t == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "任务不存在"})
		return
	}

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "ResponseWriter 不支持流式输出"})
		return
	}

	// 已结束：直接吐全文 + done
	if t.FinishedAt != nil {
		writeSSE(c.Writer, "log", t.LogText)
		writeSSE(c.Writer, "done", t.Status)
		flusher.Flush()
		return
	}

	ch, unsub := h.svc.Hub().Subscribe(id)
	defer unsub()

	// 先把已有的 log_text 当作回放发出
	if t.LogText != "" {
		writeSSE(c.Writer, "log", t.LogText)
		flusher.Flush()
	}

	notify := c.Request.Context().Done()
	for {
		select {
		case line, ok := <-ch:
			if !ok {
				// 任务结束：再查一次状态
				if final, err := h.svc.GetTask(id); err == nil && final != nil {
					writeSSE(c.Writer, "done", final.Status)
					flusher.Flush()
				}
				return
			}
			writeSSE(c.Writer, "log", line)
			flusher.Flush()
		case <-notify:
			return
		}
	}
}

// writeSSE 写一条 SSE 事件；多行 data 自动按行拆分（SSE 规范）。
func writeSSE(w io.Writer, event, data string) {
	fmt.Fprintf(w, "event: %s\n", event)
	// 按行拆分，每行一个 data:
	start := 0
	for i := 0; i < len(data); i++ {
		if data[i] == '\n' {
			fmt.Fprintf(w, "data: %s\n", data[start:i])
			start = i + 1
		}
	}
	if start < len(data) {
		fmt.Fprintf(w, "data: %s\n", data[start:])
	}
	if start == len(data) && len(data) == 0 {
		fmt.Fprint(w, "data: \n")
	}
	fmt.Fprint(w, "\n")
}

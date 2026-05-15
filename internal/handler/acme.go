package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/LemonZuo/homer/internal/acme"
	"github.com/LemonZuo/homer/internal/model"

	"github.com/gin-gonic/gin"
)

// ACMEHandler ACME 自动签发接口。
type ACMEHandler struct {
	svc *acme.Service
}

func NewACMEHandler(svc *acme.Service) *ACMEHandler {
	return &ACMEHandler{svc: svc}
}

func (h *ACMEHandler) Register(rg *gin.RouterGroup) {
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
	g.PUT("/ssh-deploy-configs/:id", h.updateSSHDeployConfig)
	g.DELETE("/ssh-deploy-configs/:id", h.deleteSSHDeployConfig)
	g.POST("/ssh-deploy-configs/:id/deploy", h.deploySSHConfig)
	g.GET("/credentials", h.listCredentials)
	g.POST("/credentials", h.upsertCredential)
	g.DELETE("/credentials/:id", h.deleteCredential)
	g.GET("/domains", h.listDomains)
	g.POST("/domains", h.createDomain)
	g.PUT("/domains/:id", h.updateDomain)
	g.DELETE("/domains/:id", h.deleteDomain)
	g.GET("/domains/:id/cert", h.domainCert)
	g.POST("/domains/:id/issue", h.issue)
	g.POST("/domains/:id/revoke", h.revoke)
	g.POST("/domains/:id/upload-cas", h.uploadCAS)
	g.POST("/domains/:id/deploy-ssh", h.deploySSH)
	g.GET("/domains/:id/ssh-deploy-configs", h.listSSHDeployConfigs)
	g.POST("/domains/:id/ssh-deploy-configs", h.upsertSSHDeployConfig)
	g.POST("/domains/:id/ssh-deploy-configs/deploy", h.deploySSHConfigsByDomain)
	g.GET("/tasks", h.listTasks)
	g.GET("/tasks/:id", h.getTask)
	g.GET("/tasks/:id/stream", h.streamTask)
}

func (h *ACMEHandler) providers(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"data": h.svc.Credentials().Providers()})
}

func (h *ACMEHandler) listAccounts(c *gin.Context) {
	items, err := h.svc.Accounts().List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

func (h *ACMEHandler) upsertAccount(c *gin.Context) {
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

func (h *ACMEHandler) updateAccount(c *gin.Context) {
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

func (h *ACMEHandler) deleteAccount(c *gin.Context) {
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

func (h *ACMEHandler) listSSHTargets(c *gin.Context) {
	items, err := h.svc.SSHTargets().List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

func (h *ACMEHandler) upsertSSHTarget(c *gin.Context) {
	var t model.ACMESSHTarget
	if err := c.ShouldBindJSON(&t); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	t.ID = 0
	row, err := h.svc.SSHTargets().Upsert(&t)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": row})
}

func (h *ACMEHandler) updateSSHTarget(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 id"})
		return
	}
	var t model.ACMESSHTarget
	if err := c.ShouldBindJSON(&t); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	t.ID = id
	row, err := h.svc.SSHTargets().Upsert(&t)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": row})
}

func (h *ACMEHandler) deleteSSHTarget(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 id"})
		return
	}
	if err := h.svc.SSHTargets().Delete(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "已删除"})
}

func (h *ACMEHandler) listSSHDeployConfigs(c *gin.Context) {
	domainID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || domainID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 id"})
		return
	}
	items, err := h.svc.SSHDeploys().ListByDomain(domainID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

func (h *ACMEHandler) upsertSSHDeployConfig(c *gin.Context) {
	domainID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || domainID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 id"})
		return
	}
	var cfg model.ACMESSHDeployConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	cfg.ID = 0
	row, err := h.svc.SSHDeploys().Upsert(domainID, &cfg)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": row})
}

func (h *ACMEHandler) updateSSHDeployConfig(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 id"})
		return
	}
	var cfg model.ACMESSHDeployConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	if cfg.DomainID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "domain_id 无效"})
		return
	}
	cfg.ID = id
	row, err := h.svc.SSHDeploys().Upsert(cfg.DomainID, &cfg)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": row})
}

func (h *ACMEHandler) deleteSSHDeployConfig(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 id"})
		return
	}
	if err := h.svc.SSHDeploys().Delete(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "已删除"})
}

func (h *ACMEHandler) listCredentials(c *gin.Context) {
	items, err := h.svc.Credentials().List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

func (h *ACMEHandler) upsertCredential(c *gin.Context) {
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
		switch err := acme.Validate(body.Provider, envs); {
		case err == nil:
			// 校验通过，继续保存
		case errors.Is(err, acme.ErrNoValidator):
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

func (h *ACMEHandler) deleteCredential(c *gin.Context) {
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

func (h *ACMEHandler) listDomains(c *gin.Context) {
	items, err := h.svc.ListDomains()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

func (h *ACMEHandler) createDomain(c *gin.Context) {
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

func (h *ACMEHandler) updateDomain(c *gin.Context) {
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

func (h *ACMEHandler) deleteDomain(c *gin.Context) {
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

func (h *ACMEHandler) domainCert(c *gin.Context) {
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

func (h *ACMEHandler) issue(c *gin.Context) {
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

func (h *ACMEHandler) revoke(c *gin.Context) {
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

func (h *ACMEHandler) uploadCAS(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 id"})
		return
	}
	taskID, err := h.svc.UploadCASTaskAsync(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"task_id": taskID}})
}

func (h *ACMEHandler) deploySSH(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 id"})
		return
	}
	var body struct {
		TargetID      int64  `json:"target_id"`
		CertPath      string `json:"cert_path"`
		KeyPath       string `json:"key_path"`
		ChainPath     string `json:"chain_path"`
		FullchainPath string `json:"fullchain_path"`
		DeployCommand string `json:"deploy_command"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	taskID, err := h.svc.DeploySSHTaskAsync(id, body.TargetID, acme.SSHDeployOptions{
		CertPath:      body.CertPath,
		KeyPath:       body.KeyPath,
		ChainPath:     body.ChainPath,
		FullchainPath: body.FullchainPath,
		DeployCommand: body.DeployCommand,
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"task_id": taskID}})
}

func (h *ACMEHandler) deploySSHConfig(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 id"})
		return
	}
	taskID, err := h.svc.DeploySSHConfigTaskAsync(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"task_id": taskID}})
}

func (h *ACMEHandler) deploySSHConfigsByDomain(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 id"})
		return
	}
	taskIDs, err := h.svc.DeploySSHConfigsByDomainAsync(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"task_ids": taskIDs}})
}

func (h *ACMEHandler) listTasks(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	items, err := h.svc.ListTasks(limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

func (h *ACMEHandler) getTask(c *gin.Context) {
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

// streamTask SSE 推送任务日志。若任务已结束（FinishedAt 非空），
// 直接一次性发完 log_text 并关闭；运行中则订阅 hub。
func (h *ACMEHandler) streamTask(c *gin.Context) {
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

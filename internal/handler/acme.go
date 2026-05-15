package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/LemonZuo/homer/internal/acme"
	acmesafeline "github.com/LemonZuo/homer/internal/acme/deployer/safeline"
	acmessh "github.com/LemonZuo/homer/internal/acme/deployer/ssh"
	acmeproviders "github.com/LemonZuo/homer/internal/acme/providers"
	"github.com/LemonZuo/homer/internal/model"

	"github.com/gin-gonic/gin"
)

// ACMEHandler ACME 自动签发接口。
type ACMEHandler struct {
	svc             *acme.Service
	sshTargets      *acmessh.TargetStore
	sshDeploys      *acmessh.DeployConfigStore
	safelineTargets *acmesafeline.TargetStore
	safelineDeploys *acmesafeline.DeployConfigStore
}

func NewACMEHandler(svc *acme.Service) *ACMEHandler {
	return &ACMEHandler{
		svc:             svc,
		sshTargets:      acmessh.NewTargetStore(svc.DeployTargets()),
		sshDeploys:      acmessh.NewDeployConfigStore(svc.DeployConfigs()),
		safelineTargets: acmesafeline.NewTargetStore(svc.DeployTargets()),
		safelineDeploys: acmesafeline.NewDeployConfigStore(svc.DeployConfigs()),
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
	g.GET("/safeline-targets", h.listSafelineTargets)
	g.POST("/safeline-targets", h.upsertSafelineTarget)
	g.PUT("/safeline-targets/:id", h.updateSafelineTarget)
	g.DELETE("/safeline-targets/:id", h.deleteSafelineTarget)
	g.POST("/safeline-targets/:id/test", h.testSafelineTarget)
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
	g.GET("/domains/:id/deploy-configs", h.listDeployConfigs)
	g.POST("/domains/:id/deploy-configs", h.upsertDeployConfig)
	g.POST("/domains/:id/deploy-configs/deploy", h.deployConfigsByDomain)
	g.GET("/domains/:id/safeline-deploy-configs", h.listSafelineDeployConfigs)
	g.POST("/domains/:id/safeline-deploy-configs", h.upsertSafelineDeployConfig)
	g.POST("/domains/:id/safeline-deploy-configs/deploy", h.deploySafelineConfigsByDomain)
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
	items, err := h.sshTargets.List()
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
	row, err := h.sshTargets.Upsert(&t)
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
	row, err := h.sshTargets.Upsert(&t)
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
	if err := h.sshTargets.Delete(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "已删除"})
}

func (h *ACMEHandler) listSafelineTargets(c *gin.Context) {
	items, err := h.safelineTargets.List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

func (h *ACMEHandler) upsertSafelineTarget(c *gin.Context) {
	var t model.ACMESafelineTarget
	if err := c.ShouldBindJSON(&t); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	t.ID = 0
	row, err := h.safelineTargets.Upsert(&t)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": row})
}

func (h *ACMEHandler) updateSafelineTarget(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 id"})
		return
	}
	var t model.ACMESafelineTarget
	if err := c.ShouldBindJSON(&t); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	t.ID = id
	row, err := h.safelineTargets.Upsert(&t)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": row})
}

func (h *ACMEHandler) deleteSafelineTarget(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 id"})
		return
	}
	if err := h.safelineTargets.Delete(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "已删除"})
}

func (h *ACMEHandler) testSafelineTarget(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 id"})
		return
	}
	if err := h.safelineTargets.Test(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "连接正常"})
}

func (h *ACMEHandler) listDeployTargets(c *gin.Context) {
	items, err := h.svc.DeployTargets().List(c.Query("kind"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

func (h *ACMEHandler) upsertDeployTarget(c *gin.Context) {
	var body deployTargetPayload
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	t := body.toModel(0)
	row, err := h.svc.DeployTargets().Upsert(&t)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": row})
}

func (h *ACMEHandler) updateDeployTarget(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 id"})
		return
	}
	var body deployTargetPayload
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	t := body.toModel(id)
	row, err := h.svc.DeployTargets().Upsert(&t)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": row})
}

func (h *ACMEHandler) deleteDeployTarget(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 id"})
		return
	}
	if err := h.svc.DeployTargets().Delete(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "已删除"})
}

func (h *ACMEHandler) testDeployTarget(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 id"})
		return
	}
	if err := h.svc.DeployTargets().Test(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "连接正常"})
}

func (h *ACMEHandler) listDeployConfigs(c *gin.Context) {
	domainID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || domainID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 id"})
		return
	}
	items, err := h.svc.DeployConfigs().ListByDomain(domainID, c.Query("kind"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

func (h *ACMEHandler) upsertDeployConfig(c *gin.Context) {
	domainID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || domainID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 id"})
		return
	}
	var body deployConfigPayload
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	cfg := body.toModel(0)
	row, err := h.svc.DeployConfigs().Upsert(domainID, &cfg)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": row})
}

func (h *ACMEHandler) updateDeployConfig(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 id"})
		return
	}
	var body deployConfigPayload
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	cfg := body.toModel(id)
	if cfg.DomainID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "domain_id 无效"})
		return
	}
	row, err := h.svc.DeployConfigs().Upsert(cfg.DomainID, &cfg)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": row})
}

func (h *ACMEHandler) deleteDeployConfig(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 id"})
		return
	}
	if err := h.svc.DeployConfigs().Delete(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "已删除"})
}

func (h *ACMEHandler) deployConfig(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 id"})
		return
	}
	taskID, err := h.svc.DeployConfigTaskAsync(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"task_id": taskID}})
}

func (h *ACMEHandler) deployConfigsByDomain(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 id"})
		return
	}
	taskIDs, err := h.svc.DeployConfigsByDomainAsync(id, c.Query("kind"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"task_ids": taskIDs}})
}

func (h *ACMEHandler) listSSHDeployConfigs(c *gin.Context) {
	domainID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || domainID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 id"})
		return
	}
	items, err := h.sshDeploys.ListByDomain(domainID)
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
	row, err := h.sshDeploys.Upsert(domainID, &cfg)
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
	row, err := h.sshDeploys.Upsert(cfg.DomainID, &cfg)
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
	if err := h.sshDeploys.Delete(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "已删除"})
}

func (h *ACMEHandler) listSafelineDeployConfigs(c *gin.Context) {
	domainID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || domainID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 id"})
		return
	}
	items, err := h.safelineDeploys.ListByDomain(domainID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

func (h *ACMEHandler) upsertSafelineDeployConfig(c *gin.Context) {
	domainID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || domainID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 id"})
		return
	}
	var cfg model.ACMESafelineDeployConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	cfg.ID = 0
	row, err := h.safelineDeploys.Upsert(domainID, &cfg)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": row})
}

func (h *ACMEHandler) updateSafelineDeployConfig(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 id"})
		return
	}
	var cfg model.ACMESafelineDeployConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	if cfg.DomainID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "domain_id 无效"})
		return
	}
	cfg.ID = id
	row, err := h.safelineDeploys.Upsert(cfg.DomainID, &cfg)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": row})
}

func (h *ACMEHandler) deleteSafelineDeployConfig(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 id"})
		return
	}
	if err := h.safelineDeploys.Delete(id); err != nil {
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
	configJSON := acme.MustJSON(acmessh.DeployOptions{
		CertPath:      body.CertPath,
		KeyPath:       body.KeyPath,
		ChainPath:     body.ChainPath,
		FullchainPath: body.FullchainPath,
		DeployCommand: body.DeployCommand,
	})
	taskID, err := h.svc.DeployAdHocTaskAsync(id, body.TargetID, acme.DeployKindSSH, configJSON)
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

func (h *ACMEHandler) deploySafelineConfig(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 id"})
		return
	}
	taskID, err := h.svc.DeploySafelineConfigTaskAsync(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"task_id": taskID}})
}

func (h *ACMEHandler) deploySafelineConfigsByDomain(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 id"})
		return
	}
	taskIDs, err := h.svc.DeploySafelineConfigsByDomainAsync(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"task_ids": taskIDs}})
}

func (h *ACMEHandler) listTasks(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))
	items, total, err := h.svc.ListTasks(page, pageSize)
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

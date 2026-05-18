package acme

import (
	"net/http"

	acmesvc "github.com/LemonZuo/homer/internal/acme"
	acmessh "github.com/LemonZuo/homer/internal/acme/deployer/ssh"
	"github.com/LemonZuo/homer/internal/model"

	"github.com/gin-gonic/gin"
)

func (h *Handler) listSSHTargets(c *gin.Context) {
	items, err := h.sshTargets.List()
	if serverErr(c, err) {
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

func (h *Handler) upsertSSHTarget(c *gin.Context) {
	var t model.ACMESSHTarget
	if !bindJSON(c, &t) {
		return
	}
	t.ID = 0
	row, err := h.sshTargets.Upsert(&t)
	if badReq(c, err) {
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": row})
}

func (h *Handler) updateSSHTarget(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var t model.ACMESSHTarget
	if !bindJSON(c, &t) {
		return
	}
	t.ID = id
	row, err := h.sshTargets.Upsert(&t)
	if badReq(c, err) {
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": row})
}

func (h *Handler) deleteSSHTarget(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	if err := h.sshTargets.Delete(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "已删除"})
}

func (h *Handler) testSSHTarget(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	if err := h.sshTargets.Test(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "连接正常"})
}

func (h *Handler) listSSHDeployConfigs(c *gin.Context) {
	domainID, ok := parseID(c)
	if !ok {
		return
	}
	items, err := h.sshDeploys.ListByDomain(domainID)
	if serverErr(c, err) {
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

func (h *Handler) upsertSSHDeployConfig(c *gin.Context) {
	domainID, ok := parseID(c)
	if !ok {
		return
	}
	var cfg model.ACMESSHDeployConfig
	if !bindJSON(c, &cfg) {
		return
	}
	cfg.ID = 0
	row, err := h.sshDeploys.Upsert(domainID, &cfg)
	if badReq(c, err) {
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": row})
}

func (h *Handler) updateSSHDeployConfig(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var cfg model.ACMESSHDeployConfig
	if !bindJSON(c, &cfg) {
		return
	}
	if cfg.DomainID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "domain_id 无效"})
		return
	}
	cfg.ID = id
	row, err := h.sshDeploys.Upsert(cfg.DomainID, &cfg)
	if badReq(c, err) {
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": row})
}

func (h *Handler) deleteSSHDeployConfig(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	if err := h.sshDeploys.Delete(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "已删除"})
}

func (h *Handler) listSSHCredentials(c *gin.Context) {
	items, err := h.svc.SSHCredentials().List()
	if serverErr(c, err) {
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

func (h *Handler) upsertSSHCredential(c *gin.Context) {
	var cred model.SSHCredential
	if !bindJSON(c, &cred) {
		return
	}
	cred.ID = 0
	row, err := h.svc.SSHCredentials().Upsert(&cred)
	if badReq(c, err) {
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": row})
}

func (h *Handler) updateSSHCredential(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var cred model.SSHCredential
	if !bindJSON(c, &cred) {
		return
	}
	cred.ID = id
	row, err := h.svc.SSHCredentials().Upsert(&cred)
	if badReq(c, err) {
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": row})
}

func (h *Handler) deleteSSHCredential(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	if err := h.svc.SSHCredentials().Delete(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "已删除"})
}

func (h *Handler) deploySSH(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
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
	if !bindJSON(c, &body) {
		return
	}
	configJSON := acmesvc.MustJSON(acmessh.DeployOptions{
		CertPath:      body.CertPath,
		KeyPath:       body.KeyPath,
		ChainPath:     body.ChainPath,
		FullchainPath: body.FullchainPath,
		DeployCommand: body.DeployCommand,
	})
	taskID, err := h.svc.DeployAdHocTaskAsync(id, body.TargetID, acmesvc.DeployKindSSH, configJSON)
	if badReq(c, err) {
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"task_id": taskID}})
}

func (h *Handler) deploySSHConfig(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	taskID, err := h.svc.DeploySSHConfigTaskAsync(id)
	if badReq(c, err) {
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"task_id": taskID}})
}

func (h *Handler) deploySSHConfigsByDomain(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	taskIDs, err := h.svc.DeploySSHConfigsByDomainAsync(id)
	if badReq(c, err) {
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"task_ids": taskIDs}})
}

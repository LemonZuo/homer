package acme

import (
	"net/http"

	acmesvc "github.com/LemonZuo/homer/internal/acme"
	"github.com/LemonZuo/homer/internal/model"

	"github.com/gin-gonic/gin"
)

func (h *Handler) listCASTargets(c *gin.Context) {
	items, err := h.casTargets.List()
	if serverErr(c, err) {
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

func (h *Handler) upsertCASTarget(c *gin.Context) {
	var t model.ACMEUploadCASTarget
	if !bindJSON(c, &t) {
		return
	}
	t.ID = 0
	row, err := h.casTargets.Upsert(&t)
	if badReq(c, err) {
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": row})
}

func (h *Handler) updateCASTarget(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var t model.ACMEUploadCASTarget
	if !bindJSON(c, &t) {
		return
	}
	t.ID = id
	row, err := h.casTargets.Upsert(&t)
	if badReq(c, err) {
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": row})
}

func (h *Handler) deleteCASTarget(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	if err := h.casTargets.Delete(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "已删除"})
}

func (h *Handler) testCASTarget(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	if err := h.casTargets.Test(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "连接正常"})
}

func (h *Handler) listCASDeployConfigs(c *gin.Context) {
	domainID, ok := parseID(c)
	if !ok {
		return
	}
	items, err := h.casDeploys.ListByDomain(domainID)
	if serverErr(c, err) {
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

func (h *Handler) upsertCASDeployConfig(c *gin.Context) {
	domainID, ok := parseID(c)
	if !ok {
		return
	}
	var cfg model.ACMEUploadCASDeployConfig
	if !bindJSON(c, &cfg) {
		return
	}
	cfg.ID = 0
	row, err := h.casDeploys.Upsert(domainID, &cfg)
	if badReq(c, err) {
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": row})
}

func (h *Handler) updateCASDeployConfig(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var cfg model.ACMEUploadCASDeployConfig
	if !bindJSON(c, &cfg) {
		return
	}
	if cfg.DomainID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "domain_id 无效"})
		return
	}
	cfg.ID = id
	row, err := h.casDeploys.Upsert(cfg.DomainID, &cfg)
	if badReq(c, err) {
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": row})
}

func (h *Handler) deleteCASDeployConfig(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	if err := h.casDeploys.Delete(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "已删除"})
}

func (h *Handler) deployCASConfig(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	taskID, err := h.svc.DeployConfigTaskAsync(id)
	if badReq(c, err) {
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"task_id": taskID}})
}

func (h *Handler) deployCASConfigsByDomain(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	taskIDs, err := h.svc.DeployConfigsByDomainAsync(id, acmesvc.DeployKindUploadCAS)
	if badReq(c, err) {
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"task_ids": taskIDs}})
}

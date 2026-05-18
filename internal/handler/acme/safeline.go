package acme

import (
	"net/http"

	"github.com/LemonZuo/homer/internal/model"

	"github.com/gin-gonic/gin"
)

func (h *Handler) listSafelineTargets(c *gin.Context) {
	items, err := h.safelineTargets.List()
	if serverErr(c, err) {
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

func (h *Handler) upsertSafelineTarget(c *gin.Context) {
	var t model.ACMESafelineTarget
	if !bindJSON(c, &t) {
		return
	}
	t.ID = 0
	row, err := h.safelineTargets.Upsert(&t)
	if badReq(c, err) {
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": row})
}

func (h *Handler) updateSafelineTarget(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var t model.ACMESafelineTarget
	if !bindJSON(c, &t) {
		return
	}
	t.ID = id
	row, err := h.safelineTargets.Upsert(&t)
	if badReq(c, err) {
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": row})
}

func (h *Handler) deleteSafelineTarget(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	if err := h.safelineTargets.Delete(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "已删除"})
}

func (h *Handler) testSafelineTarget(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	if err := h.safelineTargets.Test(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "连接正常"})
}

func (h *Handler) listSafelineDeployConfigs(c *gin.Context) {
	domainID, ok := parseID(c)
	if !ok {
		return
	}
	items, err := h.safelineDeploys.ListByDomain(domainID)
	if serverErr(c, err) {
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

func (h *Handler) upsertSafelineDeployConfig(c *gin.Context) {
	domainID, ok := parseID(c)
	if !ok {
		return
	}
	var cfg model.ACMESafelineDeployConfig
	if !bindJSON(c, &cfg) {
		return
	}
	cfg.ID = 0
	row, err := h.safelineDeploys.Upsert(domainID, &cfg)
	if badReq(c, err) {
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": row})
}

func (h *Handler) updateSafelineDeployConfig(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var cfg model.ACMESafelineDeployConfig
	if !bindJSON(c, &cfg) {
		return
	}
	if cfg.DomainID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "domain_id 无效"})
		return
	}
	cfg.ID = id
	row, err := h.safelineDeploys.Upsert(cfg.DomainID, &cfg)
	if badReq(c, err) {
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": row})
}

func (h *Handler) deleteSafelineDeployConfig(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	if err := h.safelineDeploys.Delete(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "已删除"})
}

func (h *Handler) deploySafelineConfig(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	taskID, err := h.svc.DeploySafelineConfigTaskAsync(id)
	if badReq(c, err) {
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"task_id": taskID}})
}

func (h *Handler) deploySafelineConfigsByDomain(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	taskIDs, err := h.svc.DeploySafelineConfigsByDomainAsync(id)
	if badReq(c, err) {
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"task_ids": taskIDs}})
}

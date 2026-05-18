package acme

import (
	"net/http"

	acmesvc "github.com/LemonZuo/homer/internal/acme"
	"github.com/LemonZuo/homer/internal/model"

	"github.com/gin-gonic/gin"
)

func (h *Handler) listFnOSTargets(c *gin.Context) {
	items, err := h.fnosTargets.List()
	if serverErr(c, err) {
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

func (h *Handler) upsertFnOSTarget(c *gin.Context) {
	var t model.ACMEFnOSTarget
	if !bindJSON(c, &t) {
		return
	}
	t.ID = 0
	row, err := h.fnosTargets.Upsert(&t)
	if badReq(c, err) {
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": row})
}

func (h *Handler) updateFnOSTarget(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var t model.ACMEFnOSTarget
	if !bindJSON(c, &t) {
		return
	}
	t.ID = id
	row, err := h.fnosTargets.Upsert(&t)
	if badReq(c, err) {
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": row})
}

func (h *Handler) deleteFnOSTarget(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	if err := h.fnosTargets.Delete(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "已删除"})
}

func (h *Handler) testFnOSTarget(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	if err := h.fnosTargets.Test(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "连接正常"})
}

func (h *Handler) listFnOSDeployConfigs(c *gin.Context) {
	domainID, ok := parseID(c)
	if !ok {
		return
	}
	items, err := h.fnosDeploys.ListByDomain(domainID)
	if serverErr(c, err) {
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

func (h *Handler) upsertFnOSDeployConfig(c *gin.Context) {
	domainID, ok := parseID(c)
	if !ok {
		return
	}
	var cfg model.ACMEFnOSDeployConfig
	if !bindJSON(c, &cfg) {
		return
	}
	cfg.ID = 0
	row, err := h.fnosDeploys.Upsert(domainID, &cfg)
	if badReq(c, err) {
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": row})
}

func (h *Handler) updateFnOSDeployConfig(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var cfg model.ACMEFnOSDeployConfig
	if !bindJSON(c, &cfg) {
		return
	}
	if cfg.DomainID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "domain_id 无效"})
		return
	}
	cfg.ID = id
	row, err := h.fnosDeploys.Upsert(cfg.DomainID, &cfg)
	if badReq(c, err) {
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": row})
}

func (h *Handler) deleteFnOSDeployConfig(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	if err := h.fnosDeploys.Delete(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "已删除"})
}

func (h *Handler) deployFnOSConfig(c *gin.Context) {
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

func (h *Handler) deployFnOSConfigsByDomain(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	taskIDs, err := h.svc.DeployConfigsByDomainAsync(id, acmesvc.DeployKindFnOS)
	if badReq(c, err) {
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"task_ids": taskIDs}})
}

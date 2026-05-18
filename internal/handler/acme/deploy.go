package acme

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func (h *Handler) listDeployTargets(c *gin.Context) {
	items, err := h.svc.DeployTargets().List(c.Query("kind"))
	if serverErr(c, err) {
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

func (h *Handler) upsertDeployTarget(c *gin.Context) {
	var body deployTargetPayload
	if !bindJSON(c, &body) {
		return
	}
	t := body.toModel(0)
	row, err := h.svc.DeployTargets().Upsert(&t)
	if badReq(c, err) {
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": row})
}

func (h *Handler) updateDeployTarget(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var body deployTargetPayload
	if !bindJSON(c, &body) {
		return
	}
	t := body.toModel(id)
	row, err := h.svc.DeployTargets().Upsert(&t)
	if badReq(c, err) {
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": row})
}

func (h *Handler) deleteDeployTarget(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	if err := h.svc.DeployTargets().Delete(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "已删除"})
}

func (h *Handler) testDeployTarget(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	if err := h.svc.DeployTargets().Test(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "连接正常"})
}

func (h *Handler) listDeployConfigs(c *gin.Context) {
	domainID, ok := parseID(c)
	if !ok {
		return
	}
	items, err := h.svc.DeployConfigs().ListByDomain(domainID, c.Query("kind"))
	if serverErr(c, err) {
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

func (h *Handler) upsertDeployConfig(c *gin.Context) {
	domainID, ok := parseID(c)
	if !ok {
		return
	}
	var body deployConfigPayload
	if !bindJSON(c, &body) {
		return
	}
	cfg := body.toModel(0)
	row, err := h.svc.DeployConfigs().Upsert(domainID, &cfg)
	if badReq(c, err) {
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": row})
}

func (h *Handler) updateDeployConfig(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var body deployConfigPayload
	if !bindJSON(c, &body) {
		return
	}
	cfg := body.toModel(id)
	if cfg.DomainID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "domain_id 无效"})
		return
	}
	row, err := h.svc.DeployConfigs().Upsert(cfg.DomainID, &cfg)
	if badReq(c, err) {
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": row})
}

func (h *Handler) deleteDeployConfig(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	if err := h.svc.DeployConfigs().Delete(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "已删除"})
}

func (h *Handler) deployConfig(c *gin.Context) {
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

func (h *Handler) deployConfigsByDomain(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	taskIDs, err := h.svc.DeployConfigsByDomainAsync(id, c.Query("kind"))
	if badReq(c, err) {
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"task_ids": taskIDs}})
}

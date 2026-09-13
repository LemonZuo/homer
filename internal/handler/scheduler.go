package handler

import (
	"errors"
	"net/http"

	"github.com/LemonZuo/homer/internal/modulecontrol"
	"github.com/LemonZuo/homer/internal/scheduler"

	"github.com/gin-gonic/gin"
)

// SchedulerHandler 暴露后台任务面板所需的只读列表与手动触发。
type SchedulerHandler struct {
	sched *scheduler.Scheduler
}

func NewSchedulerHandler(s *scheduler.Scheduler) *SchedulerHandler {
	return &SchedulerHandler{sched: s}
}

func (h *SchedulerHandler) Register(api *gin.RouterGroup) {
	api.GET("/scheduler/jobs", h.list)
	api.POST("/scheduler/jobs/:name/run", h.run)
}

func (h *SchedulerHandler) list(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"data": h.sched.Jobs()})
}

func (h *SchedulerHandler) run(c *gin.Context) {
	if err := h.sched.Trigger(c.Param("name")); err != nil {
		if errors.Is(err, modulecontrol.ErrDisabled) {
			c.JSON(http.StatusForbidden, gin.H{"code": "MODULE_DISABLED", "error": err.Error()})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

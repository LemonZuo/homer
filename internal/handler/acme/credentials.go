package acme

import (
	"encoding/json"
	"errors"
	"net/http"

	acmeproviders "github.com/LemonZuo/homer/internal/acme/providers"
	"github.com/LemonZuo/homer/internal/model"

	"github.com/gin-gonic/gin"
)

func (h *Handler) listCredentials(c *gin.Context) {
	items, err := h.svc.Credentials().List()
	if serverErr(c, err) {
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
	if !bindJSON(c, &body) {
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
	if badReq(c, err) {
		return
	}
	resp := gin.H{"data": row}
	if warn != "" {
		resp["warning"] = warn
	}
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) deleteCredential(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	if err := h.svc.Credentials().Delete(id); err != nil {
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

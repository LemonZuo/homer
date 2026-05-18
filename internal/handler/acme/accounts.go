package acme

import (
	"net/http"

	"github.com/LemonZuo/homer/internal/model"

	"github.com/gin-gonic/gin"
)

func (h *Handler) providers(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"data": h.svc.Credentials().Providers()})
}

func (h *Handler) listAccounts(c *gin.Context) {
	items, err := h.svc.Accounts().List()
	if serverErr(c, err) {
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

func (h *Handler) upsertAccount(c *gin.Context) {
	var a model.ACMEAccount
	if !bindJSON(c, &a) {
		return
	}
	a.ID = 0
	row, err := h.svc.Accounts().Upsert(&a)
	if badReq(c, err) {
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": row})
}

func (h *Handler) updateAccount(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var a model.ACMEAccount
	if !bindJSON(c, &a) {
		return
	}
	a.ID = id
	row, err := h.svc.Accounts().Upsert(&a)
	if badReq(c, err) {
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": row})
}

func (h *Handler) deleteAccount(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	if err := h.svc.Accounts().Delete(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "已删除"})
}

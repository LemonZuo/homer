package upsmon

// UPS SSH 凭证管理接口。

import (
	"strconv"
	"time"

	"github.com/LemonZuo/homer/internal/model"
	"github.com/gin-gonic/gin"
	"net/http"
)

// credentialDTO 列表返回形态:不回吐密码 / 私钥明文,只暴露元数据。
type credentialDTO struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Username  string `json:"username"`
	AuthType  string `json:"auth_type"`
	RefCount  int64  `json:"ref_count"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

func toCredentialDTO(c model.UPSSSHCredential) credentialDTO {
	return credentialDTO{
		ID:        c.ID,
		Name:      c.Name,
		Username:  c.Username,
		AuthType:  c.AuthType,
		RefCount:  c.RefCount,
		CreatedAt: c.CreatedAt.Format(time.RFC3339),
		UpdatedAt: c.UpdatedAt.Format(time.RFC3339),
	}
}

func (h *Handler) listCredentials(c *gin.Context) {
	rows, err := h.creds.List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	out := make([]credentialDTO, 0, len(rows))
	for _, r := range rows {
		out = append(out, toCredentialDTO(r))
	}
	c.JSON(http.StatusOK, gin.H{"data": out})
}

func (h *Handler) upsertCredential(c *gin.Context) {
	var in model.UPSSSHCredential
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求体无效"})
		return
	}
	if idStr := c.Param("id"); idStr != "" {
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "id 无效"})
			return
		}
		in.ID = id
	}
	row, err := h.creds.Upsert(&in)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": toCredentialDTO(*row)})
}

func (h *Handler) deleteCredential(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id 无效"})
		return
	}
	if err := h.creds.Delete(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"id": id}})
}

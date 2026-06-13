package upsmon

// UPS 主机管理接口。

import (
	"strconv"
	"time"

	"github.com/LemonZuo/homer/internal/model"
	"github.com/gin-gonic/gin"
	"net/http"
)

// hostDTO 列表返回形态:把 auth_json/config_json 展平,前端直接展示。
type hostDTO struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	Endpoint     string `json:"endpoint"`
	AuthSource   string `json:"auth_source"`
	CredentialID int64  `json:"credential_id"`
	Username     string `json:"username"`
	AuthType     string `json:"auth_type"`
	BastionID    int64  `json:"bastion_id"`
	Enabled      bool   `json:"enabled"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}

func toHostDTO(row model.UPSHost) hostDTO {
	t, _ := ParseUPSTarget(row)
	dto := hostDTO{
		ID:        row.ID,
		Name:      row.Name,
		Endpoint:  row.Endpoint,
		Enabled:   bool(row.Enabled),
		CreatedAt: row.CreatedAt.Format(time.RFC3339),
		UpdatedAt: row.UpdatedAt.Format(time.RFC3339),
	}
	if t != nil {
		dto.AuthSource = t.AuthSource
		dto.CredentialID = t.CredentialID
		dto.Username = t.Username
		dto.AuthType = t.AuthType
		dto.BastionID = t.BastionID
	}
	return dto
}

func (h *Handler) listHosts(c *gin.Context) {
	rows, err := h.hosts.List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	out := make([]hostDTO, 0, len(rows))
	for _, r := range rows {
		out = append(out, toHostDTO(r))
	}
	c.JSON(http.StatusOK, gin.H{"data": out})
}

func (h *Handler) upsertHost(c *gin.Context) {
	var in HostInput
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
	row, err := h.hosts.Upsert(in)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": toHostDTO(*row)})
}

func (h *Handler) deleteHost(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id 无效"})
		return
	}
	if err := h.hosts.Delete(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"id": id}})
}

func (h *Handler) toggleHost(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id 无效"})
		return
	}
	var body struct {
		Enable bool `json:"enable"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求体无效"})
		return
	}
	if err := h.hosts.SetEnabled(id, body.Enable); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"id": id, "enabled": body.Enable}})
}

// testHost 立即拨号一次,跑 upsc -l 探测,前端用于"测试连通性"按钮。
func (h *Handler) testHost(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id 无效"})
		return
	}
	res, err := h.sampler.ProbeByHostID(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	// HostResult.UPSes 标了 json:"-",直接返回会丢失名字,这里平铺出来给前端用。
	names := make([]string, 0, len(res.UPSes))
	for _, u := range res.UPSes {
		names = append(names, u.Name)
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{
		"ok":        res.OK,
		"error":     res.Error,
		"has_ups":   res.HasUPS,
		"ups_names": names,
		"diag":      res.Diag,
	}})
}

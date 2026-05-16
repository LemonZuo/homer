package handler

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/LemonZuo/homer/internal/model"
	"github.com/LemonZuo/homer/internal/sms"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type SMSHandler struct {
	DB   *gorm.DB
	crud *CRUD[model.SmsForwarder]
}

func NewSMSHandler(db *gorm.DB) *SMSHandler {
	return &SMSHandler{DB: db, crud: NewCRUD[model.SmsForwarder](db)}
}

func (h *SMSHandler) Register(api *gin.RouterGroup) {
	g := api.Group("/sms")
	// 转发器配置 CRUD：/api/sms/forwarders
	h.crud.Register(g, "/forwarders")
	// 操作：均需带 target_id 指定使用哪台转发器
	g.POST("/config/query", h.configQuery)
	g.POST("/send", h.send)
	g.POST("/query", h.query)
}

// resolve 根据 target_id 取出启用的转发器并构造客户端。
func (h *SMSHandler) resolve(c *gin.Context, targetID int64) (*sms.Client, bool) {
	if targetID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请选择短信转发器"})
		return nil, false
	}
	var row model.SmsForwarder
	if err := h.DB.Where("id = ?", targetID).First(&row).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "转发器不存在"})
		return nil, false
	}
	if !bool(row.Enabled) {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "该转发器已停用"})
		return nil, false
	}
	cli := sms.New(row.ServerURL, row.Secret, row.TimeoutSeconds)
	if !cli.Enabled() {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "转发器地址或密钥为空"})
		return nil, false
	}
	return cli, true
}

type configReq struct {
	TargetID int64 `json:"target_id"`
}

func (h *SMSHandler) configQuery(c *gin.Context) {
	var req configReq
	_ = c.ShouldBindJSON(&req)
	cli, ok := h.resolve(c, req.TargetID)
	if !ok {
		return
	}
	raw, err := cli.Post("/config/query", nil)
	respond(c, raw, err)
}

type sendReq struct {
	TargetID     int64  `json:"target_id"`
	SimSlot      int    `json:"sim_slot" binding:"required,min=1,max=2"`
	PhoneNumbers string `json:"phone_numbers" binding:"required"`
	MsgContent   string `json:"msg_content" binding:"required"`
}

func (h *SMSHandler) send(c *gin.Context) {
	var req sendReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	cli, ok := h.resolve(c, req.TargetID)
	if !ok {
		return
	}
	raw, err := cli.Post("/sms/send", map[string]any{
		"sim_slot":      req.SimSlot,
		"phone_numbers": strings.TrimSpace(req.PhoneNumbers),
		"msg_content":   req.MsgContent,
	})
	respond(c, raw, err)
}

type queryReq struct {
	TargetID int64  `json:"target_id"`
	Type     int    `json:"type" binding:"required,min=1,max=2"`
	PageNum  int    `json:"page_num" binding:"required,min=1"`
	PageSize int    `json:"page_size" binding:"required,min=1,max=200"`
	Keyword  string `json:"keyword"`
}

func (h *SMSHandler) query(c *gin.Context) {
	var req queryReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	cli, ok := h.resolve(c, req.TargetID)
	if !ok {
		return
	}
	raw, err := cli.Post("/sms/query", map[string]any{
		"type":      req.Type,
		"page_num":  req.PageNum,
		"page_size": req.PageSize,
		"keyword":   req.Keyword,
	})
	respond(c, raw, err)
}

// respond 把上游 JSON 透传给前端；上游返回非 JSON 时包一层 {raw}
func respond(c *gin.Context, raw []byte, err error) {
	if err != nil && len(raw) == 0 {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	var v any
	if jerr := json.Unmarshal(raw, &v); jerr != nil {
		c.JSON(http.StatusOK, gin.H{"raw": string(raw)})
		return
	}
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error(), "data": v})
		return
	}
	c.Data(http.StatusOK, "application/json; charset=utf-8", raw)
}

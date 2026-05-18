package acme

import (
	acmesvc "github.com/LemonZuo/homer/internal/acme"
	"github.com/LemonZuo/homer/internal/model"

	"github.com/gin-gonic/gin"
)

// Handler ACME 自动签发接口的 HTTP handler。
type Handler struct {
	svc *acmesvc.Service
}

func New(svc *acmesvc.Service) *Handler {
	return &Handler{svc: svc}
}

type deployTargetPayload struct {
	Name       string         `json:"name"`
	Kind       string         `json:"kind"`
	Endpoint   string         `json:"endpoint"`
	AuthJSON   string         `json:"auth_json"`
	ConfigJSON string         `json:"config_json"`
	Enabled    model.BoolFlag `json:"enabled"`
}

func (p deployTargetPayload) toModel(id int64) model.ACMEDeployTarget {
	return model.ACMEDeployTarget{
		ID:         id,
		Name:       p.Name,
		Kind:       p.Kind,
		Endpoint:   p.Endpoint,
		AuthJSON:   p.AuthJSON,
		ConfigJSON: p.ConfigJSON,
		Enabled:    p.Enabled,
	}
}

type deployConfigPayload struct {
	DomainID   int64          `json:"domain_id"`
	TargetID   int64          `json:"target_id"`
	Kind       string         `json:"kind"`
	Name       string         `json:"name"`
	ConfigJSON string         `json:"config_json"`
	StateJSON  string         `json:"state_json"`
	AutoDeploy model.BoolFlag `json:"auto_deploy"`
	Enabled    model.BoolFlag `json:"enabled"`
}

func (p deployConfigPayload) toModel(id int64) model.ACMEDeployConfig {
	return model.ACMEDeployConfig{
		ID:         id,
		DomainID:   p.DomainID,
		TargetID:   p.TargetID,
		Kind:       p.Kind,
		Name:       p.Name,
		ConfigJSON: p.ConfigJSON,
		StateJSON:  p.StateJSON,
		AutoDeploy: p.AutoDeploy,
		Enabled:    p.Enabled,
	}
}

func (h *Handler) Register(rg *gin.RouterGroup) {
	g := rg.Group("/acme")
	g.GET("/providers", h.providers)
	g.GET("/accounts", h.listAccounts)
	g.POST("/accounts", h.upsertAccount)
	g.PUT("/accounts/:id", h.updateAccount)
	g.DELETE("/accounts/:id", h.deleteAccount)
	g.GET("/deploy/targets", h.listDeployTargets)
	g.POST("/deploy/targets", h.upsertDeployTarget)
	g.PUT("/deploy/targets/:id", h.updateDeployTarget)
	g.DELETE("/deploy/targets/:id", h.deleteDeployTarget)
	g.POST("/deploy/targets/:id/test", h.testDeployTarget)
	g.PUT("/deploy/configs/:id", h.updateDeployConfig)
	g.DELETE("/deploy/configs/:id", h.deleteDeployConfig)
	g.POST("/deploy/configs/:id/deploy", h.deployConfig)
	g.GET("/credentials", h.listCredentials)
	g.POST("/credentials", h.upsertCredential)
	g.DELETE("/credentials/:id", h.deleteCredential)
	g.GET("/ssh-credentials", h.listSSHCredentials)
	g.POST("/ssh-credentials", h.upsertSSHCredential)
	g.PUT("/ssh-credentials/:id", h.updateSSHCredential)
	g.DELETE("/ssh-credentials/:id", h.deleteSSHCredential)
	g.GET("/domains", h.listDomains)
	g.POST("/domains", h.createDomain)
	g.PUT("/domains/:id", h.updateDomain)
	g.DELETE("/domains/:id", h.deleteDomain)
	g.GET("/domains/:id/cert", h.domainCert)
	g.GET("/domains/:id/cert/download", h.downloadCert)
	g.POST("/domains/:id/issue", h.issue)
	g.POST("/domains/:id/revoke", h.revoke)
	g.GET("/domains/:id/deploy-configs", h.listDeployConfigs)
	g.POST("/domains/:id/deploy-configs", h.upsertDeployConfig)
	g.POST("/domains/:id/deploy-configs/deploy", h.deployConfigsByDomain)
	g.GET("/tasks", h.listTasks)
	g.GET("/tasks/:id", h.getTask)
	g.POST("/tasks/:id/retry", h.retryTask)
	g.GET("/tasks/:id/stream", h.streamTask)
}

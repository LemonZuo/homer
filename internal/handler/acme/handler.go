package acme

import (
	acmesvc "github.com/LemonZuo/homer/internal/acme"
	acmealicas "github.com/LemonZuo/homer/internal/acme/deployer/alicas"
	acmefnos "github.com/LemonZuo/homer/internal/acme/deployer/fnos"
	acmesafeline "github.com/LemonZuo/homer/internal/acme/deployer/safeline"
	acmessh "github.com/LemonZuo/homer/internal/acme/deployer/ssh"
	"github.com/LemonZuo/homer/internal/model"

	"github.com/gin-gonic/gin"
)

// Handler ACME 自动签发接口的 HTTP handler。
type Handler struct {
	svc             *acmesvc.Service
	sshTargets      *acmessh.TargetStore
	sshDeploys      *acmessh.DeployConfigStore
	safelineTargets *acmesafeline.TargetStore
	safelineDeploys *acmesafeline.DeployConfigStore
	casTargets      *acmealicas.TargetStore
	casDeploys      *acmealicas.DeployConfigStore
	fnosTargets     *acmefnos.TargetStore
	fnosDeploys     *acmefnos.DeployConfigStore
}

func New(svc *acmesvc.Service) *Handler {
	return &Handler{
		svc:             svc,
		sshTargets:      acmessh.NewTargetStore(svc.DeployTargets()),
		sshDeploys:      acmessh.NewDeployConfigStore(svc.DeployConfigs()),
		safelineTargets: acmesafeline.NewTargetStore(svc.DeployTargets()),
		safelineDeploys: acmesafeline.NewDeployConfigStore(svc.DeployConfigs()),
		casTargets:      acmealicas.NewTargetStore(svc.DeployTargets()),
		casDeploys:      acmealicas.NewDeployConfigStore(svc.DeployConfigs()),
		fnosTargets:     acmefnos.NewTargetStore(svc.DeployTargets()),
		fnosDeploys:     acmefnos.NewDeployConfigStore(svc.DeployConfigs()),
	}
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
	g.GET("/ssh-targets", h.listSSHTargets)
	g.POST("/ssh-targets", h.upsertSSHTarget)
	g.PUT("/ssh-targets/:id", h.updateSSHTarget)
	g.DELETE("/ssh-targets/:id", h.deleteSSHTarget)
	g.POST("/ssh-targets/:id/test", h.testSSHTarget)
	g.GET("/safeline-targets", h.listSafelineTargets)
	g.POST("/safeline-targets", h.upsertSafelineTarget)
	g.PUT("/safeline-targets/:id", h.updateSafelineTarget)
	g.DELETE("/safeline-targets/:id", h.deleteSafelineTarget)
	g.POST("/safeline-targets/:id/test", h.testSafelineTarget)
	g.GET("/cas-targets", h.listCASTargets)
	g.POST("/cas-targets", h.upsertCASTarget)
	g.PUT("/cas-targets/:id", h.updateCASTarget)
	g.DELETE("/cas-targets/:id", h.deleteCASTarget)
	g.POST("/cas-targets/:id/test", h.testCASTarget)
	g.GET("/fnos-targets", h.listFnOSTargets)
	g.POST("/fnos-targets", h.upsertFnOSTarget)
	g.PUT("/fnos-targets/:id", h.updateFnOSTarget)
	g.DELETE("/fnos-targets/:id", h.deleteFnOSTarget)
	g.POST("/fnos-targets/:id/test", h.testFnOSTarget)
	g.GET("/deploy/targets", h.listDeployTargets)
	g.POST("/deploy/targets", h.upsertDeployTarget)
	g.PUT("/deploy/targets/:id", h.updateDeployTarget)
	g.DELETE("/deploy/targets/:id", h.deleteDeployTarget)
	g.POST("/deploy/targets/:id/test", h.testDeployTarget)
	g.PUT("/deploy/configs/:id", h.updateDeployConfig)
	g.DELETE("/deploy/configs/:id", h.deleteDeployConfig)
	g.POST("/deploy/configs/:id/deploy", h.deployConfig)
	g.PUT("/ssh-deploy-configs/:id", h.updateSSHDeployConfig)
	g.DELETE("/ssh-deploy-configs/:id", h.deleteSSHDeployConfig)
	g.POST("/ssh-deploy-configs/:id/deploy", h.deploySSHConfig)
	g.PUT("/safeline-deploy-configs/:id", h.updateSafelineDeployConfig)
	g.DELETE("/safeline-deploy-configs/:id", h.deleteSafelineDeployConfig)
	g.POST("/safeline-deploy-configs/:id/deploy", h.deploySafelineConfig)
	g.PUT("/cas-deploy-configs/:id", h.updateCASDeployConfig)
	g.DELETE("/cas-deploy-configs/:id", h.deleteCASDeployConfig)
	g.POST("/cas-deploy-configs/:id/deploy", h.deployCASConfig)
	g.PUT("/fnos-deploy-configs/:id", h.updateFnOSDeployConfig)
	g.DELETE("/fnos-deploy-configs/:id", h.deleteFnOSDeployConfig)
	g.POST("/fnos-deploy-configs/:id/deploy", h.deployFnOSConfig)
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
	g.POST("/domains/:id/deploy-ssh", h.deploySSH)
	g.GET("/domains/:id/ssh-deploy-configs", h.listSSHDeployConfigs)
	g.POST("/domains/:id/ssh-deploy-configs", h.upsertSSHDeployConfig)
	g.POST("/domains/:id/ssh-deploy-configs/deploy", h.deploySSHConfigsByDomain)
	g.GET("/domains/:id/deploy-configs", h.listDeployConfigs)
	g.POST("/domains/:id/deploy-configs", h.upsertDeployConfig)
	g.POST("/domains/:id/deploy-configs/deploy", h.deployConfigsByDomain)
	g.GET("/domains/:id/safeline-deploy-configs", h.listSafelineDeployConfigs)
	g.POST("/domains/:id/safeline-deploy-configs", h.upsertSafelineDeployConfig)
	g.POST("/domains/:id/safeline-deploy-configs/deploy", h.deploySafelineConfigsByDomain)
	g.GET("/domains/:id/cas-deploy-configs", h.listCASDeployConfigs)
	g.POST("/domains/:id/cas-deploy-configs", h.upsertCASDeployConfig)
	g.POST("/domains/:id/cas-deploy-configs/deploy", h.deployCASConfigsByDomain)
	g.GET("/domains/:id/fnos-deploy-configs", h.listFnOSDeployConfigs)
	g.POST("/domains/:id/fnos-deploy-configs", h.upsertFnOSDeployConfig)
	g.POST("/domains/:id/fnos-deploy-configs/deploy", h.deployFnOSConfigsByDomain)
	g.GET("/tasks", h.listTasks)
	g.GET("/tasks/:id", h.getTask)
	g.POST("/tasks/:id/retry", h.retryTask)
	g.GET("/tasks/:id/stream", h.streamTask)
}

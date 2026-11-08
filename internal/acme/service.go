package acme

import (
	"sync"
	"time"

	"github.com/LemonZuo/homer/internal/modulecontrol"
	"github.com/LemonZuo/homer/internal/notify"
	"gorm.io/gorm"
)

// Service ACME 业务编排：域名 CRUD、签发、续期、落盘、部署、SSE 日志。
type Service struct {
	modules        *modulecontrol.Manager
	db             *gorm.DB
	manager        *Manager
	credstore      *CredentialStore
	sshCredstore   *SSHCredentialStore
	accountStore   *AccountStore
	deployTargets  *DeployTargetStore
	deployConfigs  *DeployConfigStore
	deployRegistry *DeployRegistry
	hub            *SSEHub
	notifier       notify.Notifier // 签发/部署终态失败告警；nil 不告警
	dataDir        string
	renewDays      int

	deployRetry        int           // 部署任务允许总执行次数（含首次），1=不重试
	deployRetryBackoff time.Duration // 退避基数，实际间隔 = backoff * 已执行次数

	issueMu sync.Mutex // 串行化签发（lego logger / env 是全局状态）
}

func (s *Service) SetModules(m *modulecontrol.Manager) { s.modules = m }

func NewService(db *gorm.DB, mgr *Manager, store *CredentialStore, sshCreds *SSHCredentialStore, accounts *AccountStore, deployTargets *DeployTargetStore, deployConfigs *DeployConfigStore, deployRegistry *DeployRegistry, hub *SSEHub, dataDir string, renewDays int, deployRetry int, deployRetryBackoff time.Duration) *Service {
	if deployRetry < 1 {
		deployRetry = 1
	}
	return &Service{db: db, manager: mgr, credstore: store, sshCredstore: sshCreds, accountStore: accounts, deployTargets: deployTargets, deployConfigs: deployConfigs, deployRegistry: deployRegistry, hub: hub, dataDir: dataDir, renewDays: renewDays, deployRetry: deployRetry, deployRetryBackoff: deployRetryBackoff}
}

func (s *Service) Hub() *SSEHub                        { return s.hub }
func (s *Service) Credentials() *CredentialStore       { return s.credstore }
func (s *Service) SSHCredentials() *SSHCredentialStore { return s.sshCredstore }
func (s *Service) Accounts() *AccountStore             { return s.accountStore }
func (s *Service) DeployTargets() *DeployTargetStore   { return s.deployTargets }
func (s *Service) DeployConfigs() *DeployConfigStore   { return s.deployConfigs }

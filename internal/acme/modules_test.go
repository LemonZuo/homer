package acme

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/LemonZuo/homer/internal/model"
	"github.com/LemonZuo/homer/internal/modulecontrol"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// 使用专用临时数据库运行真实持久化和异步队列测试；默认不连接任何数据库。
func moduleTestService(t *testing.T) (*Service, *modulecontrol.Manager) {
	t.Helper()
	dsn := os.Getenv("HOMER_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("HOMER_TEST_MYSQL_DSN 未配置，跳过独立 MySQL 集成测试")
	}
	if !strings.Contains(dsn, "/homer_modules_test?") {
		t.Fatal("只允许专用 homer_modules_test 数据库")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := db.AutoMigrate(&model.ModuleSetting{}, &model.ACMEIssueTask{}); err != nil {
		t.Fatal(err)
	}
	m, err := modulecontrol.New(context.Background(), modulecontrol.NewDBStore(db))
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range m.Snapshot().Modules {
		if v.Key == "acme" && !v.Enabled {
			if _, err := m.Set(context.Background(), "acme", true, v.Version); err != nil {
				t.Fatal(err)
			}
		}
	}
	return &Service{db: db, modules: m, hub: NewSSEHub(), deployRetry: 3}, m
}

func toggleACME(t *testing.T, m *modulecontrol.Manager, enabled bool) {
	t.Helper()
	for _, v := range m.Snapshot().Modules {
		if v.Key == "acme" {
			if _, err := m.Set(context.Background(), "acme", enabled, v.Version); err != nil {
				t.Fatal(err)
			}
			return
		}
	}
}

func TestQueuedIssueAndDeployPauseAcrossDisableEnable(t *testing.T) {
	s, m := moduleTestService(t)
	ticket, err := m.Ticket("acme")
	if err != nil {
		t.Fatal(err)
	}
	toggleACME(t, m, false)
	toggleACME(t, m, true)
	for _, kind := range []string{"issue", "deploy_ssh", "revoke"} {
		task := model.ACMEIssueTask{MainDomain: "module-test.invalid", Kind: kind, Status: "pending", Attempt: 0}
		if err := s.db.Create(&task).Error; err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { s.db.Delete(&model.ACMEIssueTask{}, task.ID) })
		if kind == "issue" {
			s.runIssue(task.ID, kind, model.ACMEDomain{}, ticket)
		} else if kind == "revoke" {
			s.runRevoke(task.ID, model.ACMEDomain{}, model.ACMECert{}, ticket)
		} else {
			s.runDeploy(task.ID, model.ACMEDomain{}, model.ACMECert{}, model.ACMEDeployTarget{}, model.ACMEDeployConfig{}, ticket)
		}
		if err := s.db.First(&task, task.ID).Error; err != nil {
			t.Fatal(err)
		}
		if task.Status != "paused" || task.Attempt != 0 || task.FinishedAt == nil || task.LogText == "" {
			t.Fatalf("queued task executed: %+v", task)
		}
	}
}

type blockingDriver struct {
	started, release chan struct{}
	fail             bool
}

func TestNewIssueSupersedesOldPausedRenewal(t *testing.T) {
	s, _ := moduleTestService(t)
	domainID := time.Now().UnixNano()
	t.Cleanup(func() { s.db.Where("domain_id = ?", domainID).Delete(&model.ACMEIssueTask{}) })
	paused := model.ACMEIssueTask{DomainID: domainID, Kind: "renew", Status: "paused"}
	if err := s.db.Create(&paused).Error; err != nil {
		t.Fatal(err)
	}
	if blocked, err := s.hasOutstandingIssue(domainID); err != nil || !blocked {
		t.Fatalf("paused task should block: %v %v", blocked, err)
	}
	fresh := model.ACMEIssueTask{DomainID: domainID, Kind: "issue", Status: "success"}
	if err := s.db.Create(&fresh).Error; err != nil {
		t.Fatal(err)
	}
	if blocked, err := s.hasOutstandingIssue(domainID); err != nil || blocked {
		t.Fatalf("old pause blocked future renewal: %v %v", blocked, err)
	}
	fresh.ID = 0
	fresh.Status = "pending"
	if err := s.db.Create(&fresh).Error; err != nil {
		t.Fatal(err)
	}
	if blocked, err := s.hasOutstandingIssue(domainID); err != nil || !blocked {
		t.Fatalf("pending issue should block: %v %v", blocked, err)
	}
}

// paused 多半是重启 / 停用打断的：宽限期内阻止自动续期，过期后放行，避免永久跳过续期。
func TestStalePausedRenewalStopsBlocking(t *testing.T) {
	s, _ := moduleTestService(t)
	domainID := time.Now().UnixNano()
	t.Cleanup(func() { s.db.Where("domain_id = ?", domainID).Delete(&model.ACMEIssueTask{}) })
	recent := time.Now().Add(-time.Hour)
	paused := model.ACMEIssueTask{DomainID: domainID, Kind: "renew", Status: "paused", FinishedAt: &recent}
	if err := s.db.Create(&paused).Error; err != nil {
		t.Fatal(err)
	}
	if blocked, err := s.hasOutstandingIssue(domainID); err != nil || !blocked {
		t.Fatalf("recent pause should block: %v %v", blocked, err)
	}
	stale := time.Now().Add(-pausedRenewGrace - time.Hour)
	if err := s.db.Model(&paused).Update("finished_at", &stale).Error; err != nil {
		t.Fatal(err)
	}
	if blocked, err := s.hasOutstandingIssue(domainID); err != nil || blocked {
		t.Fatalf("stale pause must not block renewal: %v %v", blocked, err)
	}
}

// 部署重试用尽进入终态失败时要告警；状态落库不受影响。
func TestFinalDeployFailureAlerts(t *testing.T) {
	s, m := moduleTestService(t)
	out := newFakeAlertOut(true)
	s.notifier = out
	driver := &blockingDriver{started: make(chan struct{}), release: make(chan struct{}), fail: true}
	close(driver.release)
	s.deployRegistry = NewDeployRegistry(driver)
	task := model.ACMEIssueTask{MainDomain: "alert.invalid", Kind: "deploy_test", Status: "pending", ConfigID: 1, Attempt: 2, MaxAttempt: 3}
	if err := s.db.Create(&task).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.db.Delete(&model.ACMEIssueTask{}, task.ID) })
	ticket, _ := m.Ticket("acme")
	s.runDeploy(task.ID, model.ACMEDomain{MainDomain: "alert.invalid"}, model.ACMECert{},
		model.ACMEDeployTarget{Name: "nas"}, model.ACMEDeployConfig{ID: 1, Kind: "test", Name: "cfg"}, ticket)
	if err := s.db.First(&task, task.ID).Error; err != nil {
		t.Fatal(err)
	}
	if task.Status != "failed" || task.Attempt != 3 {
		t.Fatalf("unexpected final state: %+v", task)
	}
	msg := out.wait(t)
	if msg.Title != "证书部署失败" || !strings.Contains(msg.Text, "已尝试 3/3 次") || !strings.Contains(msg.Text, "simulated deployment failure") {
		t.Fatalf("unexpected alert: %+v", msg)
	}
}

func (*blockingDriver) Kind() string                                { return "test" }
func (*blockingDriver) Label() string                               { return "测试" }
func (*blockingDriver) ValidateTarget(model.ACMEDeployTarget) error { return nil }
func (*blockingDriver) ValidateConfig(model.ACMEDeployTarget, model.ACMEDeployConfig) error {
	return nil
}
func (*blockingDriver) TestTarget(context.Context, model.ACMEDeployTarget) error { return nil }
func (d *blockingDriver) Deploy(context.Context, DeployRequest) (*DeployResult, error) {
	close(d.started)
	<-d.release
	if d.fail {
		return nil, errors.New("simulated deployment failure")
	}
	return &DeployResult{}, nil
}

func TestRunningDeploymentDrainsAndDoesNotScheduleNewRetry(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{false: "success", true: "failure"}[fail], func(t *testing.T) {
			s, m := moduleTestService(t)
			driver := &blockingDriver{started: make(chan struct{}), release: make(chan struct{}), fail: fail}
			s.deployRegistry = NewDeployRegistry(driver)
			task := model.ACMEIssueTask{Kind: "deploy_test", Status: "pending", ConfigID: 1, MaxAttempt: 3}
			if err := s.db.Create(&task).Error; err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { s.db.Delete(&model.ACMEIssueTask{}, task.ID) })
			ticket, _ := m.Ticket("acme")
			done := make(chan struct{})
			go func() {
				defer close(done)
				s.runDeploy(task.ID, model.ACMEDomain{}, model.ACMECert{}, model.ACMEDeployTarget{}, model.ACMEDeployConfig{Kind: "test"}, ticket)
			}()
			select {
			case <-driver.started:
			case <-time.After(3 * time.Second):
				t.Fatal("deployment did not start")
			}
			toggleACME(t, m, false)
			close(driver.release)
			select {
			case <-done:
			case <-time.After(3 * time.Second):
				t.Fatal("deployment did not drain")
			}
			if err := s.db.First(&task, task.ID).Error; err != nil {
				t.Fatal(err)
			}
			want := "success"
			if fail {
				want = "paused"
			}
			if task.Status != want || task.Attempt != 1 || task.NextRetryAt != nil {
				t.Fatalf("unexpected final state: %+v", task)
			}
		})
	}
}

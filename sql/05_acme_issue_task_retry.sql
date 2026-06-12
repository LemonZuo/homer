-- acme_issue_task 增加失败重试相关列。
-- 仅持久化部署配置触发的部署任务参与重试（config_id>0），由 acme-deploy-retry
-- cron 扫描 status='retrying' 且 next_retry_at<=now 的任务择时拉起。
--
-- 用 procedure 做幂等：旧库缺列时补，AutoMigrate 已补过则跳过，全新库已是新
-- schema 也跳过。历史行 attempt=0/max_attempt=1/config_id=0，status 不会是
-- retrying，调度器扫不到 → 平滑迁移、零影响。

DROP PROCEDURE IF EXISTS homer_acme_issue_task_retry;
DELIMITER //
CREATE PROCEDURE homer_acme_issue_task_retry()
BEGIN
  IF NOT EXISTS (SELECT 1 FROM INFORMATION_SCHEMA.COLUMNS
                 WHERE TABLE_SCHEMA = DATABASE()
                   AND TABLE_NAME = 'acme_issue_task'
                   AND COLUMN_NAME = 'attempt') THEN
    ALTER TABLE `acme_issue_task` ADD COLUMN `attempt` INT NOT NULL DEFAULT 0 COMMENT '已执行次数';
  END IF;
  IF NOT EXISTS (SELECT 1 FROM INFORMATION_SCHEMA.COLUMNS
                 WHERE TABLE_SCHEMA = DATABASE()
                   AND TABLE_NAME = 'acme_issue_task'
                   AND COLUMN_NAME = 'max_attempt') THEN
    ALTER TABLE `acme_issue_task` ADD COLUMN `max_attempt` INT NOT NULL DEFAULT 1 COMMENT '允许总次数';
  END IF;
  IF NOT EXISTS (SELECT 1 FROM INFORMATION_SCHEMA.COLUMNS
                 WHERE TABLE_SCHEMA = DATABASE()
                   AND TABLE_NAME = 'acme_issue_task'
                   AND COLUMN_NAME = 'config_id') THEN
    ALTER TABLE `acme_issue_task` ADD COLUMN `config_id` BIGINT NOT NULL DEFAULT 0 COMMENT '部署配置 ID';
  END IF;
  IF NOT EXISTS (SELECT 1 FROM INFORMATION_SCHEMA.COLUMNS
                 WHERE TABLE_SCHEMA = DATABASE()
                   AND TABLE_NAME = 'acme_issue_task'
                   AND COLUMN_NAME = 'next_retry_at') THEN
    ALTER TABLE `acme_issue_task` ADD COLUMN `next_retry_at` DATETIME NULL COMMENT '下次重试时刻';
  END IF;
  IF NOT EXISTS (SELECT 1 FROM INFORMATION_SCHEMA.STATISTICS
                 WHERE TABLE_SCHEMA = DATABASE()
                   AND TABLE_NAME = 'acme_issue_task'
                   AND INDEX_NAME = 'idx_retry') THEN
    ALTER TABLE `acme_issue_task` ADD INDEX `idx_retry` (`status`, `next_retry_at`);
  END IF;
END //
DELIMITER ;
CALL homer_acme_issue_task_retry();
DROP PROCEDURE IF EXISTS homer_acme_issue_task_retry;

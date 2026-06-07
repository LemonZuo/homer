-- UPS 监控:每 30 秒采样 1 次,保留 7 天。
-- 数据源是 acme_deploy_target 里 kind IN ('ssh','fnos') 且 ups_monitor='1' 的机器,
-- 远端跑 `upsc`,解析品牌型号 / 供电类型 / 剩余电量 / 预估续航,落 ups_sample + ups_state。
-- 告警基于 ups_state 的状态机(OL→OB / OB→LB / *→OL),通过 notify.Hub 走 module='ups'。

-- 给 acme_deploy_target 加一个 ups_monitor 开关。
-- 不另开机器表(避免同一台机器在 ACME 与 UPS 两边重复维护),直接复用现有目标行。
-- 只在 kind IN ('ssh','fnos') 上有语义,其他 kind 忽略此列。
DROP PROCEDURE IF EXISTS homer_acme_add_ups_monitor;
DELIMITER //
CREATE PROCEDURE homer_acme_add_ups_monitor()
BEGIN
  IF NOT EXISTS (SELECT 1 FROM INFORMATION_SCHEMA.COLUMNS
                 WHERE TABLE_SCHEMA = DATABASE()
                   AND TABLE_NAME = 'acme_deploy_target'
                   AND COLUMN_NAME = 'ups_monitor') THEN
    ALTER TABLE `acme_deploy_target`
      ADD COLUMN `ups_monitor` VARCHAR(1) NOT NULL DEFAULT '0' AFTER `enabled`;
  END IF;
END //
DELIMITER ;
CALL homer_acme_add_ups_monitor();
DROP PROCEDURE IF EXISTS homer_acme_add_ups_monitor;

CREATE TABLE IF NOT EXISTS `ups_sample` (
  `id` BIGINT NOT NULL AUTO_INCREMENT,
  `host_kind` VARCHAR(16) NOT NULL,
  `host_id` BIGINT NOT NULL,
  `host_name` VARCHAR(64) NOT NULL DEFAULT '',
  `ups_name` VARCHAR(64) NOT NULL,
  `mfr` VARCHAR(128) NOT NULL DEFAULT '',
  `model` VARCHAR(128) NOT NULL DEFAULT '',
  `power_source` VARCHAR(16) NOT NULL DEFAULT 'unknown',
  `battery_percent` TINYINT NOT NULL DEFAULT -1,
  `runtime_minutes` INT NOT NULL DEFAULT -1,
  `raw_status` VARCHAR(64) NOT NULL DEFAULT '',
  `sampled_at` DATETIME(3) NOT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_host_ups_time` (`host_kind`, `host_id`, `ups_name`, `sampled_at` DESC),
  KEY `idx_sampled_at` (`sampled_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS `ups_state` (
  `host_kind` VARCHAR(16) NOT NULL,
  `host_id` BIGINT NOT NULL,
  `ups_name` VARCHAR(64) NOT NULL,
  `host_name` VARCHAR(64) NOT NULL DEFAULT '',
  `mfr` VARCHAR(128) NOT NULL DEFAULT '',
  `model` VARCHAR(128) NOT NULL DEFAULT '',
  `last_power_source` VARCHAR(16) NOT NULL DEFAULT 'unknown',
  `last_battery_percent` TINYINT NOT NULL DEFAULT -1,
  `last_runtime_minutes` INT NOT NULL DEFAULT -1,
  `last_raw_status` VARCHAR(64) NOT NULL DEFAULT '',
  `last_alert_at` DATETIME(3) DEFAULT NULL,
  `updated_at` DATETIME(3) NOT NULL,
  PRIMARY KEY (`host_kind`, `host_id`, `ups_name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

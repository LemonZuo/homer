-- UPS 模块从 acme_deploy_target 完全解耦：
--   1. 新建 ups_ssh_credential —— UPS 专属凭证库（不与 ACME 共享）
--   2. 新建 ups_host          —— UPS 自管的机器列表（含 endpoint + auth_json + 可选 bastion）
--   3. 清空 ups_sample / ups_state —— host_kind 语义从 'ssh'/'fnos' 切到 'ups'，历史数据按用户确认弃掉
--   4. 下线 acme_deploy_target.ups_monitor 列
--
-- 切换后：sampler 直接读 ups_host，HostKind 写常量 'ups'，HostID 写 ups_host.id。
-- bastion 走 ups_host.config_json = {"bastion_host_id": <ups_host.id>}，仍只允许单跳。

CREATE TABLE IF NOT EXISTS `ups_ssh_credential` (
  `id`          BIGINT       NOT NULL AUTO_INCREMENT,
  `name`        VARCHAR(64)  NOT NULL                COMMENT '凭证名称',
  `username`    VARCHAR(128) NOT NULL DEFAULT ''     COMMENT '用户名',
  `auth_type`   VARCHAR(16)  NOT NULL DEFAULT 'password' COMMENT '认证类型',
  `password`    TEXT         NULL                    COMMENT '密码',
  `private_key` TEXT         NULL                    COMMENT '私钥',
  `passphrase`  TEXT         NULL                    COMMENT '私钥口令',
  `created_at`  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at`  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_name` (`name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='UPS 模块专用 SSH 凭证';

CREATE TABLE IF NOT EXISTS `ups_host` (
  `id`          BIGINT       NOT NULL AUTO_INCREMENT,
  `name`        VARCHAR(64)  NOT NULL                COMMENT '机器名称',
  `endpoint`    VARCHAR(512) NOT NULL DEFAULT ''     COMMENT 'SSH 入口',
  `auth_json`   TEXT         NOT NULL                COMMENT '认证配置',
  `config_json` TEXT         NOT NULL                COMMENT '扩展配置',
  `enabled`     VARCHAR(1)   NOT NULL DEFAULT '1'    COMMENT '是否启用',
  `created_at`  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at`  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_name` (`name`),
  KEY `idx_enabled` (`enabled`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='UPS 监控目标机器';

-- host_kind 语义从 'ssh'/'fnos' 改成 'ups'，老行没法直接更新（host_id 不再对应 acme_deploy_target.id）。
-- 个人场景只有 1 台 UPS，历史曲线清空即可。
TRUNCATE TABLE `ups_sample`;
TRUNCATE TABLE `ups_state`;

-- 下线 acme_deploy_target.ups_monitor 列（幂等）。
DROP PROCEDURE IF EXISTS homer_acme_drop_ups_monitor;
DELIMITER //
CREATE PROCEDURE homer_acme_drop_ups_monitor()
BEGIN
  IF EXISTS (SELECT 1 FROM INFORMATION_SCHEMA.COLUMNS
             WHERE TABLE_SCHEMA = DATABASE()
               AND TABLE_NAME = 'acme_deploy_target'
               AND COLUMN_NAME = 'ups_monitor') THEN
    ALTER TABLE `acme_deploy_target` DROP COLUMN `ups_monitor`;
  END IF;
END //
DELIMITER ;
CALL homer_acme_drop_ups_monitor();
DROP PROCEDURE IF EXISTS homer_acme_drop_ups_monitor;

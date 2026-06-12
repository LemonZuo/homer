-- ESXi SSH 监控模块独立建表(参照 UPS 模块的解耦结构):
--   1. esxi_ssh_credential —— ESXi 专属凭证库(不与 UPS / ACME 共享,可配只读 / 低权账号)
--   2. esxi_host          —— ESXi 主机列表(含 endpoint + auth_json + 可选 bastion)
--   3. esxi_sample        —— 标量趋势时间序列(CPU 温度 / MCE / 磁盘 max / VM 数等)
--   4. esxi_state         —— 每台机器最新一次完整快照(变长结构用 JSON 列)
--
-- host_kind 列冗余固定为 'esxi',与 UPS 一样保留命名以便日后扩展。
--
-- 重建调试用(生产迁移不要启用,否则会清空 ESXi 配置与历史数据):
-- DROP TABLE IF EXISTS `esxi_state`;
-- DROP TABLE IF EXISTS `esxi_sample`;
-- DROP TABLE IF EXISTS `esxi_host`;
-- DROP TABLE IF EXISTS `esxi_ssh_credential`;

CREATE TABLE IF NOT EXISTS `esxi_ssh_credential` (
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
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='ESXi 模块专用 SSH 凭证';

CREATE TABLE IF NOT EXISTS `esxi_host` (
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
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='ESXi 监控目标机器';

CREATE TABLE IF NOT EXISTS `esxi_sample` (
  `id`                    BIGINT       NOT NULL AUTO_INCREMENT,
  `host_kind`             VARCHAR(16)  NOT NULL                COMMENT '主机类型',
  `host_id`               BIGINT       NOT NULL                COMMENT '主机 ID',
  `host_name`             VARCHAR(64)  NOT NULL DEFAULT ''     COMMENT '主机名',
  `cpu_max_c`             SMALLINT     NOT NULL DEFAULT -1     COMMENT 'CPU 最高温',
  `cpu_avg_c`             SMALLINT     NOT NULL DEFAULT -1     COMMENT 'CPU 平均温度',
  `cpu_tjmax_c`           SMALLINT     NOT NULL DEFAULT -1     COMMENT 'CPU TjMax',
  `mce_state`             VARCHAR(16)  NOT NULL DEFAULT ''     COMMENT 'MCE 状态',
  `mce_corrected_total`   BIGINT       NOT NULL DEFAULT 0      COMMENT 'MCE 可纠正错误',
  `mce_uncorrected_total` BIGINT       NOT NULL DEFAULT 0      COMMENT 'MCE 不可纠正错误',
  `disk_max_c`            SMALLINT     NOT NULL DEFAULT -1     COMMENT '磁盘最高温',
  `vm_total`              SMALLINT     NOT NULL DEFAULT -1     COMMENT 'VM 总数',
  `vm_powered_on`         SMALLINT     NOT NULL DEFAULT -1     COMMENT '已开机 VM 数',
  `sampled_at`            DATETIME(3)  NOT NULL                COMMENT '采样时刻',
  PRIMARY KEY (`id`),
  KEY `idx_host_time` (`host_kind`, `host_id`, `sampled_at` DESC),
  KEY `idx_sampled_at` (`sampled_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='ESXi 标量趋势时间序列';

CREATE TABLE IF NOT EXISTS `esxi_state` (
  `host_kind`        VARCHAR(16)  NOT NULL                COMMENT '主机类型',
  `host_id`          BIGINT       NOT NULL                COMMENT '主机 ID',
  `host_name`        VARCHAR(64)  NOT NULL DEFAULT ''     COMMENT '主机名',
  `reachable`        VARCHAR(1)   NOT NULL DEFAULT '0'    COMMENT '是否可达',
  `last_error`       VARCHAR(512) NOT NULL DEFAULT ''     COMMENT '最近错误',
  `platform_json`    TEXT         NULL                    COMMENT '平台信息',
  `cpu_static_json`  TEXT         NULL                    COMMENT 'CPU 静态信息',
  `memory_json`      TEXT         NULL                    COMMENT '内存信息',
  `runtime_json`     TEXT         NULL                    COMMENT '运行时使用率',
  `cpu_temp_json`    TEXT         NULL                    COMMENT 'CPU 温度',
  `mce_json`         TEXT         NULL                    COMMENT 'MCE 信息',
  `disk_json`        TEXT         NULL                    COMMENT '磁盘信息',
  `usb_json`         TEXT         NULL                    COMMENT 'USB 信息',
  `vm_json`          TEXT         NULL                    COMMENT 'VM 列表',
  `alert_state_json` TEXT         NULL                    COMMENT '告警状态',
  `last_alert_at`    DATETIME(3)  DEFAULT NULL            COMMENT '最近告警时刻',
  `sampled_at`       DATETIME(3)  DEFAULT NULL            COMMENT '采样时刻',
  `updated_at`       DATETIME(3)  NOT NULL                COMMENT '更新时刻',
  PRIMARY KEY (`host_kind`, `host_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='ESXi 最新状态快照(每台 host 一行)';

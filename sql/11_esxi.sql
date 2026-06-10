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
  `name`        VARCHAR(64)  NOT NULL                COMMENT '凭证名称,前端下拉显示',
  `username`    VARCHAR(128) NOT NULL DEFAULT ''     COMMENT '登录用户名',
  `auth_type`   VARCHAR(16)  NOT NULL DEFAULT 'password' COMMENT 'password | key',
  `password`    TEXT         NULL                    COMMENT 'password 模式登录密码',
  `private_key` TEXT         NULL                    COMMENT 'key 模式 OpenSSH 私钥',
  `passphrase`  TEXT         NULL                    COMMENT 'key 模式私钥口令(可选)',
  `created_at`  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at`  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_name` (`name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='ESXi 模块专用 SSH 凭证';

CREATE TABLE IF NOT EXISTS `esxi_host` (
  `id`          BIGINT       NOT NULL AUTO_INCREMENT,
  `name`        VARCHAR(64)  NOT NULL                COMMENT '机器名称',
  `endpoint`    VARCHAR(512) NOT NULL DEFAULT ''     COMMENT 'host:port(ESXi SSH 入口,默认 22)',
  `auth_json`   TEXT         NOT NULL                COMMENT 'JSON: {"auth_source":"inline|credential","credential_id":N,"username":...,"auth_type":"password|key","password":...,"private_key":...,"passphrase":...}',
  `config_json` TEXT         NOT NULL                COMMENT 'JSON: {"bastion_host_id":N} 或 {}',
  `enabled`     VARCHAR(1)   NOT NULL DEFAULT '1'    COMMENT '是否启用:1/0',
  `created_at`  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at`  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_name` (`name`),
  KEY `idx_enabled` (`enabled`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='ESXi 监控目标机器';

CREATE TABLE IF NOT EXISTS `esxi_sample` (
  `id`                    BIGINT       NOT NULL AUTO_INCREMENT,
  `host_kind`             VARCHAR(16)  NOT NULL                COMMENT '固定 esxi(保留列名,便于将来扩展)',
  `host_id`               BIGINT       NOT NULL                COMMENT 'esxi_host.id',
  `host_name`             VARCHAR(64)  NOT NULL DEFAULT ''     COMMENT '冗余主机名',
  `cpu_max_c`             SMALLINT     NOT NULL DEFAULT -1     COMMENT 'CPU 各核最高温;-1 表示无数据',
  `cpu_avg_c`             SMALLINT     NOT NULL DEFAULT -1     COMMENT 'CPU 平均温度;-1 表示无数据',
  `cpu_tjmax_c`           SMALLINT     NOT NULL DEFAULT -1     COMMENT 'CPU TjMax 节流阈值',
  `mce_state`             VARCHAR(16)  NOT NULL DEFAULT ''     COMMENT 'MCE 健康状态: Green / Yellow / Red',
  `mce_corrected_total`   BIGINT       NOT NULL DEFAULT 0      COMMENT 'MCE 累计可纠正错误数',
  `mce_uncorrected_total` BIGINT       NOT NULL DEFAULT 0      COMMENT 'MCE 累计不可纠正错误数',
  `disk_max_c`            SMALLINT     NOT NULL DEFAULT -1     COMMENT '所有盘里最热的那块温度;-1 表示无数据',
  `vm_total`              SMALLINT     NOT NULL DEFAULT -1     COMMENT 'VM 总数;-1 表示无数据',
  `vm_powered_on`         SMALLINT     NOT NULL DEFAULT -1     COMMENT '已开机 VM 数;-1 表示无数据',
  `sampled_at`            DATETIME(3)  NOT NULL                COMMENT '采样时刻',
  PRIMARY KEY (`id`),
  KEY `idx_host_time` (`host_kind`, `host_id`, `sampled_at` DESC),
  KEY `idx_sampled_at` (`sampled_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='ESXi 标量趋势时间序列';

CREATE TABLE IF NOT EXISTS `esxi_state` (
  `host_kind`        VARCHAR(16)  NOT NULL                COMMENT '固定 esxi',
  `host_id`          BIGINT       NOT NULL                COMMENT 'esxi_host.id',
  `host_name`        VARCHAR(64)  NOT NULL DEFAULT ''     COMMENT '冗余主机名',
  `reachable`        VARCHAR(1)   NOT NULL DEFAULT '0'    COMMENT '最近一轮是否拿到数据: 1/0',
  `last_error`       VARCHAR(512) NOT NULL DEFAULT ''     COMMENT '最近一轮失败原因(成功时清空)',
  `platform_json`    TEXT         NULL                    COMMENT 'JSON: 主机标识 + ESXi 版本',
  `cpu_static_json`  TEXT         NULL                    COMMENT 'JSON: CPU 静态信息(brand/family/cores/freq/...)',
  `memory_json`      TEXT         NULL                    COMMENT 'JSON: 内存信息(total/reliable/numa)',
  `runtime_json`     TEXT         NULL                    COMMENT 'JSON: CPU/内存运行时使用率',
  `cpu_temp_json`    TEXT         NULL                    COMMENT 'JSON: { tjmax_c, cores:[{id,temp_c,headroom_c}], max_c, avg_c }',
  `mce_json`         TEXT         NULL                    COMMENT 'JSON: { state, corrected_total, corrected_ewma, uncorrected_total, period_seconds }',
  `disk_json`        TEXT         NULL                    COMMENT 'JSON: [{device, model, type, temp_c, threshold_c, status}]',
  `usb_json`         TEXT         NULL                    COMMENT 'JSON: { controllers, arbitrator_running, available_for_passthrough, vm_owned }',
  `vm_json`          TEXT         NULL                    COMMENT 'JSON: [{id, name, guest_os, state}]',
  `last_alert_at`    DATETIME(3)  DEFAULT NULL            COMMENT '最近一次告警时刻(去抖留痕)',
  `sampled_at`       DATETIME(3)  DEFAULT NULL            COMMENT '最近一次成功采样时刻',
  `updated_at`       DATETIME(3)  NOT NULL                COMMENT '本行最近一次写入时刻',
  PRIMARY KEY (`host_kind`, `host_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='ESXi 最新状态快照(每台 host 一行)';

-- Homer 数据库 schema（首次建表）。
-- 全新部署直接执行本文件即可。
-- 注意：所有表均使用 DROP TABLE IF EXISTS，已有数据的环境不要直接重跑，
-- 请按需整理增量迁移（老库迁移见 sql/01_migrate_birthday_rename.sql）。


-- ============================================================
-- 生日提醒
-- ============================================================
-- 字段语义：
--   birthday          : 公历生日字符串 yyyy-MM-dd（用户输入）
--   chinese_birthday  : 农历生日中文字符串，由后端 BeforeSave 自动算
--   zodiac            : 生肖，由后端 BeforeSave 自动算
--   enabled           : varchar('0'/'1'); Go 侧用 BoolFlag 自动转 bool
DROP TABLE IF EXISTS `birthday_reminder`;
CREATE TABLE `birthday_reminder` (
  `id`               BIGINT       NOT NULL AUTO_INCREMENT,
  `name`             VARCHAR(30)  NOT NULL DEFAULT ''     COMMENT '姓名',
  `birthday`         VARCHAR(10)  NOT NULL DEFAULT ''     COMMENT '公历生日',
  `chinese_birthday` VARCHAR(30)  NOT NULL DEFAULT ''     COMMENT '农历生日',
  `zodiac`           VARCHAR(30)  NOT NULL DEFAULT ''     COMMENT '生肖',
  `enabled`          VARCHAR(1)   NOT NULL DEFAULT '1'    COMMENT '是否启用',
  PRIMARY KEY (`id`),
  KEY `idx_chinese_birthday` (`chinese_birthday`, `enabled`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='生日提醒';


-- ============================================================
-- ACME 自动签发
-- ============================================================
-- 老 Java 项目用 SSH 调远端 acme.sh，无数据模型；本仓库迁移到 lego 后需要自建表。
-- ACME CA 账号（Let's Encrypt / ZeroSSL / 自定义 directory）存 acme_account 表；
-- ZeroSSL 的 EAB KID/HMAC 也存库，不再从 env 读取。
-- DNS provider 凭证存 acme_credential 表，envs_json 里的 key 与 lego 文档里该 provider
-- 期望的环境变量名一致（如 alidns 用 ALICLOUD_ACCESS_KEY / ALICLOUD_SECRET_KEY）。

DROP TABLE IF EXISTS `acme_account`;
CREATE TABLE `acme_account` (
  `id`            BIGINT       NOT NULL AUTO_INCREMENT,
  `name`          VARCHAR(64)  NOT NULL                COMMENT '账号名称',
  `ca`            VARCHAR(32)  NOT NULL                COMMENT 'CA 类型',
  `directory_url` VARCHAR(512) NOT NULL DEFAULT ''     COMMENT 'ACME directory URL',
  `email`         VARCHAR(255) NOT NULL                COMMENT '注册邮箱',
  `eab_kid`       VARCHAR(255) NOT NULL DEFAULT ''     COMMENT 'EAB KID',
  `eab_hmac`      VARCHAR(512) NOT NULL DEFAULT ''     COMMENT 'EAB HMAC',
  `enabled`       VARCHAR(1)   NOT NULL DEFAULT '1'    COMMENT '是否启用',
  `created_at`    DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at`    DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_name` (`name`),
  KEY `idx_enabled` (`enabled`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='ACME CA 账号';

DROP TABLE IF EXISTS `acme_credential`;
CREATE TABLE `acme_credential` (
  `id`         BIGINT      NOT NULL AUTO_INCREMENT,
  `provider`   VARCHAR(64) NOT NULL                COMMENT 'DNS provider',
  `envs_json`  TEXT        NOT NULL                COMMENT '环境变量 JSON',
  `created_at` DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_provider` (`provider`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='ACME DNS provider 凭证';

DROP TABLE IF EXISTS `acme_domain`;
CREATE TABLE `acme_domain` (
  `id`           BIGINT       NOT NULL AUTO_INCREMENT,
  `main_domain`  VARCHAR(255) NOT NULL                COMMENT '主域名',
  `san_domains`  VARCHAR(1024) NOT NULL DEFAULT ''    COMMENT 'SAN 域名',
  `account_id`   BIGINT       NOT NULL DEFAULT 0      COMMENT '账号 ID',
  `provider`     VARCHAR(64)  NOT NULL                COMMENT 'DNS provider',
  `san_providers` VARCHAR(1024) NOT NULL DEFAULT ''   COMMENT 'SAN provider 覆盖',
  `enabled`      VARCHAR(1)   NOT NULL DEFAULT '1'    COMMENT '是否启用',
  `created_at`   DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at`   DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_main_domain` (`main_domain`),
  KEY `idx_account_id` (`account_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='ACME 签发域名';

DROP TABLE IF EXISTS `acme_safeline_target`;
DROP TABLE IF EXISTS `acme_safeline_deploy_config`;
DROP TABLE IF EXISTS `acme_ssh_target`;
DROP TABLE IF EXISTS `acme_ssh_deploy_config`;

DROP TABLE IF EXISTS `acme_deploy_target`;
CREATE TABLE `acme_deploy_target` (
  `id`          BIGINT       NOT NULL AUTO_INCREMENT,
  `name`        VARCHAR(64)  NOT NULL                COMMENT '目标名称',
  `kind`        VARCHAR(32)  NOT NULL                COMMENT '目标类型',
  `endpoint`    VARCHAR(512) NOT NULL DEFAULT ''     COMMENT '目标地址',
  `auth_json`   TEXT         NOT NULL                COMMENT '认证配置',
  `config_json` TEXT         NOT NULL                COMMENT '目标配置',
  `enabled`     VARCHAR(1)   NOT NULL DEFAULT '1'    COMMENT '是否启用',
  `created_at`  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at`  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_kind_name` (`kind`, `name`),
  KEY `idx_kind` (`kind`),
  KEY `idx_enabled` (`enabled`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='ACME 部署目标';

DROP TABLE IF EXISTS `acme_deploy_config`;
CREATE TABLE `acme_deploy_config` (
  `id`          BIGINT      NOT NULL AUTO_INCREMENT,
  `domain_id`   BIGINT      NOT NULL                COMMENT '域名 ID',
  `target_id`   BIGINT      NOT NULL                COMMENT '目标 ID',
  `kind`        VARCHAR(32) NOT NULL                COMMENT '目标类型',
  `name`        VARCHAR(64) NOT NULL DEFAULT ''     COMMENT '配置名称',
  `config_json` TEXT        NOT NULL                COMMENT '部署配置',
  `state_json`  TEXT        NOT NULL                COMMENT '部署状态',
  `auto_deploy` VARCHAR(1)  NOT NULL DEFAULT '0'    COMMENT '自动部署',
  `enabled`     VARCHAR(1)  NOT NULL DEFAULT '1'    COMMENT '是否启用',
  `created_at`  DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at`  DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_domain_id` (`domain_id`),
  KEY `idx_target_id` (`target_id`),
  KEY `idx_kind` (`kind`),
  KEY `idx_enabled_auto` (`enabled`, `auto_deploy`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='ACME 部署配置';

DROP TABLE IF EXISTS `acme_cert`;
CREATE TABLE `acme_cert` (
  `id`             BIGINT     NOT NULL AUTO_INCREMENT,
  `domain_id`      BIGINT     NOT NULL                COMMENT '域名 ID',
  `cert_pem`       MEDIUMTEXT NOT NULL                COMMENT '证书',
  `key_pem`        MEDIUMTEXT NOT NULL                COMMENT '私钥',
  `chain_pem`      MEDIUMTEXT NOT NULL                COMMENT '中间证书',
  `fullchain_pem`  MEDIUMTEXT NOT NULL                COMMENT '完整链',
  `serial`         VARCHAR(128) NOT NULL DEFAULT ''   COMMENT '序列号',
  `not_before`     DATETIME   NULL                    COMMENT '生效时间',
  `not_after`      DATETIME   NULL                    COMMENT '到期时间',
  `status`         VARCHAR(16) NOT NULL DEFAULT 'active' COMMENT '状态',
  `revoked_at`     DATETIME   NULL                    COMMENT '吊销时间',
  `issued_at`      DATETIME   NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '签发时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_domain_id` (`domain_id`),
  KEY `idx_not_after` (`not_after`),
  KEY `idx_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='ACME 证书';

DROP TABLE IF EXISTS `acme_issue_task`;
CREATE TABLE `acme_issue_task` (
  `id`           BIGINT       NOT NULL AUTO_INCREMENT,
  `domain_id`    BIGINT       NOT NULL                COMMENT '域名 ID',
  `main_domain`  VARCHAR(255) NOT NULL DEFAULT ''     COMMENT '主域名',
  `kind`         VARCHAR(32)  NOT NULL                COMMENT '任务类型',
  `status`       VARCHAR(16)  NOT NULL                COMMENT '状态',
  `started_at`   DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '开始时间',
  `finished_at`  DATETIME     NULL                    COMMENT '结束时间',
  `log_text`     MEDIUMTEXT   NOT NULL                COMMENT '日志',
  `error_msg`    VARCHAR(1024) NOT NULL DEFAULT ''    COMMENT '错误信息',
  `attempt`      INT          NOT NULL DEFAULT 0      COMMENT '已执行次数',
  `max_attempt`  INT          NOT NULL DEFAULT 1      COMMENT '允许总次数',
  `config_id`    BIGINT       NOT NULL DEFAULT 0      COMMENT '部署配置 ID',
  `next_retry_at` DATETIME    NULL                    COMMENT '下次重试时刻',
  PRIMARY KEY (`id`),
  KEY `idx_domain_started` (`domain_id`, `started_at`),
  KEY `idx_status` (`status`),
  KEY `idx_retry` (`status`, `next_retry_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='ACME 签发任务';


-- ============================================================
-- 短信转发器（SmsForwarder Android）
-- ============================================================
-- 老 Java 项目里用 application.yml 单实例配置；本仓库改为多实例存库 + 前端切换。
-- 对接服务端「客户端安全措施」全部 4 种模式（auth_mode）：
--   0 无、1 签名(sign_key)、2 RSA(rsa_public_key)、3 SM4(sm4_key)。
-- 老 Java 的 RSA/SM4 实现有误，这里按 SmsForwarder Android 源码重新对齐。

DROP TABLE IF EXISTS `sms_forwarder`;
CREATE TABLE `sms_forwarder` (
  `id`         BIGINT       NOT NULL AUTO_INCREMENT,
  `name`       VARCHAR(64)  NOT NULL                COMMENT '转发器名称',
  `server_url` VARCHAR(512) NOT NULL DEFAULT ''     COMMENT '服务端地址',
  `auth_mode`  INT          NOT NULL DEFAULT 1      COMMENT '安全模式',
  `sign_key`   VARCHAR(512) NOT NULL DEFAULT ''     COMMENT '签名密钥',
  `rsa_public_key` TEXT     NULL                    COMMENT 'RSA 公钥',
  `sm4_key`    VARCHAR(64)  NOT NULL DEFAULT ''     COMMENT 'SM4 密钥',
  `timeout_seconds` INT     NOT NULL DEFAULT 30     COMMENT '超时秒数',
  `enabled`    VARCHAR(1)   NOT NULL DEFAULT '1'    COMMENT '是否启用',
  `created_at` DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_name` (`name`),
  KEY `idx_enabled` (`enabled`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='短信转发器';


-- ============================================================
-- SSH 登录凭证
-- ============================================================
-- 多台 acme_deploy_target 可引用同一条凭证，避免重复输入。
-- 引用关系存在 acme_deploy_target.auth_json 里：{"auth_source":"credential","credential_id":42}。
-- 不在 acme_deploy_target 加 FK 列，保持 driver 多态：safeline 等其他 kind 不感知此表。
DROP TABLE IF EXISTS `ssh_credential`;
CREATE TABLE `ssh_credential` (
  `id`           BIGINT       NOT NULL AUTO_INCREMENT,
  `name`         VARCHAR(64)  NOT NULL                COMMENT '凭证名称',
  `username`     VARCHAR(128) NOT NULL DEFAULT ''     COMMENT '用户名',
  `auth_type`    VARCHAR(16)  NOT NULL DEFAULT 'password' COMMENT '认证类型',
  `password`     TEXT         NULL                    COMMENT '密码',
  `private_key`  TEXT         NULL                    COMMENT '私钥',
  `passphrase`   TEXT         NULL                    COMMENT '私钥口令',
  `created_at`   DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at`   DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_name` (`name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='SSH 登录凭证';


-- ============================================================
-- 事项提醒（一次性日期）
-- ============================================================
-- 触发规则：事项当天前 lead_days 天起，每天 cron 推送一次。
-- 去重：last_notified_at 记录最近一次发送时间，同一天命中则不重复推。
-- 过期：event_date < today 直接跳过；如需归档由人工删除。

DROP TABLE IF EXISTS `event_reminder`;
CREATE TABLE `event_reminder` (
  `id`                BIGINT       NOT NULL AUTO_INCREMENT,
  `title`             VARCHAR(128) NOT NULL                COMMENT '事项标题',
  `event_date`        VARCHAR(10)  NOT NULL                COMMENT '事项日期',
  `lead_days`         INT          NOT NULL DEFAULT 5      COMMENT '提前天数',
  `remark`            VARCHAR(255) NOT NULL DEFAULT ''     COMMENT '备注',
  `enabled`           VARCHAR(1)   NOT NULL DEFAULT '1'    COMMENT '是否启用',
  `last_notified_at`  DATETIME     NULL                    COMMENT '最近推送时间',
  `created_at`        DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at`        DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_enabled_event_date` (`enabled`, `event_date`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='事项提醒';


-- ============================================================
-- 通知通道与模块绑定
-- ============================================================
-- 通道类型决定 config_json 结构：
--   wework:  {"corp_id","agent_id","secret","tag_id"}
--   email:   {"api_key","from","to"}（Resend）
--   webhook: {"url"}
-- 凭证当前以明文形式存库（P0 加密待办）。
-- module 取值：birthday | event | bypass | scheduler_alert | ups | esxi，由代码常量约束。

DROP TABLE IF EXISTS `notify_channel`;
CREATE TABLE `notify_channel` (
  `id`          BIGINT      NOT NULL AUTO_INCREMENT,
  `name`        VARCHAR(64) NOT NULL                COMMENT '通道名称',
  `type`        VARCHAR(16) NOT NULL                COMMENT '通道类型',
  `config_json` TEXT        NOT NULL                COMMENT '通道配置',
  `enabled`     VARCHAR(1)  NOT NULL DEFAULT '1'    COMMENT '是否启用',
  `created_at`  DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at`  DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_name` (`name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='通知通道';

DROP TABLE IF EXISTS `notify_binding`;
CREATE TABLE `notify_binding` (
  `id`         BIGINT      NOT NULL AUTO_INCREMENT,
  `module`     VARCHAR(32) NOT NULL                COMMENT '模块',
  `channel_id` BIGINT      NOT NULL                COMMENT '通道 ID',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_notify_bind` (`module`, `channel_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='模块通道绑定';


-- ============================================================
-- 调度任务执行状态
-- ============================================================
-- 重启后面板/healthz 仍可见最近一次结果。
-- 历史环形仍只在内存，这里只保留「最近一次」+ 连续失败计数（告警防抖用）。

DROP TABLE IF EXISTS `scheduler_job_state`;
CREATE TABLE `scheduler_job_state` (
  `name`         VARCHAR(64) NOT NULL                COMMENT '任务名',
  `last_start`   DATETIME    NULL                    COMMENT '最近开始时间',
  `last_end`     DATETIME    NULL                    COMMENT '最近结束时间',
  `last_ok`      VARCHAR(1)  NOT NULL DEFAULT '1'    COMMENT '是否成功',
  `last_err`     TEXT        NULL                    COMMENT '错误信息',
  `last_trigger` VARCHAR(16) NOT NULL DEFAULT ''     COMMENT '触发方式',
  `consec_fails` INT         NOT NULL DEFAULT 0      COMMENT '连续失败次数',
  `updated_at`   DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='调度任务状态';


-- ============================================================
-- UPS 监控
-- ============================================================
-- 数据源：ups_host 表里 enabled='1' 的机器，通过 SSH 跑 `upsc` 拉 NUT 字段。
-- SSH 凭证独立维护在 ups_ssh_credential，与 ACME 的 ssh_credential 完全隔离，
-- 这样可以给 UPS 配只允许执行 upsc 的低权账号。
-- 落两张表：
--   ups_sample —— 时间序列，按 UPS_RETENTION_DAYS 保留（默认 7d），定时清理
--   ups_state  —— 每台 (host, ups) 的最新状态快照，告警状态机比对用
-- 哨兵值：battery_percent / runtime_minutes / input_voltage / output_voltage /
-- load_percent / real_power / battery_voltage / battery_nominal_voltage 缺数据时存 -1。
-- 告警走 notify.Hub 的 module='ups' 通道，状态转换才发（mains/battery/low_battery）。

DROP TABLE IF EXISTS `ups_ssh_credential`;
CREATE TABLE `ups_ssh_credential` (
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
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='UPS SSH 凭证';

DROP TABLE IF EXISTS `ups_host`;
CREATE TABLE `ups_host` (
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
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='UPS 监控机器';

DROP TABLE IF EXISTS `ups_sample`;
CREATE TABLE `ups_sample` (
  `id`              BIGINT       NOT NULL AUTO_INCREMENT,
  `host_kind`       VARCHAR(16)  NOT NULL                COMMENT '主机类型',
  `host_id`         BIGINT       NOT NULL                COMMENT '主机 ID',
  `host_name`       VARCHAR(64)  NOT NULL DEFAULT ''     COMMENT '主机名',
  `ups_name`        VARCHAR(64)  NOT NULL                COMMENT 'UPS 名称',
  `mfr`             VARCHAR(128) NOT NULL DEFAULT ''     COMMENT '品牌',
  `model`           VARCHAR(128) NOT NULL DEFAULT ''     COMMENT '型号',
  `power_source`    VARCHAR(16)  NOT NULL DEFAULT 'unknown' COMMENT '供电状态',
  `battery_percent` TINYINT      NOT NULL DEFAULT -1     COMMENT '剩余电量',
  `runtime_minutes` INT          NOT NULL DEFAULT -1     COMMENT '续航分钟',
  `battery_voltage`         DECIMAL(5,1) NOT NULL DEFAULT -1 COMMENT '电池端电压',
  `battery_nominal_voltage` DECIMAL(5,1) NOT NULL DEFAULT -1 COMMENT '电池标称电压',
  `battery_type`            VARCHAR(16)  NOT NULL DEFAULT '' COMMENT '电池类型',
  `input_voltage`   DECIMAL(6,1) NOT NULL DEFAULT -1     COMMENT '输入电压',
  `output_voltage`  DECIMAL(6,1) NOT NULL DEFAULT -1     COMMENT '输出电压',
  `load_percent`    TINYINT      NOT NULL DEFAULT -1     COMMENT '负载百分比',
  `real_power`      SMALLINT     NOT NULL DEFAULT -1     COMMENT '实时功率',
  `raw_status`      VARCHAR(64)  NOT NULL DEFAULT ''     COMMENT '原始状态',
  `sampled_at`      DATETIME(3)  NOT NULL                COMMENT '采样时刻',
  PRIMARY KEY (`id`),
  KEY `idx_host_ups_time` (`host_kind`, `host_id`, `ups_name`, `sampled_at` DESC),
  KEY `idx_sampled_at` (`sampled_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='UPS 采样';

DROP TABLE IF EXISTS `ups_state`;
CREATE TABLE `ups_state` (
  `host_kind`            VARCHAR(16)  NOT NULL                COMMENT '主机类型',
  `host_id`              BIGINT       NOT NULL                COMMENT '主机 ID',
  `ups_name`             VARCHAR(64)  NOT NULL                COMMENT 'UPS 名称',
  `host_name`            VARCHAR(64)  NOT NULL DEFAULT ''     COMMENT '主机名',
  `mfr`                  VARCHAR(128) NOT NULL DEFAULT ''     COMMENT '品牌',
  `model`                VARCHAR(128) NOT NULL DEFAULT ''     COMMENT '型号',
  `last_power_source`    VARCHAR(16)  NOT NULL DEFAULT 'unknown' COMMENT '最近供电状态',
  `last_battery_percent` TINYINT      NOT NULL DEFAULT -1     COMMENT '最近电量',
  `last_runtime_minutes` INT          NOT NULL DEFAULT -1     COMMENT '最近续航分钟',
  `last_battery_voltage`         DECIMAL(5,1) NOT NULL DEFAULT -1 COMMENT '最近电池端电压',
  `last_battery_nominal_voltage` DECIMAL(5,1) NOT NULL DEFAULT -1 COMMENT '最近电池标称电压',
  `last_battery_type`            VARCHAR(16)  NOT NULL DEFAULT '' COMMENT '最近电池类型',
  `last_input_voltage`   DECIMAL(6,1) NOT NULL DEFAULT -1     COMMENT '最近输入电压',
  `last_output_voltage`  DECIMAL(6,1) NOT NULL DEFAULT -1     COMMENT '最近输出电压',
  `last_load_percent`    TINYINT      NOT NULL DEFAULT -1     COMMENT '最近负载百分比',
  `last_real_power`      SMALLINT     NOT NULL DEFAULT -1     COMMENT '最近实时功率',
  `last_raw_status`      VARCHAR(64)  NOT NULL DEFAULT ''     COMMENT '最近原始状态',
  `last_alert_at`        DATETIME(3)  DEFAULT NULL            COMMENT '最近告警时刻',
  `updated_at`           DATETIME(3)  NOT NULL                COMMENT '更新时刻',
  PRIMARY KEY (`host_kind`, `host_id`, `ups_name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='UPS 最新状态';

-- ============================================================================
-- ESXi SSH 监控模块（与 UPS 同结构：独立凭证 + 独立 host + sample/state 双表）
-- ============================================================================
-- esxi_sample 只存标量趋势（CPU/盘 温度、MCE 计数、VM 数等），变长结构（cores
-- 数组、disk 数组、vm 列表等）放在 esxi_state 的 *_json 列里。
-- 缺数据统一用 -1（数值列）或空串（字符串列）占位，避免 NULL 让前端判空炸图。
-- 告警走 notify.Hub 的 module='esxi' 通道，由 service 状态机决定是否抑制。
--
-- 增量脚本：sql/11_esxi.sql（CREATE TABLE IF NOT EXISTS，可与 AutoMigrate 共存）。

DROP TABLE IF EXISTS `esxi_ssh_credential`;
CREATE TABLE `esxi_ssh_credential` (
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
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='ESXi SSH 凭证';

DROP TABLE IF EXISTS `esxi_host`;
CREATE TABLE `esxi_host` (
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
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='ESXi 监控机器';

DROP TABLE IF EXISTS `esxi_sample`;
CREATE TABLE `esxi_sample` (
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
  `cpu_usage_percent`     SMALLINT     NOT NULL DEFAULT -1     COMMENT 'CPU 使用率',
  `memory_usage_percent`  SMALLINT     NOT NULL DEFAULT -1     COMMENT '内存使用率',
  `vm_total`              SMALLINT     NOT NULL DEFAULT -1     COMMENT 'VM 总数',
  `vm_powered_on`         SMALLINT     NOT NULL DEFAULT -1     COMMENT '已开机 VM 数',
  `cpu_temp_json`         TEXT         NULL                    COMMENT '每核温度明细',
  `disk_temp_json`        TEXT         NULL                    COMMENT '每盘温度明细',
  `disk_usage_json`       TEXT         NULL                    COMMENT '每盘容量明细',
  `sampled_at`            DATETIME(3)  NOT NULL                COMMENT '采样时刻',
  PRIMARY KEY (`id`),
  KEY `idx_host_time` (`host_kind`, `host_id`, `sampled_at` DESC),
  KEY `idx_sampled_at` (`sampled_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='ESXi 采样';

DROP TABLE IF EXISTS `esxi_state`;
CREATE TABLE `esxi_state` (
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
  `boot_json`        TEXT         NULL                    COMMENT '启动信息',
  `nic_json`         TEXT         NULL                    COMMENT '物理网卡',
  `topology_json`    TEXT         NULL                    COMMENT '网络拓扑',
  `alert_state_json` TEXT         NULL                    COMMENT '告警状态',
  `last_alert_at`    DATETIME(3)  DEFAULT NULL            COMMENT '最近告警时刻',
  `sampled_at`       DATETIME(3)  DEFAULT NULL            COMMENT '采样时刻',
  `updated_at`       DATETIME(3)  NOT NULL                COMMENT '更新时刻',
  PRIMARY KEY (`host_kind`, `host_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='ESXi 最新状态';

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
  `id`               BIGINT       NOT NULL AUTO_INCREMENT COMMENT '唯一标识',
  `name`             VARCHAR(30)  NOT NULL DEFAULT ''     COMMENT '姓名',
  `birthday`         VARCHAR(10)  NOT NULL DEFAULT ''     COMMENT '公历生日 yyyy-MM-dd',
  `chinese_birthday` VARCHAR(30)  NOT NULL DEFAULT ''     COMMENT '农历生日（后端自动）',
  `zodiac`           VARCHAR(30)  NOT NULL DEFAULT ''     COMMENT '生肖（后端自动）',
  `enabled`          VARCHAR(1)   NOT NULL DEFAULT '1'    COMMENT '是否启用：1/0',
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
  `name`          VARCHAR(64)  NOT NULL                COMMENT '账号名称，前端显示与本地目录命名使用',
  `ca`            VARCHAR(32)  NOT NULL                COMMENT 'letsencrypt | zerossl | custom',
  `directory_url` VARCHAR(512) NOT NULL DEFAULT ''     COMMENT 'ACME directory URL；内置 CA 自动回填',
  `email`         VARCHAR(255) NOT NULL                COMMENT 'ACME 注册邮箱',
  `eab_kid`       VARCHAR(255) NOT NULL DEFAULT ''     COMMENT 'External Account Binding KID，ZeroSSL 必填',
  `eab_hmac`      VARCHAR(512) NOT NULL DEFAULT ''     COMMENT 'External Account Binding HMAC，ZeroSSL 必填',
  `enabled`       VARCHAR(1)   NOT NULL DEFAULT '1'    COMMENT '是否启用：1/0',
  `created_at`    DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at`    DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_name` (`name`),
  KEY `idx_enabled` (`enabled`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='ACME CA 账号配置';

DROP TABLE IF EXISTS `acme_credential`;
CREATE TABLE `acme_credential` (
  `id`         BIGINT      NOT NULL AUTO_INCREMENT,
  `provider`   VARCHAR(64) NOT NULL                COMMENT 'lego DNS provider key',
  `envs_json`  TEXT        NOT NULL                COMMENT 'JSON: {"LEGO_ENV_KEY":"value"}',
  `created_at` DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_provider` (`provider`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='ACME DNS provider 凭证';

DROP TABLE IF EXISTS `acme_domain`;
CREATE TABLE `acme_domain` (
  `id`           BIGINT       NOT NULL AUTO_INCREMENT COMMENT '唯一标识',
  `main_domain`  VARCHAR(255) NOT NULL                COMMENT '主域名',
  `san_domains`  VARCHAR(1024) NOT NULL DEFAULT ''    COMMENT 'SAN，逗号分隔；可空',
  `account_id`   BIGINT       NOT NULL DEFAULT 0      COMMENT 'ACME CA 账号 ID',
  `provider`     VARCHAR(64)  NOT NULL                COMMENT 'lego DNS provider key（alidns/cloudflare/dnspod...）',
  `san_providers` VARCHAR(1024) NOT NULL DEFAULT ''   COMMENT '按域名指定 provider 的覆盖表 JSON {"b.com":"alidns"}；空=全用 provider',
  `enabled`      VARCHAR(1)   NOT NULL DEFAULT '1'    COMMENT '是否启用自动续期：1/0',
  `created_at`   DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at`   DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_main_domain` (`main_domain`),
  KEY `idx_account_id` (`account_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='ACME 自动签发域名（同一主域名可有多张证书并存）';

DROP TABLE IF EXISTS `acme_safeline_target`;
DROP TABLE IF EXISTS `acme_safeline_deploy_config`;
DROP TABLE IF EXISTS `acme_ssh_target`;
DROP TABLE IF EXISTS `acme_ssh_deploy_config`;

DROP TABLE IF EXISTS `acme_deploy_target`;
CREATE TABLE `acme_deploy_target` (
  `id`          BIGINT       NOT NULL AUTO_INCREMENT,
  `name`        VARCHAR(64)  NOT NULL                COMMENT '目标名称',
  `kind`        VARCHAR(32)  NOT NULL                COMMENT 'ssh | safeline | upload_cas | ...',
  `endpoint`    VARCHAR(512) NOT NULL DEFAULT ''     COMMENT '目标地址；ssh 为 host:port，雷池为管理端根地址，upload_cas 留空',
  `auth_json`   TEXT         NOT NULL                COMMENT 'driver 认证配置 JSON',
  `config_json` TEXT         NOT NULL                COMMENT 'driver 目标配置 JSON',
  `enabled`     VARCHAR(1)   NOT NULL DEFAULT '1'    COMMENT '是否启用：1/0',
  `created_at`  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at`  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_kind_name` (`kind`, `name`),
  KEY `idx_kind` (`kind`),
  KEY `idx_enabled` (`enabled`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='ACME 证书部署目标';

DROP TABLE IF EXISTS `acme_deploy_config`;
CREATE TABLE `acme_deploy_config` (
  `id`          BIGINT      NOT NULL AUTO_INCREMENT,
  `domain_id`   BIGINT      NOT NULL                COMMENT 'ACME 域名 ID',
  `target_id`   BIGINT      NOT NULL                COMMENT '部署目标 ID',
  `kind`        VARCHAR(32) NOT NULL                COMMENT 'ssh | safeline | upload_cas | ...',
  `name`        VARCHAR(64) NOT NULL DEFAULT ''     COMMENT '部署配置名称',
  `config_json` TEXT        NOT NULL                COMMENT 'driver 部署配置 JSON',
  `state_json`  TEXT        NOT NULL                COMMENT 'driver 部署状态 JSON，如雷池 cert_id',
  `auto_deploy` VARCHAR(1)  NOT NULL DEFAULT '0'    COMMENT '签发/续期成功后是否自动部署：1/0',
  `enabled`     VARCHAR(1)  NOT NULL DEFAULT '1'    COMMENT '是否启用：1/0',
  `created_at`  DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at`  DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_domain_id` (`domain_id`),
  KEY `idx_target_id` (`target_id`),
  KEY `idx_kind` (`kind`),
  KEY `idx_enabled_auto` (`enabled`, `auto_deploy`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='ACME 证书部署配置';

DROP TABLE IF EXISTS `acme_cert`;
CREATE TABLE `acme_cert` (
  `id`             BIGINT     NOT NULL AUTO_INCREMENT,
  `domain_id`      BIGINT     NOT NULL,
  `cert_pem`       MEDIUMTEXT NOT NULL,
  `key_pem`        MEDIUMTEXT NOT NULL,
  `chain_pem`      MEDIUMTEXT NOT NULL,
  `fullchain_pem`  MEDIUMTEXT NOT NULL,
  `serial`         VARCHAR(128) NOT NULL DEFAULT '',
  `not_before`     DATETIME   NULL,
  `not_after`      DATETIME   NULL,
  `status`         VARCHAR(16) NOT NULL DEFAULT 'active' COMMENT 'active | revoked',
  `revoked_at`     DATETIME   NULL,
  `issued_at`      DATETIME   NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_domain_id` (`domain_id`),
  KEY `idx_not_after` (`not_after`),
  KEY `idx_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='ACME 签发的证书';

DROP TABLE IF EXISTS `acme_issue_task`;
CREATE TABLE `acme_issue_task` (
  `id`           BIGINT       NOT NULL AUTO_INCREMENT,
  `domain_id`    BIGINT       NOT NULL,
  `main_domain`  VARCHAR(255) NOT NULL DEFAULT '',
  `kind`         VARCHAR(32)  NOT NULL COMMENT 'issue | renew | revoke | deploy_ssh | deploy_safeline | deploy_upload_cas | deploy',
  `status`       VARCHAR(16)  NOT NULL COMMENT 'pending | running | success | failed | retrying',
  `started_at`   DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `finished_at`  DATETIME     NULL,
  `log_text`     MEDIUMTEXT   NOT NULL,
  `error_msg`    VARCHAR(1024) NOT NULL DEFAULT '',
  `attempt`      INT          NOT NULL DEFAULT 0 COMMENT '已执行次数',
  `max_attempt`  INT          NOT NULL DEFAULT 1 COMMENT '允许总次数，1=不重试',
  `config_id`    BIGINT       NOT NULL DEFAULT 0 COMMENT '触发的持久化部署配置 id，>0 才参与重试',
  `next_retry_at` DATETIME    NULL COMMENT '下次可重试时刻',
  PRIMARY KEY (`id`),
  KEY `idx_domain_started` (`domain_id`, `started_at`),
  KEY `idx_status` (`status`),
  KEY `idx_retry` (`status`, `next_retry_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='ACME 签发/续期任务流水';


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
  `name`       VARCHAR(64)  NOT NULL                COMMENT '转发器名称，前端下拉显示',
  `server_url` VARCHAR(512) NOT NULL DEFAULT ''     COMMENT '服务端地址，如 http://192.168.1.100:5000',
  `auth_mode`  INT          NOT NULL DEFAULT 1      COMMENT '客户端安全措施：0无 1签名 2RSA 3SM4',
  `sign_key`   VARCHAR(512) NOT NULL DEFAULT ''     COMMENT '签名模式 HmacSHA256 密钥',
  `rsa_public_key` TEXT     NULL                    COMMENT 'RSA 模式服务端公钥（X.509/SPKI DER 的 Base64）',
  `sm4_key`    VARCHAR(64)  NOT NULL DEFAULT ''     COMMENT 'SM4 模式密钥（16 字节，32 位 hex）',
  `timeout_seconds` INT     NOT NULL DEFAULT 30     COMMENT '请求超时秒数，旧机器可适当调大',
  `enabled`    VARCHAR(1)   NOT NULL DEFAULT '1'    COMMENT '是否启用：1/0',
  `created_at` DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_name` (`name`),
  KEY `idx_enabled` (`enabled`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='短信转发器服务端配置';


-- ============================================================
-- SSH 登录凭证
-- ============================================================
-- 多台 acme_deploy_target 可引用同一条凭证，避免重复输入。
-- 引用关系存在 acme_deploy_target.auth_json 里：{"auth_source":"credential","credential_id":42}。
-- 不在 acme_deploy_target 加 FK 列，保持 driver 多态：safeline 等其他 kind 不感知此表。
DROP TABLE IF EXISTS `ssh_credential`;
CREATE TABLE `ssh_credential` (
  `id`           BIGINT       NOT NULL AUTO_INCREMENT,
  `name`         VARCHAR(64)  NOT NULL                COMMENT '凭证名称，前端下拉显示',
  `username`     VARCHAR(128) NOT NULL DEFAULT ''     COMMENT '登录用户名（与密钥/密码绑定）',
  `auth_type`    VARCHAR(16)  NOT NULL DEFAULT 'password' COMMENT 'password | key',
  `password`     TEXT         NULL                    COMMENT 'password 模式登录密码',
  `private_key`  TEXT         NULL                    COMMENT 'key 模式 OpenSSH 私钥',
  `passphrase`   TEXT         NULL                    COMMENT 'key 模式私钥口令（可选）',
  `created_at`   DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at`   DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_name` (`name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='SSH 登录凭证（可被多台机器复用）';


-- ============================================================
-- 事项提醒（一次性日期）
-- ============================================================
-- 触发规则：事项当天前 lead_days 天起，每天 cron 推送一次。
-- 去重：last_notified_at 记录最近一次发送时间，同一天命中则不重复推。
-- 过期：event_date < today 直接跳过；如需归档由人工删除。

DROP TABLE IF EXISTS `event_reminder`;
CREATE TABLE `event_reminder` (
  `id`                BIGINT       NOT NULL AUTO_INCREMENT,
  `title`             VARCHAR(128) NOT NULL                COMMENT '事项标题，推送时作为正文主体',
  `event_date`        VARCHAR(10)  NOT NULL                COMMENT '事项日期 YYYY-MM-DD',
  `lead_days`         INT          NOT NULL DEFAULT 5      COMMENT '提前多少天起开始每天提醒',
  `remark`            VARCHAR(255) NOT NULL DEFAULT ''     COMMENT '备注，附加在推送末尾',
  `enabled`           VARCHAR(1)   NOT NULL DEFAULT '1'    COMMENT '是否启用：1/0',
  `last_notified_at`  DATETIME     NULL                    COMMENT '最近一次推送时间，用于同一天去重',
  `created_at`        DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at`        DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_enabled_event_date` (`enabled`, `event_date`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='事项提醒（一次性日期）';


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
  `name`        VARCHAR(64) NOT NULL                COMMENT '通道名称，前端显示',
  `type`        VARCHAR(16) NOT NULL                COMMENT 'wework | email | webhook',
  `config_json` TEXT        NOT NULL                COMMENT '类型相关配置 JSON',
  `enabled`     VARCHAR(1)  NOT NULL DEFAULT '1'    COMMENT '是否启用：1/0',
  `created_at`  DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at`  DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_name` (`name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='通知通道（出站）';

DROP TABLE IF EXISTS `notify_binding`;
CREATE TABLE `notify_binding` (
  `id`         BIGINT      NOT NULL AUTO_INCREMENT,
  `module`     VARCHAR(32) NOT NULL                COMMENT 'birthday | event | bypass | scheduler_alert | ups | esxi',
  `channel_id` BIGINT      NOT NULL                COMMENT 'notify_channel.id',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_notify_bind` (`module`, `channel_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='模块 → 通道 多对多绑定';


-- ============================================================
-- 调度任务执行状态
-- ============================================================
-- 重启后面板/healthz 仍可见最近一次结果。
-- 历史环形仍只在内存，这里只保留「最近一次」+ 连续失败计数（告警防抖用）。

DROP TABLE IF EXISTS `scheduler_job_state`;
CREATE TABLE `scheduler_job_state` (
  `name`         VARCHAR(64) NOT NULL                COMMENT '任务名（唯一）',
  `last_start`   DATETIME    NULL                    COMMENT '最近一次开始时间',
  `last_end`     DATETIME    NULL                    COMMENT '最近一次结束时间',
  `last_ok`      VARCHAR(1)  NOT NULL DEFAULT '1'    COMMENT '最近一次是否成功：1/0',
  `last_err`     TEXT        NULL                    COMMENT '最近一次错误信息',
  `last_trigger` VARCHAR(16) NOT NULL DEFAULT ''     COMMENT 'cron | manual',
  `consec_fails` INT         NOT NULL DEFAULT 0      COMMENT '连续失败次数，成功清零',
  `updated_at`   DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='调度任务最近一次执行状态';


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
  `name`        VARCHAR(64)  NOT NULL                COMMENT '凭证名称，前端下拉显示',
  `username`    VARCHAR(128) NOT NULL DEFAULT ''     COMMENT '登录用户名',
  `auth_type`   VARCHAR(16)  NOT NULL DEFAULT 'password' COMMENT 'password | key',
  `password`    TEXT         NULL                    COMMENT 'password 模式登录密码',
  `private_key` TEXT         NULL                    COMMENT 'key 模式 OpenSSH 私钥',
  `passphrase`  TEXT         NULL                    COMMENT 'key 模式私钥口令（可选）',
  `created_at`  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at`  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_name` (`name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='UPS 模块专用 SSH 凭证';

DROP TABLE IF EXISTS `ups_host`;
CREATE TABLE `ups_host` (
  `id`          BIGINT       NOT NULL AUTO_INCREMENT,
  `name`        VARCHAR(64)  NOT NULL                COMMENT '机器名称',
  `endpoint`    VARCHAR(512) NOT NULL DEFAULT ''     COMMENT 'host:port（NUT upsd 所在机器 SSH 入口）',
  `auth_json`   TEXT         NOT NULL                COMMENT 'JSON: {"auth_source":"inline|credential","credential_id":N,"username":...,"auth_type":"password|key","password":...,"private_key":...,"passphrase":...}',
  `config_json` TEXT         NOT NULL                COMMENT 'JSON: {"bastion_id":N} 或空 {}',
  `enabled`     VARCHAR(1)   NOT NULL DEFAULT '1'    COMMENT '是否启用：1/0',
  `created_at`  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at`  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_name` (`name`),
  KEY `idx_enabled` (`enabled`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='UPS 监控目标机器';

DROP TABLE IF EXISTS `ups_sample`;
CREATE TABLE `ups_sample` (
  `id`              BIGINT       NOT NULL AUTO_INCREMENT,
  `host_kind`       VARCHAR(16)  NOT NULL                COMMENT '固定 ups（保留列名，便于将来扩展）',
  `host_id`         BIGINT       NOT NULL                COMMENT 'ups_host.id',
  `host_name`       VARCHAR(64)  NOT NULL DEFAULT ''     COMMENT '冗余主机名，便于历史查询不依赖关联表',
  `ups_name`        VARCHAR(64)  NOT NULL                COMMENT 'NUT upsname（同一台机器可能挂多台 UPS）',
  `mfr`             VARCHAR(128) NOT NULL DEFAULT ''     COMMENT '品牌',
  `model`           VARCHAR(128) NOT NULL DEFAULT ''     COMMENT '型号',
  `power_source`    VARCHAR(16)  NOT NULL DEFAULT 'unknown' COMMENT 'mains | battery | low_battery | unknown',
  `battery_percent` TINYINT      NOT NULL DEFAULT -1     COMMENT '剩余电量百分比；-1 表示无数据',
  `runtime_minutes` INT          NOT NULL DEFAULT -1     COMMENT '预估续航分钟；-1 表示无数据',
  `battery_voltage`         DECIMAL(5,1) NOT NULL DEFAULT -1 COMMENT '当前电池端电压 V；-1 表示无数据',
  `battery_nominal_voltage` DECIMAL(5,1) NOT NULL DEFAULT -1 COMMENT '电池组标称电压 V（12/24/48...）；-1 表示无数据',
  `battery_type`            VARCHAR(16)  NOT NULL DEFAULT '' COMMENT '电池类型（PbAc=铅酸 / Li-ion=锂电...）；空串表示无数据',
  `input_voltage`   DECIMAL(6,1) NOT NULL DEFAULT -1     COMMENT '输入电压 V；-1 表示无数据',
  `output_voltage`  DECIMAL(6,1) NOT NULL DEFAULT -1     COMMENT '输出电压 V；-1 表示无数据',
  `load_percent`    TINYINT      NOT NULL DEFAULT -1     COMMENT '负载百分比；-1 表示无数据',
  `real_power`      SMALLINT     NOT NULL DEFAULT -1     COMMENT '实时功率 W（回退 ups.power）；-1 表示无数据',
  `raw_status`      VARCHAR(64)  NOT NULL DEFAULT ''     COMMENT 'NUT ups.status 原文（OL/OB/LB/CHRG ...）',
  `sampled_at`      DATETIME(3)  NOT NULL                COMMENT '采样时刻',
  PRIMARY KEY (`id`),
  KEY `idx_host_ups_time` (`host_kind`, `host_id`, `ups_name`, `sampled_at` DESC),
  KEY `idx_sampled_at` (`sampled_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='UPS 采样时间序列';

DROP TABLE IF EXISTS `ups_state`;
CREATE TABLE `ups_state` (
  `host_kind`            VARCHAR(16)  NOT NULL                COMMENT '固定 ups（保留列名，便于将来扩展）',
  `host_id`              BIGINT       NOT NULL                COMMENT 'ups_host.id',
  `ups_name`             VARCHAR(64)  NOT NULL                COMMENT 'NUT upsname',
  `host_name`            VARCHAR(64)  NOT NULL DEFAULT ''     COMMENT '冗余主机名',
  `mfr`                  VARCHAR(128) NOT NULL DEFAULT ''     COMMENT '品牌',
  `model`                VARCHAR(128) NOT NULL DEFAULT ''     COMMENT '型号',
  `last_power_source`    VARCHAR(16)  NOT NULL DEFAULT 'unknown' COMMENT '最近一次供电状态',
  `last_battery_percent` TINYINT      NOT NULL DEFAULT -1     COMMENT '最近一次电量',
  `last_runtime_minutes` INT          NOT NULL DEFAULT -1     COMMENT '最近一次续航分钟',
  `last_battery_voltage`         DECIMAL(5,1) NOT NULL DEFAULT -1 COMMENT '最近一次电池端电压',
  `last_battery_nominal_voltage` DECIMAL(5,1) NOT NULL DEFAULT -1 COMMENT '最近一次电池标称电压',
  `last_battery_type`            VARCHAR(16)  NOT NULL DEFAULT '' COMMENT '最近一次电池类型',
  `last_input_voltage`   DECIMAL(6,1) NOT NULL DEFAULT -1     COMMENT '最近一次输入电压',
  `last_output_voltage`  DECIMAL(6,1) NOT NULL DEFAULT -1     COMMENT '最近一次输出电压',
  `last_load_percent`    TINYINT      NOT NULL DEFAULT -1     COMMENT '最近一次负载百分比',
  `last_real_power`      SMALLINT     NOT NULL DEFAULT -1     COMMENT '最近一次实时功率 W',
  `last_raw_status`      VARCHAR(64)  NOT NULL DEFAULT ''     COMMENT '最近一次 ups.status 原文',
  `last_alert_at`        DATETIME(3)  DEFAULT NULL            COMMENT '最近一次告警时刻（去抖留痕）',
  `updated_at`           DATETIME(3)  NOT NULL                COMMENT '本行最近一次写入时刻',
  PRIMARY KEY (`host_kind`, `host_id`, `ups_name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='UPS 最新状态快照（每台 host × UPS 一行）';

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
  `name`        VARCHAR(64)  NOT NULL                COMMENT '凭证名称，前端下拉显示',
  `username`    VARCHAR(128) NOT NULL DEFAULT ''     COMMENT '登录用户名',
  `auth_type`   VARCHAR(16)  NOT NULL DEFAULT 'password' COMMENT 'password | key',
  `password`    TEXT         NULL                    COMMENT 'password 模式登录密码',
  `private_key` TEXT         NULL                    COMMENT 'key 模式 OpenSSH 私钥',
  `passphrase`  TEXT         NULL                    COMMENT 'key 模式私钥口令（可选）',
  `created_at`  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at`  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_name` (`name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='ESXi 模块专用 SSH 凭证';

DROP TABLE IF EXISTS `esxi_host`;
CREATE TABLE `esxi_host` (
  `id`          BIGINT       NOT NULL AUTO_INCREMENT,
  `name`        VARCHAR(64)  NOT NULL                COMMENT '机器名称',
  `endpoint`    VARCHAR(512) NOT NULL DEFAULT ''     COMMENT 'host:port（ESXi SSH 入口，默认 22）',
  `auth_json`   TEXT         NOT NULL                COMMENT 'JSON: {"auth_source":"inline|credential","credential_id":N,"username":...,"auth_type":"password|key","password":...,"private_key":...,"passphrase":...}',
  `config_json` TEXT         NOT NULL                COMMENT 'JSON: {"bastion_id":N} 或 {}',
  `enabled`     VARCHAR(1)   NOT NULL DEFAULT '1'    COMMENT '是否启用：1/0',
  `created_at`  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at`  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_name` (`name`),
  KEY `idx_enabled` (`enabled`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='ESXi 监控目标机器';

DROP TABLE IF EXISTS `esxi_sample`;
CREATE TABLE `esxi_sample` (
  `id`                    BIGINT       NOT NULL AUTO_INCREMENT,
  `host_kind`             VARCHAR(16)  NOT NULL                COMMENT '固定 esxi（保留列名，便于将来扩展）',
  `host_id`               BIGINT       NOT NULL                COMMENT 'esxi_host.id',
  `host_name`             VARCHAR(64)  NOT NULL DEFAULT ''     COMMENT '冗余主机名',
  `cpu_max_c`             SMALLINT     NOT NULL DEFAULT -1     COMMENT 'CPU 各核最高温；-1 表示无数据',
  `cpu_avg_c`             SMALLINT     NOT NULL DEFAULT -1     COMMENT 'CPU 平均温度；-1 表示无数据',
  `cpu_tjmax_c`           SMALLINT     NOT NULL DEFAULT -1     COMMENT 'CPU TjMax 节流阈值',
  `mce_state`             VARCHAR(16)  NOT NULL DEFAULT ''     COMMENT 'MCE 健康状态：Green / Yellow / Red',
  `mce_corrected_total`   BIGINT       NOT NULL DEFAULT 0      COMMENT 'MCE 累计可纠正错误数',
  `mce_uncorrected_total` BIGINT       NOT NULL DEFAULT 0      COMMENT 'MCE 累计不可纠正错误数',
  `disk_max_c`            SMALLINT     NOT NULL DEFAULT -1     COMMENT '所有盘里最热的那块温度；-1 表示无数据',
  `vm_total`              SMALLINT     NOT NULL DEFAULT -1     COMMENT 'VM 总数；-1 表示无数据',
  `vm_powered_on`         SMALLINT     NOT NULL DEFAULT -1     COMMENT '已开机 VM 数；-1 表示无数据',
  `cpu_temp_json`         TEXT         NULL                    COMMENT '每核温度明细 JSON：[{"id":0,"temp_c":54},...]；历史曲线"每核一条线"用',
  `disk_temp_json`        TEXT         NULL                    COMMENT '每盘温度明细 JSON：[{"device":"t10.XXX","temp_c":35},...]；历史曲线"每盘一条线"用',
  `sampled_at`            DATETIME(3)  NOT NULL                COMMENT '采样时刻',
  PRIMARY KEY (`id`),
  KEY `idx_host_time` (`host_kind`, `host_id`, `sampled_at` DESC),
  KEY `idx_sampled_at` (`sampled_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='ESXi 标量趋势时间序列';

DROP TABLE IF EXISTS `esxi_state`;
CREATE TABLE `esxi_state` (
  `host_kind`        VARCHAR(16)  NOT NULL                COMMENT '固定 esxi',
  `host_id`          BIGINT       NOT NULL                COMMENT 'esxi_host.id',
  `host_name`        VARCHAR(64)  NOT NULL DEFAULT ''     COMMENT '冗余主机名',
  `reachable`        VARCHAR(1)   NOT NULL DEFAULT '0'    COMMENT '最近一轮是否拿到数据：1/0',
  `last_error`       VARCHAR(512) NOT NULL DEFAULT ''     COMMENT '最近一轮失败原因（成功时清空）',
  `platform_json`    TEXT         NULL                    COMMENT 'JSON：主机标识 + ESXi 版本',
  `cpu_static_json`  TEXT         NULL                    COMMENT 'JSON：CPU 静态信息（brand/family/cores/freq/...）',
  `memory_json`      TEXT         NULL                    COMMENT 'JSON：内存信息（total/reliable/numa）',
  `runtime_json`     TEXT         NULL                    COMMENT 'JSON：CPU/内存运行时使用率',
  `cpu_temp_json`    TEXT         NULL                    COMMENT 'JSON：{ tjmax_c, cores:[{id,temp_c,headroom_c}], max_c, avg_c }',
  `mce_json`         TEXT         NULL                    COMMENT 'JSON：{ state, corrected_total, corrected_ewma, uncorrected_total, period_seconds }',
  `disk_json`        TEXT         NULL                    COMMENT 'JSON：[{device, model, type, temp_c, threshold_c, status}]',
  `usb_json`         TEXT         NULL                    COMMENT 'JSON：{ controllers, arbitrator_running, available_for_passthrough, vm_owned }',
  `vm_json`          TEXT         NULL                    COMMENT 'JSON：[{id, name, guest_os, state}]',
  `boot_json`        TEXT         NULL                    COMMENT 'JSON：{ uptime_seconds, booted_at, crash_dump_count, last_crash_at }',
  `nic_json`         TEXT         NULL                    COMMENT 'JSON：[{name, driver, mac, link_status, speed_mbps, rx_bytes, tx_bytes, rx_errors, tx_errors, ...}]',
  `topology_json`    TEXT         NULL                    COMMENT 'JSON：{ vswitches:[{name,uplinks,portgroups}], vm_nics:[{vmid,vm_name,vswitch,portgroup,mac,ip,team_uplink}] }',
  `last_alert_at`    DATETIME(3)  DEFAULT NULL            COMMENT '最近一次告警时刻（去抖留痕）',
  `sampled_at`       DATETIME(3)  DEFAULT NULL            COMMENT '最近一次成功采样时刻',
  `updated_at`       DATETIME(3)  NOT NULL                COMMENT '本行最近一次写入时刻',
  PRIMARY KEY (`host_kind`, `host_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='ESXi 最新状态快照（每台 host 一行）';

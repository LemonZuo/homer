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
  UNIQUE KEY `uk_main_domain` (`main_domain`),
  KEY `idx_account_id` (`account_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='ACME 自动签发域名';

DROP TABLE IF EXISTS `acme_safeline_target`;
DROP TABLE IF EXISTS `acme_safeline_deploy_config`;
DROP TABLE IF EXISTS `acme_ssh_target`;
DROP TABLE IF EXISTS `acme_ssh_deploy_config`;

DROP TABLE IF EXISTS `acme_deploy_target`;
CREATE TABLE `acme_deploy_target` (
  `id`          BIGINT       NOT NULL AUTO_INCREMENT,
  `name`        VARCHAR(64)  NOT NULL                COMMENT '目标名称',
  `kind`        VARCHAR(32)  NOT NULL                COMMENT 'ssh | safeline | ...',
  `endpoint`    VARCHAR(512) NOT NULL DEFAULT ''     COMMENT '目标地址；ssh 为 host:port，雷池为管理端根地址',
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
  `kind`        VARCHAR(32) NOT NULL                COMMENT 'ssh | safeline | ...',
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
  `cas_cert_id`    BIGINT     NOT NULL DEFAULT 0 COMMENT '上传到阿里云 CAS 后的 cert_id；0=未上传',
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
  `kind`         VARCHAR(32)  NOT NULL COMMENT 'issue | renew | revoke | upload_cas | deploy_ssh | deploy_safeline | deploy',
  `status`       VARCHAR(16)  NOT NULL COMMENT 'pending | running | success | failed',
  `started_at`   DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `finished_at`  DATETIME     NULL,
  `log_text`     MEDIUMTEXT   NOT NULL,
  `error_msg`    VARCHAR(1024) NOT NULL DEFAULT '',
  PRIMARY KEY (`id`),
  KEY `idx_domain_started` (`domain_id`, `started_at`),
  KEY `idx_status` (`status`)
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
-- module 取值：birthday | event | bypass | scheduler_alert，由代码常量约束。

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
  `module`     VARCHAR(32) NOT NULL                COMMENT 'birthday | event | bypass | scheduler_alert',
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

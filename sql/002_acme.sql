-- ACME 自动签发模块。
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
  `enabled`      VARCHAR(1)   NOT NULL DEFAULT '1'    COMMENT '是否启用自动续期：1/0',
  `created_at`   DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at`   DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_main_domain` (`main_domain`),
  KEY `idx_account_id` (`account_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='ACME 自动签发域名';

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
  `kind`         VARCHAR(16)  NOT NULL COMMENT 'issue | renew | revoke | upload_cas',
  `status`       VARCHAR(16)  NOT NULL COMMENT 'pending | running | success | failed',
  `started_at`   DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `finished_at`  DATETIME     NULL,
  `log_text`     MEDIUMTEXT   NOT NULL,
  `error_msg`    VARCHAR(1024) NOT NULL DEFAULT '',
  PRIMARY KEY (`id`),
  KEY `idx_domain_started` (`domain_id`, `started_at`),
  KEY `idx_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='ACME 签发/续期任务流水';

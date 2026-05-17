-- 拆解：CAS 从 ACME 内置流程改为通用部署 driver（kind='upload_cas'）。
-- 老表里的 acme_domain.cas_enabled / acme_cert.cas_cert_id 不再使用；
-- 升级后请在前端「部署目标 → 阿里云 CAS」配置一份目标，
-- 然后在域名的部署配置里挂上即可，原 cas_enabled 不会被自动迁移。
--
-- 用 procedure 做幂等：旧库有列时删除，新装库无列时跳过。

DROP PROCEDURE IF EXISTS homer_acme_cas_decouple;
DELIMITER //
CREATE PROCEDURE homer_acme_cas_decouple()
BEGIN
  IF EXISTS (SELECT 1 FROM INFORMATION_SCHEMA.COLUMNS
             WHERE TABLE_SCHEMA = DATABASE()
               AND TABLE_NAME = 'acme_domain'
               AND COLUMN_NAME = 'cas_enabled') THEN
    ALTER TABLE `acme_domain` DROP COLUMN `cas_enabled`;
  END IF;
  IF EXISTS (SELECT 1 FROM INFORMATION_SCHEMA.COLUMNS
             WHERE TABLE_SCHEMA = DATABASE()
               AND TABLE_NAME = 'acme_cert'
               AND COLUMN_NAME = 'cas_cert_id') THEN
    ALTER TABLE `acme_cert` DROP COLUMN `cas_cert_id`;
  END IF;
END //
DELIMITER ;
CALL homer_acme_cas_decouple();
DROP PROCEDURE IF EXISTS homer_acme_cas_decouple;

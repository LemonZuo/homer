-- 去掉 acme_domain.main_domain 唯一约束，改为普通索引。
-- 场景：同一主域名可能要并存多张证书（比如一个独立 *.lib.do + 一个把 *.lib.do
-- 跟其它 wildcard 打包到一起的合并证书）。
--
-- 用 procedure 兼容三种历史状态：
--   1) 旧库：只有 SQL 脚本建的 uk_main_domain
--   2) AutoMigrate 跑过：额外多了 idx_acme_domain_main_domain（GORM 默认命名）
--   3) 全新库：可能已经是新 schema，什么都不用做

DROP PROCEDURE IF EXISTS homer_drop_main_domain_unique;
DELIMITER //
CREATE PROCEDURE homer_drop_main_domain_unique()
BEGIN
  IF EXISTS (SELECT 1 FROM INFORMATION_SCHEMA.STATISTICS
             WHERE TABLE_SCHEMA = DATABASE()
               AND TABLE_NAME = 'acme_domain'
               AND INDEX_NAME = 'uk_main_domain') THEN
    ALTER TABLE `acme_domain` DROP INDEX `uk_main_domain`;
  END IF;
  IF EXISTS (SELECT 1 FROM INFORMATION_SCHEMA.STATISTICS
             WHERE TABLE_SCHEMA = DATABASE()
               AND TABLE_NAME = 'acme_domain'
               AND INDEX_NAME = 'idx_acme_domain_main_domain') THEN
    ALTER TABLE `acme_domain` DROP INDEX `idx_acme_domain_main_domain`;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM INFORMATION_SCHEMA.STATISTICS
                 WHERE TABLE_SCHEMA = DATABASE()
                   AND TABLE_NAME = 'acme_domain'
                   AND INDEX_NAME = 'idx_main_domain') THEN
    ALTER TABLE `acme_domain` ADD INDEX `idx_main_domain` (`main_domain`);
  END IF;
END //
DELIMITER ;
CALL homer_drop_main_domain_unique();
DROP PROCEDURE IF EXISTS homer_drop_main_domain_unique;

-- 业务开关独立于各模块的设备 / 域名 enabled。已存在的配置不覆盖。
DROP PROCEDURE IF EXISTS homer_module_settings;
DELIMITER //
CREATE PROCEDURE homer_module_settings()
BEGIN
  CREATE TABLE IF NOT EXISTS `module_setting` (
    `module_key` VARCHAR(32) NOT NULL,
    `enabled` TINYINT(1) NOT NULL DEFAULT 0,
    `version` BIGINT UNSIGNED NOT NULL DEFAULT 1,
    `updated_at` DATETIME(3) NULL,
    PRIMARY KEY (`module_key`)
  );
  INSERT IGNORE INTO `module_setting` (`module_key`, `enabled`, `version`, `updated_at`) VALUES
    ('ups', 1, 1, NOW(3)), ('esxi', 1, 1, NOW(3)), ('acme', 1, 1, NOW(3)),
    ('certstore', 1, 1, NOW(3)), ('cdnops', 1, 1, NOW(3)), ('birthday', 1, 1, NOW(3)),
    ('event', 1, 1, NOW(3)), ('sms', 1, 1, NOW(3)), ('bypass', 1, 1, NOW(3));
END //
DELIMITER ;
CALL homer_module_settings;
DROP PROCEDURE IF EXISTS homer_module_settings;

-- 把三家 config_json 里的 bastion 外键 JSON 字段统一改名为 bastion_id：
--   acme_deploy_target.config_json: bastion_target_id -> bastion_id
--   ups_host.config_json          : bastion_host_id   -> bastion_id
--   esxi_host.config_json         : bastion_host_id   -> bastion_id
--
-- 背景：原本三家各自命名（ACME 沿用 deploy_target 语义；UPS/ESXi 用 host）。
--       sshlike 包统一改造后 sshlike.TargetConfig 只认 bastion_id，需要数据先迁。
--
-- 幂等性：WHERE 兼顾两种情况：
--   1) 老字段在、新字段不在 → 重命名
--   2) 新字段已存在(可能是后续插入的新行) → 跳过，避免覆盖
-- 同一脚本反复执行结果一致。

-- ACME
UPDATE `acme_deploy_target`
SET `config_json` = JSON_SET(
        JSON_REMOVE(IFNULL(`config_json`, '{}'), '$.bastion_target_id'),
        '$.bastion_id',
        CAST(JSON_EXTRACT(`config_json`, '$.bastion_target_id') AS UNSIGNED)
    )
WHERE JSON_EXTRACT(`config_json`, '$.bastion_target_id') IS NOT NULL
  AND JSON_EXTRACT(`config_json`, '$.bastion_id') IS NULL;

-- UPS（表可能在更早的迁移里建出，必须存在再 UPDATE）
DROP PROCEDURE IF EXISTS homer_rename_ups_bastion_key;
DELIMITER //
CREATE PROCEDURE homer_rename_ups_bastion_key()
BEGIN
  IF EXISTS (SELECT 1 FROM INFORMATION_SCHEMA.TABLES
             WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'ups_host') THEN
    UPDATE `ups_host`
    SET `config_json` = JSON_SET(
            JSON_REMOVE(IFNULL(`config_json`, '{}'), '$.bastion_host_id'),
            '$.bastion_id',
            CAST(JSON_EXTRACT(`config_json`, '$.bastion_host_id') AS UNSIGNED)
        )
    WHERE JSON_EXTRACT(`config_json`, '$.bastion_host_id') IS NOT NULL
      AND JSON_EXTRACT(`config_json`, '$.bastion_id') IS NULL;
  END IF;
END //
DELIMITER ;
CALL homer_rename_ups_bastion_key();
DROP PROCEDURE IF EXISTS homer_rename_ups_bastion_key;

-- ESXi
DROP PROCEDURE IF EXISTS homer_rename_esxi_bastion_key;
DELIMITER //
CREATE PROCEDURE homer_rename_esxi_bastion_key()
BEGIN
  IF EXISTS (SELECT 1 FROM INFORMATION_SCHEMA.TABLES
             WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'esxi_host') THEN
    UPDATE `esxi_host`
    SET `config_json` = JSON_SET(
            JSON_REMOVE(IFNULL(`config_json`, '{}'), '$.bastion_host_id'),
            '$.bastion_id',
            CAST(JSON_EXTRACT(`config_json`, '$.bastion_host_id') AS UNSIGNED)
        )
    WHERE JSON_EXTRACT(`config_json`, '$.bastion_host_id') IS NOT NULL
      AND JSON_EXTRACT(`config_json`, '$.bastion_id') IS NULL;
  END IF;
END //
DELIMITER ;
CALL homer_rename_esxi_bastion_key();
DROP PROCEDURE IF EXISTS homer_rename_esxi_bastion_key;

-- 给 esxi_state 加 boot_json / nic_json 两列：
--   boot_json：主机启动信息 { uptime_seconds, booted_at, crash_dump_count, last_crash_at }
--   nic_json：物理网卡列表 [{name, driver, mac, link_status, speed_mbps, rx/tx 计数等}]
--
-- 幂等：先看列是否存在，缺了才 ADD。

DROP PROCEDURE IF EXISTS homer_add_esxi_state_boot_nic;
DELIMITER //
CREATE PROCEDURE homer_add_esxi_state_boot_nic()
BEGIN
  IF EXISTS (SELECT 1 FROM INFORMATION_SCHEMA.TABLES
             WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'esxi_state') THEN
    IF NOT EXISTS (SELECT 1 FROM INFORMATION_SCHEMA.COLUMNS
                   WHERE TABLE_SCHEMA = DATABASE()
                     AND TABLE_NAME = 'esxi_state'
                     AND COLUMN_NAME = 'boot_json') THEN
      ALTER TABLE `esxi_state`
        ADD COLUMN `boot_json` TEXT NULL
          COMMENT 'JSON：{ uptime_seconds, booted_at, crash_dump_count, last_crash_at }'
          AFTER `vm_json`;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM INFORMATION_SCHEMA.COLUMNS
                   WHERE TABLE_SCHEMA = DATABASE()
                     AND TABLE_NAME = 'esxi_state'
                     AND COLUMN_NAME = 'nic_json') THEN
      ALTER TABLE `esxi_state`
        ADD COLUMN `nic_json` TEXT NULL
          COMMENT 'JSON：[{name, driver, mac, link_status, speed_mbps, rx_bytes, tx_bytes, rx_errors, tx_errors, ...}]'
          AFTER `boot_json`;
    END IF;
  END IF;
END //
DELIMITER ;
CALL homer_add_esxi_state_boot_nic();
DROP PROCEDURE IF EXISTS homer_add_esxi_state_boot_nic;

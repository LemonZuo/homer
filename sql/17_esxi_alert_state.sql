-- ESXi state 补阈值告警状态 JSON,用于连续超阈值计数与已通知去重。

DROP PROCEDURE IF EXISTS homer_esxi_state_add_alert_state;
DELIMITER //
CREATE PROCEDURE homer_esxi_state_add_alert_state()
BEGIN
  IF NOT EXISTS (SELECT 1 FROM INFORMATION_SCHEMA.COLUMNS
                 WHERE TABLE_SCHEMA = DATABASE()
                   AND TABLE_NAME = 'esxi_state'
                   AND COLUMN_NAME = 'alert_state_json') THEN
    ALTER TABLE `esxi_state`
      ADD COLUMN `alert_state_json` TEXT NULL COMMENT 'JSON: 阈值告警连续计数与已通知状态' AFTER `topology_json`;
  END IF;
END //
DELIMITER ;
CALL homer_esxi_state_add_alert_state;
DROP PROCEDURE IF EXISTS homer_esxi_state_add_alert_state;

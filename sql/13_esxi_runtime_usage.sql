-- ESXi state 补运行时资源使用率 JSON,用于 CPU/内存使用率告警去抖。

DROP PROCEDURE IF EXISTS homer_esxi_state_add_runtime;
DELIMITER //
CREATE PROCEDURE homer_esxi_state_add_runtime()
BEGIN
  IF NOT EXISTS (SELECT 1 FROM INFORMATION_SCHEMA.COLUMNS
                 WHERE TABLE_SCHEMA = DATABASE()
                   AND TABLE_NAME = 'esxi_state'
                   AND COLUMN_NAME = 'runtime_json') THEN
    ALTER TABLE `esxi_state`
      ADD COLUMN `runtime_json` TEXT NULL COMMENT '运行时使用率' AFTER `memory_json`;
  END IF;
END //
DELIMITER ;
CALL homer_esxi_state_add_runtime;
DROP PROCEDURE IF EXISTS homer_esxi_state_add_runtime;

-- ESXi 采样表补两列明细 JSON,给"每核 / 每盘单独画历史曲线"用。
-- 旧行的两列落 NULL(TEXT 默认就是 NULL),前端碰到 NULL/空串当"无明细"处理。

DROP PROCEDURE IF EXISTS homer_esxi_sample_add_detail;
DELIMITER //
CREATE PROCEDURE homer_esxi_sample_add_detail()
BEGIN
  IF NOT EXISTS (SELECT 1 FROM INFORMATION_SCHEMA.COLUMNS
                 WHERE TABLE_SCHEMA = DATABASE()
                   AND TABLE_NAME = 'esxi_sample'
                   AND COLUMN_NAME = 'cpu_temp_json') THEN
    ALTER TABLE `esxi_sample`
      ADD COLUMN `cpu_temp_json`  TEXT NULL COMMENT '每核温度明细'  AFTER `vm_powered_on`,
      ADD COLUMN `disk_temp_json` TEXT NULL COMMENT '每盘温度明细' AFTER `cpu_temp_json`;
  END IF;
END //
DELIMITER ;
CALL homer_esxi_sample_add_detail;
DROP PROCEDURE IF EXISTS homer_esxi_sample_add_detail;

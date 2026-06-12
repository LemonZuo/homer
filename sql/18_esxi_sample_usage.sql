-- ESXi 采样表补 3 个使用率/容量明细列，给历史曲线画「CPU 使用率 / 内存使用率 / 各盘已用」用：
--   cpu_usage_percent     SMALLINT  -- -1=无数据
--   memory_usage_percent  SMALLINT  -- -1=无数据
--   disk_usage_json       TEXT      -- [{"device":"naa.xxx","used_bytes":N,"capacity_bytes":N}, ...]
-- 旧行三列分别用 -1 / -1 / NULL 兜底,前端按已有约定当「无数据」处理。

DROP PROCEDURE IF EXISTS homer_esxi_sample_add_usage;
DELIMITER //
CREATE PROCEDURE homer_esxi_sample_add_usage()
BEGIN
  IF NOT EXISTS (SELECT 1 FROM INFORMATION_SCHEMA.COLUMNS
                 WHERE TABLE_SCHEMA = DATABASE()
                   AND TABLE_NAME = 'esxi_sample'
                   AND COLUMN_NAME = 'cpu_usage_percent') THEN
    ALTER TABLE `esxi_sample`
      ADD COLUMN `cpu_usage_percent`    SMALLINT NOT NULL DEFAULT -1 COMMENT 'CPU 使用率' AFTER `disk_max_c`,
      ADD COLUMN `memory_usage_percent` SMALLINT NOT NULL DEFAULT -1 COMMENT '内存使用率' AFTER `cpu_usage_percent`;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM INFORMATION_SCHEMA.COLUMNS
                 WHERE TABLE_SCHEMA = DATABASE()
                   AND TABLE_NAME = 'esxi_sample'
                   AND COLUMN_NAME = 'disk_usage_json') THEN
    ALTER TABLE `esxi_sample`
      ADD COLUMN `disk_usage_json` TEXT NULL COMMENT '每盘容量明细' AFTER `disk_temp_json`;
  END IF;
END //
DELIMITER ;
CALL homer_esxi_sample_add_usage;
DROP PROCEDURE IF EXISTS homer_esxi_sample_add_usage;

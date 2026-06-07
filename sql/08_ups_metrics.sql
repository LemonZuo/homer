-- UPS 监控:补一组电气指标列(输入/输出电压、负载、实时功率)。
-- NUT 字段对应:input.voltage / output.voltage / ups.load / ups.realpower(回退 ups.power)。
-- 缺数据时存 -1(与 battery_percent 同样的"哨兵值"约定),方便前端区分"没值"与"为 0"。

DROP PROCEDURE IF EXISTS homer_ups_add_metrics;
DELIMITER //
CREATE PROCEDURE homer_ups_add_metrics()
BEGIN
  IF NOT EXISTS (SELECT 1 FROM INFORMATION_SCHEMA.COLUMNS
                 WHERE TABLE_SCHEMA = DATABASE()
                   AND TABLE_NAME = 'ups_sample'
                   AND COLUMN_NAME = 'input_voltage') THEN
    ALTER TABLE `ups_sample`
      ADD COLUMN `input_voltage`  DECIMAL(6,1) NOT NULL DEFAULT -1 AFTER `runtime_minutes`,
      ADD COLUMN `output_voltage` DECIMAL(6,1) NOT NULL DEFAULT -1 AFTER `input_voltage`,
      ADD COLUMN `load_percent`   TINYINT      NOT NULL DEFAULT -1 AFTER `output_voltage`,
      ADD COLUMN `real_power`     SMALLINT     NOT NULL DEFAULT -1 AFTER `load_percent`;
  END IF;

  IF NOT EXISTS (SELECT 1 FROM INFORMATION_SCHEMA.COLUMNS
                 WHERE TABLE_SCHEMA = DATABASE()
                   AND TABLE_NAME = 'ups_state'
                   AND COLUMN_NAME = 'last_input_voltage') THEN
    ALTER TABLE `ups_state`
      ADD COLUMN `last_input_voltage`  DECIMAL(6,1) NOT NULL DEFAULT -1 AFTER `last_runtime_minutes`,
      ADD COLUMN `last_output_voltage` DECIMAL(6,1) NOT NULL DEFAULT -1 AFTER `last_input_voltage`,
      ADD COLUMN `last_load_percent`   TINYINT      NOT NULL DEFAULT -1 AFTER `last_output_voltage`,
      ADD COLUMN `last_real_power`     SMALLINT     NOT NULL DEFAULT -1 AFTER `last_load_percent`;
  END IF;
END //
DELIMITER ;
CALL homer_ups_add_metrics();
DROP PROCEDURE IF EXISTS homer_ups_add_metrics;

-- UPS 监控:补电池电压 + 标称电压 + 电池类型三列。
-- 来源:NUT battery.voltage / battery.voltage.nominal / battery.type。
-- voltage 缺数据时 -1,与现有 input_voltage 同样的"哨兵值"约定;
-- type 缺数据时空串(常见值 PbAc=铅酸 / Li-ion=锂电)。

DROP PROCEDURE IF EXISTS homer_ups_add_battery_meta;
DELIMITER //
CREATE PROCEDURE homer_ups_add_battery_meta()
BEGIN
  IF NOT EXISTS (SELECT 1 FROM INFORMATION_SCHEMA.COLUMNS
                 WHERE TABLE_SCHEMA = DATABASE()
                   AND TABLE_NAME = 'ups_sample'
                   AND COLUMN_NAME = 'battery_voltage') THEN
    ALTER TABLE `ups_sample`
      ADD COLUMN `battery_voltage`         DECIMAL(5,1) NOT NULL DEFAULT -1 AFTER `runtime_minutes`,
      ADD COLUMN `battery_nominal_voltage` DECIMAL(5,1) NOT NULL DEFAULT -1 AFTER `battery_voltage`,
      ADD COLUMN `battery_type`            VARCHAR(16)  NOT NULL DEFAULT '' AFTER `battery_nominal_voltage`;
  END IF;

  IF NOT EXISTS (SELECT 1 FROM INFORMATION_SCHEMA.COLUMNS
                 WHERE TABLE_SCHEMA = DATABASE()
                   AND TABLE_NAME = 'ups_state'
                   AND COLUMN_NAME = 'last_battery_voltage') THEN
    ALTER TABLE `ups_state`
      ADD COLUMN `last_battery_voltage`         DECIMAL(5,1) NOT NULL DEFAULT -1 AFTER `last_runtime_minutes`,
      ADD COLUMN `last_battery_nominal_voltage` DECIMAL(5,1) NOT NULL DEFAULT -1 AFTER `last_battery_voltage`,
      ADD COLUMN `last_battery_type`            VARCHAR(16)  NOT NULL DEFAULT '' AFTER `last_battery_nominal_voltage`;
  END IF;
END //
DELIMITER ;
CALL homer_ups_add_battery_meta();
DROP PROCEDURE IF EXISTS homer_ups_add_battery_meta;

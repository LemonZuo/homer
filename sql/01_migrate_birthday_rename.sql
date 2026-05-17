-- 老库迁移：sys_birthday_remind → birthday_reminder
-- 适用于「老 ruoyi 表已存在于当前库」的场景：原地改表名 + 改字段名，数据零拷贝保留。
-- 已是全新部署（直接跑 00_schema.sql）则无需执行本文件。

RENAME TABLE `sys_birthday_remind` TO `birthday_reminder`;

ALTER TABLE `birthday_reminder`
  CHANGE COLUMN `remind_id`               `id`               BIGINT      NOT NULL AUTO_INCREMENT COMMENT '唯一标识',
  CHANGE COLUMN `remind_name`             `name`             VARCHAR(30) NOT NULL DEFAULT ''     COMMENT '姓名',
  CHANGE COLUMN `remind_birthday`         `birthday`         VARCHAR(10) NOT NULL DEFAULT ''     COMMENT '公历生日 yyyy-MM-dd',
  CHANGE COLUMN `remind_chinese_birthday` `chinese_birthday` VARCHAR(30) NOT NULL DEFAULT ''     COMMENT '农历生日（后端自动）',
  CHANGE COLUMN `remind_zodiac`           `zodiac`           VARCHAR(30) NOT NULL DEFAULT ''     COMMENT '生肖（后端自动）',
  CHANGE COLUMN `is_remind`               `enabled`          VARCHAR(1)  NOT NULL DEFAULT '1'    COMMENT '是否启用：1/0';

ALTER TABLE `birthday_reminder`
  DROP INDEX `idx_chinese_birthday`,
  ADD KEY `idx_chinese_birthday` (`chinese_birthday`, `enabled`);

-- 若老数据在另一个库（跨库迁移），改用下面的方式（先按 00_schema.sql 建好 birthday_reminder）：
-- INSERT INTO `birthday_reminder` (`id`,`name`,`birthday`,`chinese_birthday`,`zodiac`,`enabled`)
-- SELECT `remind_id`,`remind_name`,`remind_birthday`,`remind_chinese_birthday`,`remind_zodiac`,`is_remind`
-- FROM `老库名`.`sys_birthday_remind`;

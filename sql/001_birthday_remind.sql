-- 生日提醒
-- 复用老 ruoyi 项目的表 `sys_birthday_remind`，结构保持原样。
-- 字段语义：
--   remind_birthday          : 公历生日字符串 yyyy-MM-dd（用户输入）
--   remind_chinese_birthday  : 农历生日中文字符串，由后端 BeforeSave 自动算
--   remind_zodiac            : 生肖，由后端 BeforeSave 自动算
--   is_remind                : varchar('0'/'1'); Go 侧用 BoolFlag 自动转 bool
-- 全新部署时使用以下 DDL；老库已有数据时直接复用，无需建表。
CREATE TABLE IF NOT EXISTS `sys_birthday_remind` (
  `remind_id`               BIGINT       NOT NULL AUTO_INCREMENT COMMENT '唯一标识',
  `remind_name`             VARCHAR(30)  NOT NULL DEFAULT ''     COMMENT '姓名',
  `remind_birthday`         VARCHAR(10)  NOT NULL DEFAULT ''     COMMENT '公历生日 yyyy-MM-dd',
  `remind_chinese_birthday` VARCHAR(30)  NOT NULL DEFAULT ''     COMMENT '农历生日（后端自动）',
  `remind_zodiac`           VARCHAR(30)  NOT NULL DEFAULT ''     COMMENT '生肖（后端自动）',
  `is_remind`               VARCHAR(1)   NOT NULL DEFAULT '1'    COMMENT '是否提醒：1=是 0=否',
  PRIMARY KEY (`remind_id`),
  KEY `idx_chinese_birthday` (`remind_chinese_birthday`, `is_remind`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='生日提醒';

-- 老 Java（hutool getChineseMonth）把农历 1/11/12 月写成「一月 / 十一月 / 十二月」，
-- 统一成 lunar-go 写法「正月 / 冬月 / 腊月」（与新录入一致；后端解析两种写法都兼容）。
-- 纯数据 UPDATE，WHERE 只命中旧写法，重复执行无副作用。
UPDATE `birthday_reminder` SET `chinese_birthday` = CONCAT('冬月', SUBSTRING(`chinese_birthday`, 4))
  WHERE `chinese_birthday` LIKE '十一月%';
UPDATE `birthday_reminder` SET `chinese_birthday` = CONCAT('腊月', SUBSTRING(`chinese_birthday`, 4))
  WHERE `chinese_birthday` LIKE '十二月%';
UPDATE `birthday_reminder` SET `chinese_birthday` = CONCAT('正月', SUBSTRING(`chinese_birthday`, 3))
  WHERE `chinese_birthday` LIKE '一月%';

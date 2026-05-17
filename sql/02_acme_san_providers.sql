-- 增量：acme_domain 增加 san_providers（按域名指定 DNS provider 的覆盖表）。
-- 已有数据的环境执行本文件即可；全新部署跑 00_schema.sql 已含该列。

ALTER TABLE `acme_domain`
  ADD COLUMN `san_providers` VARCHAR(1024) NOT NULL DEFAULT '' COMMENT '按域名指定 provider 的覆盖表 JSON {"b.com":"alidns"}；空=全用 provider'
  AFTER `provider`;

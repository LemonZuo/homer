-- 增量：acme_domain 增加 cas_enabled（控制本域名是否参与阿里云 CAS）。
-- 开启后：签发/续期成功自动上传 CAS；手动「上传 CAS」按钮也只有开启时才可用。
-- 默认 '0'，表示存量域名升级后不会再自动上传，需在前端逐个勾选开启。

ALTER TABLE `acme_domain`
  ADD COLUMN `cas_enabled` VARCHAR(1) NOT NULL DEFAULT '0' COMMENT '是否参与阿里云 CAS'
  AFTER `san_providers`;

-- 回填 acme_domain.created_at 零值：v0.7.8 前编辑域名用 Save 整行写回，payload 不带 created_at → 被写成零日期。
-- 真实创建时间已丢失，取「最早的任务开始时间 / 证书签发时间 / updated_at」中最早者近似。
-- 只命中零值（< 1971，避免写零日期字面量触发 NO_ZERO_DATE），重复执行无副作用。
UPDATE `acme_domain` d
SET d.`created_at` = LEAST(
    COALESCE((SELECT MIN(t.`started_at`) FROM `acme_issue_task` t WHERE t.`domain_id` = d.`id` AND t.`started_at` >= '1971-01-01'), d.`updated_at`),
    COALESCE((SELECT c.`issued_at` FROM `acme_cert` c WHERE c.`domain_id` = d.`id` AND c.`issued_at` >= '1971-01-01' LIMIT 1), d.`updated_at`),
    d.`updated_at`)
WHERE d.`created_at` < '1971-01-01' AND d.`updated_at` >= '1971-01-01';

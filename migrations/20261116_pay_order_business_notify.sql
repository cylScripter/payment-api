ALTER TABLE `pay_order`
    ADD COLUMN `business_notify_status` VARCHAR(16) NOT NULL DEFAULT ''
        COMMENT '业务通知状态，空值未收到支付回调，PENDING待投递，SUCCESS已投递' AFTER `status`,
    ADD INDEX `idx_business_notify` (`status`, `business_notify_status`, `update_time`, `id`);

-- 回滚前先停止业务通知任务。
-- ALTER TABLE `pay_order` DROP INDEX `idx_business_notify`, DROP COLUMN `business_notify_status`;

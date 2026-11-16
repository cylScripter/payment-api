ALTER TABLE `pay_notify_log`
    ADD COLUMN `request_id` VARCHAR(128) NOT NULL DEFAULT '' COMMENT '请求ID' AFTER `id`;

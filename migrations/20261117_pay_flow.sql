-- 适用于旧版 init.sql 中的 pay_flow 表结构。
-- channel_biz_no、channel_biz_type 两列及其数据会被删除。
-- 原 bill_date 转为 trade_time 后，历史日期的时间部分为 00:00:00。
ALTER TABLE `pay_flow`
    DROP INDEX `idx_merchant_bill_date`,
    CHANGE COLUMN `channel_flow_no` `trade_no` VARCHAR(128) NOT NULL COMMENT '渠道流水号',
    ADD COLUMN `refund_trade_no` VARCHAR(128) NOT NULL DEFAULT '' COMMENT '渠道退款流水号' AFTER `trade_no`,
    DROP COLUMN `channel_biz_no`,
    DROP COLUMN `channel_biz_type`,
    MODIFY COLUMN `accounted_time` DATETIME(3) NOT NULL COMMENT '收付款成功时间',
    CHANGE COLUMN `bill_date` `trade_time` DATETIME(3) NOT NULL COMMENT '账单日期',
    ADD INDEX `idx_merchant_trade_time` (`channel`, `merchant_no`, `trade_time`);

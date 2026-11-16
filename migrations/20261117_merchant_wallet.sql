CREATE TABLE IF NOT EXISTS `merchant_wallet` (
    `id`          BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `create_time` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
    `update_time` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3)
        ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
    `account`     VARCHAR(128) NOT NULL COMMENT '商户账号',
    `merchant_id` BIGINT NOT NULL COMMENT '商户ID',
    `balance`     BIGINT NOT NULL DEFAULT 0 COMMENT '钱包余额，单位分',
    `version`     BIGINT NOT NULL DEFAULT 0 COMMENT '版本号',
    PRIMARY KEY (`id`),
    KEY `idx_wallet_account` (`account`)
) ENGINE=InnoDB
  DEFAULT CHARSET=utf8mb4
  COLLATE=utf8mb4_bin
  COMMENT='商户钱包';

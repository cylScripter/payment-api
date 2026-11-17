CREATE TABLE IF NOT EXISTS `pay_merchant_api_key` (
    `id`          BIGINT NOT NULL AUTO_INCREMENT,
    `merchant_id` BIGINT NOT NULL COMMENT '所属商户ID',
    `key_hash`    CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT 'API_KEY SHA-256 摘要',
    `create_time` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '签发时间',
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_key_hash` (`key_hash`),
    KEY `idx_merchant_id` (`merchant_id`)
) ENGINE=InnoDB
  DEFAULT CHARSET=utf8mb4
  COLLATE=utf8mb4_bin
  COMMENT='商户接口密钥';

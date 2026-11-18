CREATE TABLE `pay_flow` (
    `id`               BIGINT       NOT NULL AUTO_INCREMENT,
    `flow_no`          VARCHAR(64)  NOT NULL COMMENT '网关资金流水号',
    `pay_no`           VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '关联支付单号，可空',
    `refund_no`        VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '关联退款单号，可空',
    `merchant_account` VARCHAR(128) NOT NULL DEFAULT '' COMMENT '账户',
    `channel`          VARCHAR(32)  NOT NULL COMMENT '支付渠道',
    `merchant_no`      VARCHAR(64)  NOT NULL COMMENT '渠道商户号',
    `app_id`           VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '渠道应用ID',
    `open_id`          VARCHAR(128) NOT NULL DEFAULT '' COMMENT '用户在渠道的标识',
    `account_type`     VARCHAR(32)  NOT NULL COMMENT '渠道资金账户类型，微信为 BASIC、OPERATION、FEES',
    `trade_no`         VARCHAR(128) NOT NULL COMMENT '渠道流水号',
    `refund_trade_no`  VARCHAR(128) NOT NULL DEFAULT '' COMMENT '渠道退款流水号',
    `flow_type`        VARCHAR(32)  NOT NULL COMMENT '资金变动类型',
    `direction`        VARCHAR(16)  NOT NULL COMMENT '收支方向',
    `total_amount`     BIGINT       NOT NULL COMMENT '本次发生金额，正整数，单位分',
    `real_amount`      BIGINT       NOT NULL COMMENT '实际收付款金额，正整数，单位分',
    `currency`         VARCHAR(8)   NOT NULL COMMENT '币种',
    `accounted_time`   DATETIME(3)  NOT NULL COMMENT '收付款成功时间',
    `trade_time`       DATETIME(3)  NOT NULL COMMENT '账单日期',
    `remark`           VARCHAR(512) NOT NULL DEFAULT '' COMMENT '账单备注',
    `create_time`      DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '本地入库时间',
    `update_time`      DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3)
        ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_flow_no` (`flow_no`),
    UNIQUE KEY `uk_channel_flow` (`channel`, `merchant_no`, `account_type`, `trade_no`),
    KEY `idx_pay_no` (`pay_no`),
    KEY `idx_refund_no` (`refund_no`),
    KEY `idx_merchant_trade_time` (`channel`, `merchant_no`, `trade_time`)
) ENGINE=InnoDB
  DEFAULT CHARSET=utf8mb4
  COLLATE=utf8mb4_bin
  COMMENT='渠道账户资金流水';

CREATE TABLE `pay_merchant` (
    `id`          BIGINT        NOT NULL AUTO_INCREMENT,
    `create_time` DATETIME(3)   NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '本地入库时间',
    `update_time` DATETIME(3)   NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
    `name`        VARCHAR(128)  NOT NULL COMMENT '商户名称',
    `account`     VARCHAR(128)  NOT NULL COMMENT '商户账号',
    `type`        BIGINT        NOT NULL COMMENT '商户类型：1业务商户 2平台商户',
    `status`      BIGINT        NOT NULL COMMENT '状态：1正常 2禁用',
    `notify_url`  VARCHAR(2048) NOT NULL COMMENT '通知地址',
    `notify_secret` VARCHAR(512) NOT NULL DEFAULT '' COMMENT '商户回调密钥密文',
    `return_url`  VARCHAR(2048) NOT NULL COMMENT '回调地址',
    `remark`      VARCHAR(512)  DEFAULT NULL COMMENT '备注',
    PRIMARY KEY (`id`)
) ENGINE=InnoDB
  DEFAULT CHARSET=utf8mb4
  COLLATE=utf8mb4_bin
  COMMENT='支付商户';

CREATE TABLE `pay_merchant_channel` (
    `id`                  BIGINT       NOT NULL AUTO_INCREMENT,
    `create_time`         DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '本地入库时间',
    `update_time`         DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
    `merchant_id`         BIGINT       NOT NULL COMMENT '商户ID',
    `channel_code`        VARCHAR(32)  NOT NULL COMMENT '渠道编码，wechat/alipay',
    `channel_merchant_no` VARCHAR(64)  NOT NULL COMMENT '渠道商户号',
    `channel_app_id`      VARCHAR(64)  NOT NULL COMMENT '渠道AppID',
    `config`              LONGTEXT     NOT NULL COMMENT '配置信息',
    `status`              BIGINT       NOT NULL COMMENT '状态：1正常 2禁用',
    PRIMARY KEY (`id`)
) ENGINE=InnoDB
  DEFAULT CHARSET=utf8mb4
  COLLATE=utf8mb4_bin
  COMMENT='商户支付渠道配置';

CREATE TABLE `pay_order` (
    `id`               BIGINT        NOT NULL AUTO_INCREMENT,
    `pay_no`           VARCHAR(64)   NOT NULL COMMENT '支付单号',
    `biz_order_no`     VARCHAR(64)   DEFAULT NULL COMMENT '订单号',
    `biz_id`           VARCHAR(64)   DEFAULT NULL COMMENT '业务标识',
    `biz_type`         VARCHAR(32)   DEFAULT NULL COMMENT '业务类型',
    `total_amount`     BIGINT        DEFAULT NULL COMMENT '订单金额，单位分',
    `pay_amount`       BIGINT        DEFAULT NULL COMMENT '用户实付金额，单位分',
    `refunded_amount`  BIGINT        NOT NULL DEFAULT 0 COMMENT '累计退款成功金额，单位分，与total_amount同口径',
    `status`           VARCHAR(32)   NOT NULL DEFAULT 'PENDING' COMMENT '支付状态，退款后保持SUCCESS',
    `business_notify_status` VARCHAR(16) NOT NULL DEFAULT '' COMMENT '业务通知状态，空值未收到支付回调，PENDING待投递，SUCCESS已投递',
    `channel`          VARCHAR(32)   DEFAULT NULL COMMENT '支付渠道',
    `trade_no`         VARCHAR(128)  DEFAULT NULL COMMENT '渠道交易号',
    `currency`         VARCHAR(8)    DEFAULT 'CNY' COMMENT '货币类型',
    `client_ip`        VARCHAR(45)   DEFAULT NULL COMMENT '客户端IP',
    `expire_time`      DATETIME(3)   DEFAULT NULL COMMENT '订单过期时间',
    `notify_url`       VARCHAR(2048) DEFAULT NULL COMMENT '通知地址',
    `raw_request`      LONGTEXT      DEFAULT NULL COMMENT '原始请求',
    `raw_response`     LONGTEXT      DEFAULT NULL COMMENT '原始响应',
    `pay_time`         DATETIME(3)   DEFAULT NULL COMMENT '支付时间',
    `merchant_account` VARCHAR(128)  DEFAULT NULL COMMENT '账户',
    `merchant_no`      VARCHAR(64)   DEFAULT NULL COMMENT '商户订单号',
    `app_id`           VARCHAR(64)   DEFAULT NULL COMMENT '应用ID',
    `open_id`          VARCHAR(128)  DEFAULT NULL COMMENT '用户标识',
    `version`          INT           DEFAULT NULL COMMENT '版本号',
    `subject`          VARCHAR(512)  DEFAULT NULL COMMENT '商品描述',
    `create_time`      DATETIME(3)   NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '本地入库时间',
    `update_time`      DATETIME(3)   NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_pay_no` (`pay_no`),
    KEY `idx_business_notify` (`status`, `business_notify_status`, `update_time`, `id`)
) ENGINE=InnoDB
  DEFAULT CHARSET=utf8mb4
  COLLATE=utf8mb4_bin
  COMMENT='支付单';

CREATE TABLE `pay_notify_log` (
    `id`             BIGINT       NOT NULL AUTO_INCREMENT,
    `request_id`     VARCHAR(128) NOT NULL DEFAULT '' COMMENT '请求ID',
    `pay_no`         VARCHAR(64)  NOT NULL COMMENT '支付单号',
    `trade_no`       VARCHAR(128) NOT NULL COMMENT '渠道交易号',
    `channel`        VARCHAR(32)  NOT NULL COMMENT '支付渠道',
    `notify_type`    VARCHAR(64)  NOT NULL COMMENT '通知类型',
    `request_body`   LONGTEXT     NOT NULL COMMENT '请求内容',
    `verify_status`  VARCHAR(32)  NOT NULL COMMENT '验证状态',
    `process_status` VARCHAR(32)  NOT NULL COMMENT '处理状态',
    `err_message`    TEXT         DEFAULT NULL COMMENT '错误信息',
    `create_time`    DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '本地入库时间',
    `update_time`    DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
    PRIMARY KEY (`id`)
) ENGINE=InnoDB
  DEFAULT CHARSET=utf8mb4
  COLLATE=utf8mb4_bin
  COMMENT='支付回调通知日志';

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

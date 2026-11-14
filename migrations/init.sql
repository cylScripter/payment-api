CREATE TABLE `pay_flow` (
                            `id`               BIGINT       NOT NULL AUTO_INCREMENT,
                            `flow_no`          VARCHAR(64)  NOT NULL COMMENT '网关资金流水号',
                            `pay_no`           VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '关联支付单号，可空',
                            `refund_no`        VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '关联退款单号，可空',
                            `channel`          VARCHAR(32)  NOT NULL COMMENT '支付渠道',
                            `merchant_no`      VARCHAR(64)  NOT NULL COMMENT '渠道商户号',
                            `app_id`           VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '渠道应用ID',
                            `open_id`          VARCHAR(128) NOT NULL DEFAULT '' COMMENT '用户在渠道的标识',
                            `account_type`     VARCHAR(32)  NOT NULL COMMENT '渠道资金账户类型，微信为 BASIC、OPERATION、FEES',
                            `channel_flow_no`  VARCHAR(128) NOT NULL COMMENT '渠道流水号',
                            `channel_biz_no`   VARCHAR(128) NOT NULL DEFAULT '' COMMENT '渠道业务单号，用于关联渠道交易或退款',
                            `channel_biz_type` VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '渠道原始业务类型',
                            `flow_type`        VARCHAR(32)  NOT NULL COMMENT '资金变动类型',
                            `direction`        VARCHAR(16)  NOT NULL COMMENT '收支方向',
                            `total_amount`     BIGINT       NOT NULL COMMENT '本次发生金额，正整数，单位分',
                            `real_amount`      BIGINT       NOT NULL COMMENT '实际收付款金额，正整数，单位分',
                            `currency`         VARCHAR(8)   NOT NULL COMMENT '币种',
                            `merchant_account` VARCHAR(128) NOT NULL DEFAULT '' COMMENT '账户',
                            `accounted_time`   DATETIME(3)  NOT NULL COMMENT '渠道实际记账时间',
                            `bill_date`        DATE         NOT NULL COMMENT '所属渠道账单日期',
                            `remark`           VARCHAR(512) NOT NULL DEFAULT '' COMMENT '账单备注',
                            `create_time`      DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '本地入库时间',
                            `update_time`      DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3)
                                   ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
                            PRIMARY KEY (`id`),
                            UNIQUE KEY `uk_flow_no` (`flow_no`),
                            UNIQUE KEY `uk_channel_flow` (
                                `channel`, `merchant_no`, `account_type`, `channel_flow_no`
                                ),
                            KEY `idx_pay_no` (`pay_no`),
                            KEY `idx_refund_no` (`refund_no`),
                            KEY `idx_merchant_bill_date` (`channel`, `merchant_no`, `bill_date`)
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
    UNIQUE KEY `uk_pay_no` (`pay_no`)
) ENGINE=InnoDB
  DEFAULT CHARSET=utf8mb4
  COLLATE=utf8mb4_bin
  COMMENT='支付单';

CREATE TABLE `pay_notify_log` (
    `id`             BIGINT       NOT NULL AUTO_INCREMENT,
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

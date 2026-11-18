ALTER TABLE `pay_merchant`
    ADD COLUMN `notify_secret` VARCHAR(512) NOT NULL DEFAULT '' COMMENT '商户回调密钥密文' AFTER `notify_url`;

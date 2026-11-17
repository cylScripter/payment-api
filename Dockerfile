FROM --platform=linux/amd64 gcr.io/distroless/static-debian13

WORKDIR /app

ENV TZ=Asia/Shanghai \
    CONFIG_PATH=/app/configs/config.yaml

COPY bin/payment-api ./payment-api
COPY configs/config.yaml ./configs/config.yaml

EXPOSE 8080 9090

ENTRYPOINT ["/app/payment-api"]

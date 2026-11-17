#!/bin/bash

# 获取git 当前分支名 保存到变量中
branch=$(git symbolic-ref --short -q HEAD)

echo "当前分支: $branch"

# 检查是否提供了版本参数
if [ -z "$1" ]; then
  if [ "$branch" = "dev" ]; then
      VERSION=v$(date +%Y%m%d-%H%M%S)-dev
  elif [ "$branch" = "main" ]; then
      VERSION=v$(date +%Y%m%d-%H%M%S)-main
  else
      VERSION=v$(date +%Y%m%d-%H%M%S)-$(git rev-parse --short HEAD)
  fi
  echo "未提供版本号，自动生成版本号: ${VERSION}"
else
  VERSION="$1"
fi


PROJECT=open

if [ "$branch" = "dev" ]; then
  PROJECT=library
elif [ "$branch" = "main" ]; then
  PROJECT=open
else
  PROJECT=library
fi
echo "项目名: $PROJECT"


echo "开始构建 Docker 镜像..."
echo "版本号: $VERSION"
# 构建 Docker 镜像
docker build -t="payment-api:$VERSION" .

echo "镜像构建完成，开始推送镜像..."
# 登录到 Docker 仓库
docker login -u admin -p Cyl261405 http://114.132.181.123:50000

if [ $? -ne 0 ]; then
  echo "登录 Docker 仓库失败！"
  exit 1
fi
echo "登录 Docker 仓库成功！"
echo "开始标记镜像..."
# 标记 Docker 镜像以便推送到远程仓库
docker tag payment-api:$VERSION 114.132.181.123:50000/$PROJECT/payment-api:$VERSION

if [ $? -ne 0 ]; then
  echo "标记 Docker 镜像失败！"
  exit 1
fi
echo "标记 Docker 镜像成功！"
# 推送 Docker 镜像到远程仓库
echo  "docker push 114.132.181.123:50000/$PROJECT/payment-api:$VERSION"
docker push 114.132.181.123:50000/$PROJECT/payment-api:$VERSION

if [ $? -ne 0 ]; then
  echo "推送 Docker 镜像失败！"
  exit 1
fi

echo "镜像推送完成！"

echo "开始清理镜像..."
docker rmi payment-api:$VERSION
docker rmi 114.132.181.123:50000/$PROJECT/payment-api:$VERSION
if [ $? -ne 0 ]; then
  echo "清理镜像失败！"
  exit 1
fi

echo "Docker 镜像构建完成！"

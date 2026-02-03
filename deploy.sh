#!/bin/bash
# FunnyAI 后端一键部署脚本

set -e

SERVER="root@47.251.8.19"
REMOTE_PATH="/opt/funnyai-backend"
LOCAL_PATH="/Users/liangqianwei/claudeProjects/funnyai-backend"

echo "🚀 开始部署 FunnyAI 后端..."

# 1. 同步代码到服务器
echo "📦 同步代码..."
rsync -avz --exclude '.git' --exclude 'uploads' --exclude '*.log' \
  $LOCAL_PATH/ $SERVER:$REMOTE_PATH/

# 2. 远程编译和重启
echo "🔨 编译并重启服务..."
ssh $SERVER "cd $REMOTE_PATH && go build -o funnyai-server . && systemctl restart funnyai"

# 3. 等待服务启动
sleep 3

# 4. 健康检查
echo "🏥 健康检查..."
HEALTH=$(curl -s http://47.251.8.19:8080/health)
if [[ $HEALTH == *"ok"* ]]; then
  echo "✅ 部署成功！服务运行正常"
else
  echo "❌ 部署失败！请检查日志"
  ssh $SERVER "journalctl -u funnyai -n 20"
  exit 1
fi

echo "🎉 部署完成！"

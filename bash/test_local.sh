#!/bin/bash
# 本地测试脚本 - 通过 curl 发送消息到 Matterbridge API

# 默认使用 4242 端口（用于测试脚本发送消息）
API_URL="${API_URL:-http://127.0.0.1:4242}"
TOKEN="${TOKEN:-mytoken}"
GATEWAY="${GATEWAY:-mygateway}"

# 发送消息
send_message() {
    local text="$1"
    local username="${2:-Test User}"
    
    echo "发送消息: $text"
    
    curl -X POST "$API_URL/api/message" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer $TOKEN" \
        -d "{
            \"text\": \"$text\",
            \"username\": \"$username\",
            \"userid\": \"uid001\",
            \"gateway\": \"$GATEWAY\"
        }" \
        -s | jq '.' 2>/dev/null || echo "消息已发送"
    
    echo ""
}

# 获取消息
get_messages() {
    echo "获取消息..."
    curl -X GET "$API_URL/api/messages" \
        -H "Authorization: Bearer $TOKEN" \
        -s | jq '.' 2>/dev/null || echo "无消息"
    echo ""
}

# 健康检查
health_check() {
    echo "健康检查..."
    curl -X GET "$API_URL/api/health" -s
    echo ""
}

# 主菜单
case "${1:-send}" in
    send)
        send_message "${2:-Hello, this is a test message!}" "${3:-Test User}"
        ;;
    get)
        get_messages
        ;;
    health)
        health_check
        ;;
    *)
        echo "用法: $0 [send|get|health] [message] [username]"
        echo ""
        echo "示例:"
        echo "  $0 send 'Hello, world!' 'Test User'  # 发送消息"
        echo "  $0 get                                 # 获取消息"
        echo "  $0 health                             # 健康检查"
        echo ""
        echo "环境变量:"
        echo "  API_URL=$API_URL"
        echo "  TOKEN=$TOKEN"
        echo "  GATEWAY=$GATEWAY"
        ;;
esac

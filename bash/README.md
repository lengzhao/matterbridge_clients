# Bash Client

基于 Bash 脚本的 Matterbridge API 客户端，用于快速测试和发送消息到 Matterbridge。

## 功能特性

- ✅ 发送消息到 Matterbridge API
- ✅ 获取消息队列
- ✅ 健康检查
- ✅ 支持环境变量配置
- ✅ 简单易用的命令行接口

## 使用方法

### 基本使用

```bash
# 发送消息（默认）
./test_local.sh send "Hello, world!" "Test User"

# 获取消息
./test_local.sh get

# 健康检查
./test_local.sh health
```

### 环境变量配置

可以通过环境变量配置 API 连接参数：

```bash
export API_URL="http://127.0.0.1:4242"
export TOKEN="your-token"
export GATEWAY="mygateway"

./test_local.sh send "Hello, world!"
```

### 参数说明

| 环境变量 | 说明 | 默认值 |
|---------|------|--------|
| `API_URL` | Matterbridge API URL | `http://127.0.0.1:4242` |
| `TOKEN` | API 认证 Token | `mytoken` |
| `GATEWAY` | Gateway 名称 | `mygateway` |

### 命令说明

| 命令 | 说明 | 示例 |
|------|------|------|
| `send` | 发送消息 | `./test_local.sh send "Hello" "User"` |
| `get` | 获取消息队列 | `./test_local.sh get` |
| `health` | 健康检查 | `./test_local.sh health` |

## 使用示例

### 1. 发送消息

```bash
# 使用默认参数
./test_local.sh send "Hello, this is a test message!"

# 指定用户名
./test_local.sh send "Hello, world!" "My Bot"

# 使用自定义环境变量
API_URL="http://localhost:4242" TOKEN="mytoken" GATEWAY="mygateway" \
  ./test_local.sh send "Custom message"
```

### 2. 获取消息

```bash
# 获取所有消息
./test_local.sh get

# 使用自定义 Token
TOKEN="your-token" ./test_local.sh get
```

### 3. 健康检查

```bash
# 检查 API 服务状态
./test_local.sh health
```

## 前置要求

- `curl` - HTTP 客户端
- `jq` - JSON 处理工具（可选，用于美化输出）

### 安装依赖

**macOS:**
```bash
brew install curl jq
```

**Linux (Ubuntu/Debian):**
```bash
sudo apt-get install curl jq
```

## Matterbridge 配置

### 使用提供的配置文件

本目录下提供了 `matterbridge.toml` 配置文件，可以直接使用：

```bash
# 启动 Matterbridge（使用本目录的配置文件）
matterbridge -conf bash/matterbridge.toml

# 或者从项目根目录启动
cd /path/to/matterbridge_clients
matterbridge -conf bash/matterbridge.toml
```

### 配置文件说明

配置文件 `matterbridge.toml` 包含以下设置：

```toml
[api.local]
BindAddress="127.0.0.1:4242"
Token="mytoken"
Buffer=1000

[[gateway]]
name="mygateway"
enable=true
    [[gateway.inout]]
    account="api.local"
    channel="api"
```

### 自定义配置

如果需要修改配置，可以编辑 `matterbridge.toml` 文件，或创建自己的配置文件。确保配置中的以下参数与脚本默认值匹配：

- `BindAddress`: API 监听地址（默认: `127.0.0.1:4242`）
- `Token`: API 认证 Token（默认: `mytoken`）
- `name`: Gateway 名称（默认: `mygateway`）

如果修改了这些参数，记得相应地设置环境变量或修改脚本中的默认值。

## 故障排查

### 连接失败

- 检查 Matterbridge 是否运行：`curl http://127.0.0.1:4242/api/health`
- 检查 API URL 和端口是否正确
- 检查 Token 是否匹配 Matterbridge 配置

### 消息发送失败

- 检查 Gateway 名称是否匹配
- 检查 Token 是否正确
- 查看 Matterbridge 日志确认消息是否被接收

### jq 命令未找到

如果没有安装 `jq`，脚本仍可正常工作，但 JSON 输出不会被美化。

## 扩展功能

可以基于 `test_local.sh` 扩展更多功能：

- 文件上传
- 图片发送（Base64 编码）
- 批量消息发送
- 消息过滤和搜索
- WebSocket 连接支持

参考 [API文件传输和AI对接指南](../docs/API文件传输和AI对接指南.md) 了解更多 API 功能。

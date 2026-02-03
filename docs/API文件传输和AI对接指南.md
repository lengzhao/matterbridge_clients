# Matterbridge API 文件传输和 AI Agent 对接指南

## 1. API 文件/图片支持概述

**Matterbridge REST API 完全支持图片和文档的发送和接收！** ✅

### 1.1 支持的功能

- ✅ **发送图片**：通过 base64 编码发送图片
- ✅ **发送文档**：支持各种文件类型（PDF、DOC、ZIP 等）
- ✅ **接收图片/文档**：从其他协议接收的文件会包含在消息的 `extra` 字段中
- ✅ **文件元数据**：支持文件名、大小、注释等元数据
- ✅ **WebSocket 支持**：实时接收消息和文件
- ✅ **流式传输**：支持 Server-Sent Events (SSE) 流式接收

## 2. API 端点说明

### 2.1 端点列表

| 端点 | 方法 | 说明 |
|------|------|------|
| `/api/health` | GET | 健康检查 |
| `/api/messages` | GET | 获取消息队列（轮询方式） |
| `/api/stream` | GET | 流式接收消息（SSE） |
| `/api/websocket` | GET | WebSocket 实时通信 |
| `/api/message` | POST | 发送消息（支持文件） |

### 2.2 认证方式

如果配置了 `Token`，需要在请求头中添加：

```http
Authorization: Bearer your-token-here
```

**注意**：Matterbridge 使用 Echo 框架的 KeyAuth 中间件，期望 Bearer token 格式。

## 3. 配置文件设置

### 3.1 启用 API 桥接器

在 `matterbridge.toml` 中添加 API 配置：

```toml
[api.local]
BindAddress="127.0.0.1:4242"
Token="your-secret-token"  # 可选，用于认证
Buffer=50                   # 消息缓冲区大小（可选）

[[gateway]]
name="mygateway"
enable=true
    [[gateway.inout]]
    account="api.local"
    channel="api"
    
    # 其他协议的桥接配置
    [[gateway.inout]]
    account="telegram.mybot"
    channel="-123456789"
```

## 4. 发送消息和文件

### 4.1 发送纯文本消息

```bash
curl -X POST http://127.0.0.1:4242/api/message \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer your-secret-token" \
  -d '{
    "text": "Hello from API!",
    "username": "AI Bot",
    "gateway": "mygateway"
  }'
```

### 4.2 发送图片（Base64 编码）

```bash
# 首先将图片转换为 base64
IMAGE_BASE64=$(base64 -i image.jpg)

curl -X POST http://127.0.0.1:4242/api/message \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer your-secret-token" \
  -d '{
    "text": "这是一张图片",
    "username": "AI Bot",
    "gateway": "mygateway",
    "extra": {
      "file": [
        {
          "Name": "image.jpg",
          "Data": "'"$IMAGE_BASE64"'",
          "Comment": "图片说明文字",
          "Size": 123456
        }
      ]
    }
  }'
```

### 4.3 发送文档

```bash
# 将文档转换为 base64
DOC_BASE64=$(base64 -i document.pdf)

curl -X POST http://127.0.0.1:4242/api/message \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer your-secret-token" \
  -d '{
    "text": "请查看附件",
    "username": "AI Bot",
    "gateway": "mygateway",
    "extra": {
      "file": [
        {
          "Name": "document.pdf",
          "Data": "'"$DOC_BASE64"'",
          "Comment": "PDF 文档",
          "Size": 234567
        }
      ]
    }
  }'
```

### 4.4 发送多个文件

```json
{
  "text": "多个文件",
  "username": "AI Bot",
  "gateway": "mygateway",
  "extra": {
    "file": [
      {
        "Name": "image1.jpg",
        "Data": "base64-encoded-data-1",
        "Comment": "第一张图片",
        "Size": 123456
      },
      {
        "Name": "image2.png",
        "Data": "base64-encoded-data-2",
        "Comment": "第二张图片",
        "Size": 234567
      }
    ]
  }
}
```

### 4.5 FileInfo 字段说明

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `Name` | string | ✅ | 文件名（包含扩展名） |
| `Data` | string | ✅ | Base64 编码的文件内容 |
| `Comment` | string | ❌ | 文件说明/注释 |
| `Size` | int64 | ❌ | 文件大小（字节） |
| `URL` | string | ❌ | 文件 URL（如果文件已上传到服务器） |
| `Avatar` | bool | ❌ | 是否为头像图片 |
| `SHA` | string | ❌ | 文件 SHA 哈希值 |
| `NativeID` | string | ❌ | 原始协议的文件 ID |

## 5. 接收消息和文件

### 5.1 轮询方式（GET /api/messages）

```bash
curl -X GET http://127.0.0.1:4242/api/messages \
  -H "Authorization: Bearer your-secret-token"
```

**响应示例**：

```json
[
  {
    "text": "用户发送的消息",
    "username": "alice",
    "userid": "U123456",
    "account": "telegram.mybot",
    "protocol": "telegram",
    "channel": "-123456789",
    "gateway": "mygateway",
    "id": "telegram 1234567890",
    "timestamp": "2024-01-01T12:00:00Z",
    "extra": {
      "file": [
        {
          "Name": "photo.jpg",
          "Data": "base64-encoded-image-data",
          "Comment": "图片说明",
          "Size": 123456,
          "URL": "https://example.com/file.jpg"
        }
      ]
    }
  }
]
```

**注意**：调用后消息队列会被清空，每个消息只会返回一次。

### 5.2 流式接收（GET /api/stream）

```bash
curl -N http://127.0.0.1:4242/api/stream \
  -H "Authorization: Bearer your-secret-token"
```

使用 Server-Sent Events (SSE)，消息会实时流式返回。

### 5.3 WebSocket 方式（推荐）

```javascript
// WebSocket 连接需要使用 Authorization header
// 注意：浏览器 WebSocket API 不支持自定义 header，需要通过查询参数或使用支持 header 的库
const ws = new WebSocket('ws://127.0.0.1:4242/api/websocket?key=your-secret-token');

// 或者使用支持 header 的 WebSocket 库（如 ws 库）
// const ws = new WebSocket('ws://127.0.0.1:4242/api/websocket', {
//   headers: {
//     'Authorization': 'Bearer your-secret-token'
//   }
// });

ws.onmessage = (event) => {
  const message = JSON.parse(event.data);
  console.log('收到消息:', message);
  
  // 检查是否有文件
  if (message.extra && message.extra.file) {
    message.extra.file.forEach(file => {
      // file.Data 是 base64 编码的字节数组
      // 需要解码才能使用
      const fileData = atob(file.Data);
      console.log('文件名:', file.Name);
      console.log('文件大小:', file.Size);
      console.log('文件说明:', file.Comment);
    });
  }
};
```

## 6. Python AI Agent 对接示例

### 6.1 完整的 Python 客户端实现

```python
import requests
import base64
import json
import websocket
import threading
from typing import Optional, List, Dict, Any

class MatterbridgeClient:
    """Matterbridge API 客户端，用于对接 AI Agent"""
    
    def __init__(self, api_url: str, token: Optional[str] = None):
        self.api_url = api_url.rstrip('/')
        self.token = token
        self.headers = {
            'Content-Type': 'application/json'
        }
        if token:
            self.headers['Authorization'] = f'Bearer {token}'
    
    def send_text(self, text: str, username: str, gateway: str) -> Dict:
        """发送文本消息"""
        payload = {
            'text': text,
            'username': username,
            'gateway': gateway
        }
        response = requests.post(
            f'{self.api_url}/api/message',
            headers=self.headers,
            json=payload
        )
        response.raise_for_status()
        return response.json()
    
    def send_file(
        self,
        file_path: str,
        text: str,
        username: str,
        gateway: str,
        comment: Optional[str] = None
    ) -> Dict:
        """发送文件（图片或文档）"""
        # 读取文件并转换为 base64
        with open(file_path, 'rb') as f:
            file_data = base64.b64encode(f.read()).decode('utf-8')
        
        # 获取文件大小
        import os
        file_size = os.path.getsize(file_path)
        file_name = os.path.basename(file_path)
        
        payload = {
            'text': text,
            'username': username,
            'gateway': gateway,
            'extra': {
                'file': [{
                    'Name': file_name,
                    'Data': file_data,
                    'Comment': comment or '',
                    'Size': file_size
                }]
            }
        }
        
        response = requests.post(
            f'{self.api_url}/api/message',
            headers=self.headers,
            json=payload
        )
        response.raise_for_status()
        return response.json()
    
    def send_image(
        self,
        image_path: str,
        text: str,
        username: str,
        gateway: str,
        comment: Optional[str] = None
    ) -> Dict:
        """发送图片（便捷方法）"""
        return self.send_file(image_path, text, username, gateway, comment)
    
    def get_messages(self) -> List[Dict]:
        """获取消息队列（轮询方式）"""
        response = requests.get(
            f'{self.api_url}/api/messages',
            headers=self.headers
        )
        response.raise_for_status()
        return response.json()
    
    def stream_messages(self, callback):
        """流式接收消息（SSE）"""
        response = requests.get(
            f'{self.api_url}/api/stream',
            headers=self.headers,
            stream=True
        )
        response.raise_for_status()
        
        for line in response.iter_lines():
            if line:
                try:
                    message = json.loads(line)
                    callback(message)
                except json.JSONDecodeError:
                    continue
    
    def start_websocket(self, on_message, on_error=None):
        """启动 WebSocket 连接（推荐方式）"""
        url = f'{self.api_url.replace("http://", "ws://").replace("https://", "wss://")}/api/websocket'
        # WebSocket 连接需要设置 Authorization header
        # websocket-client 库支持通过 header 参数传递
        headers = {}
        if self.token:
            headers['Authorization'] = f'Bearer {self.token}'
            # 同时支持查询参数作为备用
            url += f'?key={self.token}'
        
        def on_ws_message(ws, message):
            try:
                msg = json.loads(message)
                on_message(msg)
            except json.JSONDecodeError as e:
                if on_error:
                    on_error(e)
        
        ws = websocket.WebSocketApp(
            url,
            on_message=on_ws_message,
            on_error=on_error,
            header=headers if headers else None  # 传递 Authorization header
        )
        
        # 在后台线程运行
        def run():
            ws.run_forever()
        
        thread = threading.Thread(target=run, daemon=True)
        thread.start()
        return ws


class AIAgentBot:
    """AI Agent Bot 示例实现"""
    
    def __init__(self, matterbridge_client: MatterbridgeClient, ai_api_key: str):
        self.client = matterbridge_client
        self.ai_api_key = ai_api_key
        self.gateway = "mygateway"
        self.bot_username = "AI Assistant"
    
    def process_message(self, message: Dict):
        """处理接收到的消息"""
        # 忽略自己发送的消息
        if message.get('username') == self.bot_username:
            return
        
        text = message.get('text', '')
        username = message.get('username', 'Unknown')
        
        # 检查是否有文件
        files = []
        if message.get('extra') and message.get('extra').get('file'):
            for file_info in message['extra']['file']:
                files.append({
                    'name': file_info.get('Name'),
                    'data': file_info.get('Data'),  # base64
                    'size': file_info.get('Size'),
                    'comment': file_info.get('Comment')
                })
        
        # 调用 AI API 处理消息
        response = self.call_ai_api(text, files, username)
        
        # 发送 AI 响应
        self.client.send_text(
            text=response,
            username=self.bot_username,
            gateway=self.gateway
        )
    
    def call_ai_api(self, text: str, files: List[Dict], username: str) -> str:
        """调用 AI API（示例：OpenAI）"""
        import openai
        
        # 构建消息
        messages = [{
            'role': 'user',
            'content': f'{username} 说: {text}'
        }]
        
        # 如果有图片，添加到消息中
        if files:
            for file in files:
                if file['name'].lower().endswith(('.jpg', '.jpeg', '.png', '.gif')):
                    # OpenAI Vision API 示例
                    messages.append({
                        'role': 'user',
                        'content': [
                            {
                                'type': 'text',
                                'text': f'这是一张图片: {file["name"]}'
                            },
                            {
                                'type': 'image_url',
                                'image_url': {
                                    'url': f'data:image/jpeg;base64,{file["data"]}'
                                }
                            }
                        ]
                    })
        
        # 调用 AI API
        client = openai.OpenAI(api_key=self.ai_api_key)
        response = client.chat.completions.create(
            model='gpt-4-vision-preview',
            messages=messages
        )
        
        return response.choices[0].message.content
    
    def start(self):
        """启动 Bot"""
        def on_message(message: Dict):
            self.process_message(message)
        
        def on_error(error):
            print(f'WebSocket 错误: {error}')
        
        print('AI Bot 启动中...')
        self.client.start_websocket(on_message, on_error)
        print('AI Bot 已启动，等待消息...')


# 使用示例
if __name__ == '__main__':
    # 初始化 Matterbridge 客户端
    mb_client = MatterbridgeClient(
        api_url='http://127.0.0.1:4242',
        token='your-secret-token'
    )
    
    # 创建 AI Bot
    bot = AIAgentBot(
        matterbridge_client=mb_client,
        ai_api_key='your-openai-api-key'
    )
    
    # 启动 Bot
    bot.start()
    
    # 保持运行
    import time
    try:
        while True:
            time.sleep(1)
    except KeyboardInterrupt:
        print('Bot 已停止')
```

### 6.2 处理图片的完整示例

```python
import base64
from PIL import Image
import io

def process_image_from_message(message: Dict) -> Optional[Image.Image]:
    """从消息中提取并处理图片"""
    if not message.get('extra') or not message['extra'].get('file'):
        return None
    
    for file_info in message['extra']['file']:
        file_name = file_info.get('Name', '').lower()
        
        # 检查是否为图片
        if file_name.endswith(('.jpg', '.jpeg', '.png', '.gif', '.webp')):
            # 解码 base64
            file_data = base64.b64decode(file_info['Data'])
            
            # 转换为 PIL Image
            image = Image.open(io.BytesIO(file_data))
            
            # 可以在这里进行图片处理
            # 例如：调整大小、OCR、图像识别等
            
            return image
    
    return None

# 使用示例
def handle_message_with_image(message: Dict):
    image = process_image_from_message(message)
    if image:
        print(f'收到图片: {image.size}')
        # 进行图片分析、OCR 等处理
        # ...
```

## 7. JavaScript/TypeScript 示例

### 7.1 Node.js 客户端

```javascript
const axios = require('axios');
const WebSocket = require('ws');
const fs = require('fs');

class MatterbridgeClient {
    constructor(apiUrl, token) {
        this.apiUrl = apiUrl.replace(/\/$/, '');
        this.token = token;
        this.headers = {
            'Content-Type': 'application/json'
        };
        if (token) {
            this.headers['Authorization'] = `Bearer ${token}`;
        }
    }
    
    async sendText(text, username, gateway) {
        const response = await axios.post(
            `${this.apiUrl}/api/message`,
            {
                text,
                username,
                gateway
            },
            { headers: this.headers }
        );
        return response.data;
    }
    
    async sendFile(filePath, text, username, gateway, comment = '') {
        // 读取文件并转换为 base64
        const fileBuffer = fs.readFileSync(filePath);
        const fileData = fileBuffer.toString('base64');
        const stats = fs.statSync(filePath);
        const fileName = require('path').basename(filePath);
        
        const response = await axios.post(
            `${this.apiUrl}/api/message`,
            {
                text,
                username,
                gateway,
                extra: {
                    file: [{
                        Name: fileName,
                        Data: fileData,
                        Comment: comment,
                        Size: stats.size
                    }]
                }
            },
            { headers: this.headers }
        );
        return response.data;
    }
    
    startWebSocket(onMessage, onError) {
        const wsUrl = this.apiUrl
            .replace('http://', 'ws://')
            .replace('https://', 'wss://') + '/api/websocket';
        
        // WebSocket 连接需要设置 Authorization header
        // ws 库支持通过 options 传递 headers
        const options = {};
        if (this.token) {
            options.headers = {
                'Authorization': `Bearer ${this.token}`
            };
            // 同时支持查询参数作为备用
            wsUrl += `?key=${this.token}`;
        }
        
        const ws = new WebSocket(wsUrl, options);
        
        ws.on('message', (data) => {
            try {
                const message = JSON.parse(data.toString());
                onMessage(message);
            } catch (e) {
                if (onError) onError(e);
            }
        });
        
        ws.on('error', (error) => {
            if (onError) onError(error);
        });
        
        return ws;
    }
}

// 使用示例
const client = new MatterbridgeClient('http://127.0.0.1:4242', 'your-token');

// 发送文本
client.sendText('Hello!', 'AI Bot', 'mygateway');

// 发送图片
client.sendFile('./image.jpg', '这是一张图片', 'AI Bot', 'mygateway');

// WebSocket 接收消息
client.startWebSocket((message) => {
    console.log('收到消息:', message);
    
    // 处理文件
    if (message.extra && message.extra.file) {
        message.extra.file.forEach(file => {
            const fileBuffer = Buffer.from(file.Data, 'base64');
            // 保存文件或处理
            fs.writeFileSync(`./received_${file.Name}`, fileBuffer);
        });
    }
}, (error) => {
    console.error('WebSocket 错误:', error);
});
```

## 8. 最佳实践

### 8.1 文件大小限制

- **建议**：单个文件不超过 10MB
- **原因**：Base64 编码会增加约 33% 的大小
- **处理**：大文件建议先上传到服务器，然后使用 `URL` 字段

### 8.2 错误处理

```python
try:
    response = client.send_file('image.jpg', '图片', 'Bot', 'gateway')
except requests.exceptions.HTTPError as e:
    if e.response.status_code == 401:
        print('认证失败，请检查 Token')
    elif e.response.status_code == 400:
        print('请求格式错误:', e.response.text)
    else:
        print('发送失败:', e)
```

### 8.3 消息去重

```python
processed_ids = set()

def process_message(message):
    msg_id = message.get('id')
    if msg_id in processed_ids:
        return  # 已处理过
    processed_ids.add(msg_id)
    # 处理消息...
```

### 8.4 异步处理

```python
import asyncio
from concurrent.futures import ThreadPoolExecutor

executor = ThreadPoolExecutor(max_workers=5)

def process_message_async(message):
    executor.submit(process_message, message)
```

## 9. 常见问题

### Q1: 如何判断消息中是否包含文件？

```python
def has_file(message: Dict) -> bool:
    return bool(
        message.get('extra') and 
        message['extra'].get('file') and 
        len(message['extra']['file']) > 0
    )
```

### Q2: 如何保存接收到的文件？

```python
def save_file_from_message(message: Dict, save_dir: str = './downloads'):
    import os
    os.makedirs(save_dir, exist_ok=True)
    
    if not has_file(message):
        return
    
    for file_info in message['extra']['file']:
        file_data = base64.b64decode(file_info['Data'])
        file_path = os.path.join(save_dir, file_info['Name'])
        with open(file_path, 'wb') as f:
            f.write(file_data)
        print(f'文件已保存: {file_path}')
```

### Q3: 支持哪些图片格式？

支持所有 Matterbridge 桥接协议支持的格式：
- **图片**：JPG, PNG, GIF, WebP, BMP
- **文档**：PDF, DOC, DOCX, TXT, ZIP 等

### Q4: WebSocket 断线重连？

```python
import time

def start_websocket_with_reconnect(client, on_message):
    while True:
        try:
            ws = client.start_websocket(on_message, None)
            # 等待连接关闭
            time.sleep(1)
        except Exception as e:
            print(f'连接断开，5秒后重连: {e}')
            time.sleep(5)
```

## 10. 认证说明

### 10.1 认证格式

Matterbridge API 使用 **Bearer Token** 认证方式：

```http
Authorization: Bearer your-token-here
```

### 10.2 认证位置

- **HTTP 请求**：在 `Authorization` header 中传递
- **WebSocket 连接**：
  - 推荐：在连接时通过 `Authorization` header 传递
  - 备用：也可以通过查询参数 `?key=your-token` 传递（某些客户端可能不支持自定义 header）

### 10.3 认证示例

**HTTP 请求**：
```bash
curl -H "Authorization: Bearer mytoken" http://127.0.0.1:4242/api/messages
```

**WebSocket 连接（Python）**：
```python
ws = websocket.WebSocketApp(
    'ws://127.0.0.1:4242/api/websocket',
    header={'Authorization': 'Bearer mytoken'}
)
```

**WebSocket 连接（JavaScript/Node.js）**：
```javascript
const ws = new WebSocket('ws://127.0.0.1:4242/api/websocket', {
    headers: {
        'Authorization': 'Bearer mytoken'
    }
});
```

## 11. Go 语言客户端（agent_client）

项目提供了完整的 Go 语言客户端实现，可以直接使用：

### 11.1 快速开始

```bash
# 编译
cd agent_client
go build -o agent_client main.go client.go

# 运行
./agent_client \
  -api-url="http://127.0.0.1:4242" \
  -token="mytoken" \
  -gateway="mygateway" \
  -bot-username="AI Agent" \
  -agent-cmd="agent" \
  -agent-args="--print" \
  -mode="websocket" \
  -debug
```

### 11.2 功能特性

- ✅ **WebSocket 和轮询模式**：支持两种消息接收方式
- ✅ **并发处理**：消息异步处理，不阻塞
- ✅ **自动重连**：连接失败时自动重连
- ✅ **错误处理**：完善的错误处理和日志记录
- ✅ **消息去重**：避免重复处理相同消息

### 11.3 使用示例

```bash
# WebSocket 模式（推荐）
./agent_client \
  -api-url="http://127.0.0.1:4242" \
  -token="mytoken" \
  -gateway="mygateway" \
  -mode="websocket"

# 轮询模式
./agent_client \
  -api-url="http://127.0.0.1:4242" \
  -token="mytoken" \
  -gateway="mygateway" \
  -mode="polling" \
  -interval="3s"

# 自定义并发数
./agent_client \
  -api-url="http://127.0.0.1:4242" \
  -token="mytoken" \
  -gateway="mygateway" \
  -max-workers=20
```

详细文档请参考：`agent_client/README.md` 和 `agent_client/QUICKSTART.md`

## 12. 总结

Matterbridge API 完全支持图片和文档的传输，非常适合对接 AI Agent：

✅ **发送文件**：通过 `extra.file` 数组，使用 base64 编码  
✅ **接收文件**：从 `extra.file` 数组中获取，包含完整的文件信息  
✅ **实时通信**：WebSocket 或 SSE 流式接收  
✅ **多文件支持**：一次可以发送多个文件  
✅ **元数据支持**：文件名、大小、注释等信息完整保留  
✅ **Bearer Token 认证**：使用标准的 `Authorization: Bearer token` 格式  
✅ **Go 客户端**：提供完整的 Go 语言客户端实现（agent_client）  

通过 Matterbridge API，你可以轻松构建一个支持多协议、支持文件传输的 AI Bot！

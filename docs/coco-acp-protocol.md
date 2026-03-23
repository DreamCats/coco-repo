# coco ACP 协议研究报告

> 日期：2026-03-21
> coco 版本：0.120.12 (build 2026-03-20)
> 状态：协议全通，可落地 wrapper

---

## 1. 概述

coco（公司内部 AI 编码 CLI，别名 traecli/trae-agent/ta）内置 ACP server，通过 `coco acp serve` 启动 stdio 模式的 JSON-RPC 服务。

**核心价值：**

- 一次启动（~14s），后续零启动开销
- 20 个模型可选，全部走公司免费额度
- 自带 livecoding 插件和工具链
- 支持多轮对话、上下文压缩、模型切换

---

## 2. 协议详情

### 2.1 启动

```bash
coco acp serve   # stdio 模式，stdin/stdout JSON-RPC
```

### 2.2 方法速查

| 方法             | 用途         | 关键参数                                                               |
| ---------------- | ------------ | ---------------------------------------------------------------------- |
| `initialize`     | 握手         | `{protocolVersion: 1, capabilities: {}, clientInfo: {name, version}}`  |
| `session/new`    | 创建新会话   | `{cwd: "路径", mcpServers: []}` → 返回 `sessionId` + 模型列表          |
| `session/load`   | 加载已有会话 | `{cwd, mcpServers: []}` → 需已有 session                               |
| `session/prompt` | 发消息       | `{sessionId, prompt: [{type:"text", text:"..."}], modelId?: "模型名"}` |

### 2.3 initialize 请求/响应

```json
// 请求
{"jsonrpc":"2.0","id":1,"method":"initialize","params":{
  "protocolVersion": 1,
  "capabilities": {},
  "clientInfo": {"name":"client","version":"0.1.0"}
}}

// 响应
{"jsonrpc":"2.0","id":1,"result":{
  "_meta": {"workspace": "/path/to/workspace"},
  "agentCapabilities": {
    "mcpCapabilities": {"http": true, "sse": true},
    "promptCapabilities": {}
  },
  "authMethods": [],
  "protocolVersion": 1
}}
```

**注意：** `protocolVersion` 必须是整数 `1`，字符串（如 `"2025-03-26"`）会报错。

### 2.4 session/new 请求/响应

```json
// 请求
{"jsonrpc":"2.0","id":2,"method":"session/new","params":{
  "cwd": "/path/to/workspace",
  "mcpServers": []
}}

// 响应（含完整模型列表和可用模式）
{"jsonrpc":"2.0","id":2,"result":{
  "_meta": {"cwd": "/path/to/workspace"},
  "models": {
    "availableModels": [...],  // 20 个模型
    "currentModelId": "Doubao-Seed-2.0-Code"
  },
  "modes": {
    "availableModes": [
      {"id": "default", "name": "Default"},
      {"id": "bypass_permissions", "name": "Accept All Tools"},
      {"id": "plan", "name": "Plan Mode"}
    ],
    "currentModeId": "default"
  },
  "sessionId": "uuid"
}}
```

### 2.5 session/prompt 请求/响应

```json
// 请求
{"jsonrpc":"2.0","id":3,"method":"session/prompt","params":{
  "sessionId": "uuid",
  "prompt": [{"type": "text", "text": "你的消息"}],
  "modelId": "GPT-5"  // 可选，临时切模型
}}

// 响应：多条 notification + 最终 result
// 1. 工具调用通知
{"jsonrpc":"2.0","method":"session/update","params":{
  "sessionId": "uuid",
  "update": {
    "sessionUpdate": "tool_call",
    "kind": "read",
    "title": "Read /path/to/file",
    "status": "in_progress"
  }
}}

// 2. 流式文本 chunk
{"jsonrpc":"2.0","method":"session/update","params":{
  "sessionId": "uuid",
  "update": {
    "sessionUpdate": "agent_message_chunk",
    "content": {"text": "部分", "type": "text"}
  }
}}

// 3. 最终完成
{"jsonrpc":"2.0","id":3,"result":{"stopReason": "end_turn"}}
```

### 2.6 特殊指令

通过 prompt 文本发送斜杠命令：

| 指令               | 效果                   | 耗时  |
| ------------------ | ---------------------- | ----- |
| `/compact`         | 上下文压缩，保留摘要   | ~0.6s |
| `/livecoding:init` | 初始化 livecoding 配置 | 未测  |
| `/review <MR>`     | Review merge request   | 未测  |

### 2.7 已验证不存在的方法

以下方法返回 Method not found：
`session/start`, `session/message`, `session/send`, `session/chat`, `session/turn`, `session/query`, `session/create`, `session/run`, `session/setModel`, `session/config`, `session/command`, `message/send`, `tasks/send`, `turn/send`

---

## 3. 可用模型（20 个）

| 模型                       | 说明                       | 类型     |
| -------------------------- | -------------------------- | -------- |
| **Doubao-Seed-2.0-Code**   | Agentic coding（默认）     | 字节自研 |
| Doubao-Seed-1.8            | 多模态搜索/代码/推理       | 字节自研 |
| Doubao-Seed-Code           | 代码生成和重构             | 字节自研 |
| Doubao-Seed-Code-Beta      | 实验性代码智能             | 字节自研 |
| **MiniMax-M2.7**           | 自进化推理 + 多 agent 协作 | MiniMax  |
| MiniMax-M2.5               | 编码 + agent 工具          | MiniMax  |
| **GLM-5**                  | 前沿 agentic 智能和推理    | 智谱     |
| GLM-4.7                    | 全能型：编码/推理/UI       | 智谱     |
| **Gemini-3.1-Pro-Preview** | 高级多模态推理             | Google   |
| Gemini-3-Flash-Preview     | 快速多模态，低延迟         | Google   |
| Gemini-2.5-Pro             | 稳定多模态通用             | Google   |
| **DeepSeek-V3.2**          | 新一代代码生成 + 数学      | DeepSeek |
| DeepSeek-V3.1              | 稳定版代码/数学            | DeepSeek |
| **Kimi-K2.5**              | 原生 agentic + 工具编排    | Moonshot |
| **GPT-5.2**                | 最强推理深度               | OpenAI   |
| GPT-5.1                    | 强推理低成本               | OpenAI   |
| GPT-5                      | 通用推理                   | OpenAI   |
| **GPT-5.2-Codex**          | Beta - Agentic coding      | OpenAI   |
| Qwen3.5-Plus               | 代码理解和生成             | 阿里     |
| Qwen3-Coder-Next           | 轻量编码                   | 阿里     |

---

## 4. 可用插件命令

session/new 后通过 `session/update` 推送的 `availableCommands`：

| 命令              | 说明                         |
| ----------------- | ---------------------------- |
| `livecoding:init` | 初始化项目的 livecoding 配置 |
| `agent-new`       | 创建新子 agent 配置          |
| `init`            | 初始化 AGENTS.md             |
| `review`          | Review merge request         |
| `compact`         | 上下文压缩                   |

---

## 5. 性能数据

| 指标               | 数值                 |
| ------------------ | -------------------- |
| coco 冷启动        | ~14s                 |
| ACP 进程复用后每轮 | 7-9s（模型推理时间） |
| /compact           | 0.6s                 |
| coco -p 单次调用   | 14-15s（含启动）     |

**结论：ACP 常驻模式比每次 `coco -p` 快，多轮场景优势明显（省掉 N-1 次启动开销）。**

---

## 6. Python 客户端参考实现

```python
import subprocess, json, time, select, threading, queue

class CocoACP:
    def __init__(self, cwd='/tmp'):
        self.proc = subprocess.Popen(
            ['coco', 'acp', 'serve'],
            stdin=subprocess.PIPE, stdout=subprocess.PIPE,
            stderr=subprocess.PIPE, text=True, bufsize=1
        )
        self.queue = queue.Queue()
        self._reader = threading.Thread(target=self._read_loop, daemon=True)
        self._reader.start()
        self._id = 0

        # initialize
        self._send('initialize', {
            'protocolVersion': 1,
            'capabilities': {},
            'clientInfo': {'name': 'coco-prd', 'version': '0.1.0'}
        })
        self._wait_result()

        # session/new
        self._send('session/new', {'cwd': cwd, 'mcpServers': []})
        result = self._wait_result()
        self.session_id = result.get('sessionId', '')

    def _read_loop(self):
        while True:
            line = self.proc.stdout.readline()
            if not line:
                break
            if line.strip():
                self.queue.put(json.loads(line.strip()))

    def _send(self, method, params):
        self._id += 1
        msg = {'jsonrpc': '2.0', 'id': self._id, 'method': method, 'params': params}
        self.proc.stdin.write(json.dumps(msg) + '\n')
        self.proc.stdin.flush()
        return self._id

    def _wait_result(self, timeout=60):
        deadline = time.time() + timeout
        while time.time() < deadline:
            try:
                msg = self.queue.get(timeout=1)
                if 'result' in msg:
                    return msg['result']
            except queue.Empty:
                continue
        return None

    def prompt(self, text, model_id=None):
        """发送消息，返回完整回复文本"""
        params = {
            'sessionId': self.session_id,
            'prompt': [{'type': 'text', 'text': text}]
        }
        if model_id:
            params['modelId'] = model_id

        req_id = self._send('session/prompt', params)

        text_parts = []
        deadline = time.time() + 120
        while time.time() < deadline:
            try:
                msg = self.queue.get(timeout=1)
            except queue.Empty:
                continue

            if msg.get('id') == req_id and 'result' in msg:
                return ''.join(text_parts)

            update = msg.get('params', {}).get('update', {})
            if update.get('sessionUpdate') == 'agent_message_chunk':
                c = update.get('content', {})
                if isinstance(c, dict):
                    text_parts.append(c.get('text', ''))

        return ''.join(text_parts)

    def compact(self):
        return self.prompt('/compact')

    def close(self):
        self.proc.terminate()

# 使用示例
# client = CocoACP(cwd='/path/to/repo')
# reply = client.prompt('分析这个 PRD: ...')
# client.prompt('/compact')
# reply = client.prompt('基于上面的分析，做代码调研', model_id='GPT-5.2')
# client.close()
```

---

## 7. 待探索

- [ ] `session/load` 加载已有 session 的完整参数
- [ ] MCP server 配置（`mcpServers` 参数传什么）
- [ ] `session/prompt` 的 `modelId` 是否对整个 session 持久化还是单次生效
- [ ] 工具调用审批（bypass_permissions 模式）
- [ ] coco 的 `-c` 参数能否通过 ACP 传入
- [ ] 与 OpenClaw acpx 的兼容性（protocolVersion 差异）
- [ ] 长时间运行稳定性（进程保活、OOM 处理）
- [ ] livecoding 插件命令的完整调用方式

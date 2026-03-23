# coco-repo — 仓库级代码上下文知识库

## 背景

团队仓库业务知识复杂度较高，当前 agent 对代码理解非常强，但缺乏业务知识上下文，导致与人生成的技术方案和代码不及预期。

## 模型

目录结构

```
.livecoding/
├── README.md                    # 使用说明
└── knowledge/                   # 跨任务复用的知识库
    ├── glossary.md              # 业务术语 ↔ 代码标识符映射（核心）
    ├── idl.md                   # IDL 协议文件说明（proto/thrift 位置、生成流程、改动链路）
    ├── patterns.md              # 代码模式（Hertz handler / Kitex service 怎么写）
    ├── conventions.md           # 团队约定（命名、错误处理、日志、IDL 编写规范）
    └── dependencies.md          # 下游服务 + 接口速查（含 thrift service/method）
```

### glossary.md — 业务术语表（核心）

为什么是核心： 没有术语映射，AI 连代码都搜不到，后续所有步骤都无法进行。

```markdown
# 业务术语表

> 维护规则：每次做需求发现新术语时追加，保持按模块分组

## 直播间

| 业务术语 | 代码标识符   | 别名                       | 所在模块       |
| -------- | ------------ | -------------------------- | -------------- |
| 讲解卡   | PopCard      | pop_card, explanation_card | live/popcard/  |
| 福袋     | LuckyBag     | lucky_bag, giveaway        | live/luckybag/ |
| 小黄车   | ShoppingCart | cart, product_shelf        | live/cart/     |

## 电商

| 业务术语   | 代码标识符 | 别名              | 所在模块      |
| ---------- | ---------- | ----------------- | ------------- |
| 拍卖       | Auction    | auction, bid      | live/auction/ |
| 直播间榜单 | LiveRank   | rank, leaderboard | live/rank/    |
```

冷启动方式：

1. 买哥手动写 20-30 个高频术语（预估 30 分钟）
2. 后续每次做需求，skill 自动提示补充新发现的映射
3. 可选：扫代码 package 名 + struct 名，批量让人标注

## Knowledge 文件模板

### glossary.md 模板

```markdown
# 业务术语表

> 由 prd-mr-init 自动生成 + 人工标注
> 维护规则：每次做需求发现新术语时追加，保持按模块分组
> 标记说明：✅ = 已确认 | ❓ = AI 推测，需人工确认

## 直播间

| 业务术语 | 代码标识符 | 别名                       | 所在模块       | 状态 |
| -------- | ---------- | -------------------------- | -------------- | ---- | ----------------------- |
| 讲解卡   | PopCard    | pop_card, explanation_card | live/popcard/  | ✅   | <!-- manual -->         |
| 福袋     | LuckyBag   | lucky_bag, giveaway        | live/luckybag/ | ✅   | <!-- manual -->         |
| ???      | LiveBanner | live_banner                | live/banner/   | ❓   | <!-- auto-generated --> |

## 电商

| 业务术语 | 代码标识符 | 别名         | 所在模块        | 状态 |
| -------- | ---------- | ------------ | --------------- | ---- | ----------------------- |
| 拍卖     | Auction    | auction, bid | live/auction/   | ✅   | <!-- manual -->         |
| ???      | FlashDeal  | flash_deal   | live/promotion/ | ❓   | <!-- auto-generated --> |
```

### idl.md 模板

```markdown
# IDL 协议文件说明

> 最后更新：YYYY-MM-DD

## IDL 类型与位置

| 类型 | 框架 | IDL 语言 | 本地目录 | 说明 |
|------|------|----------|----------|------|
| HTTP API | Hertz | proto | http_idl/ | 对外 HTTP 接口定义 |
| RPC 服务 | Kitex | thrift | rpc_idl/ | 内部 RPC 接口定义 |

## 代码生成

\`\`\`bash
# HTTP（Hertz + proto）
hz update -idl http_idl/xxx.proto

# RPC（Kitex + thrift）
kitex -module xxx rpc_idl/xxx.thrift
\`\`\`

**生成产物位置：**
- Hertz：生成到 `biz/handler/`、`biz/model/`（proto 对应的 Go struct）
- Kitex：生成到 `kitex_gen/`（thrift 对应的 Go struct + client/server 代码）

## 新增字段的改动链路

1. **改 IDL** — 在 proto/thrift 中新增字段定义
2. **重新生成** — 执行上述代码生成命令
3. **改 converter** — 在 converter 层补充字段映射（最常见的改动点）
4. **改 handler/service** — 通常不用改（除非加新参数或新逻辑）

## 注意事项

- proto 字段编号不可复用已删除的编号
- thrift 字段 ID 必须递增，不可跳号后回填
- IDL 变更需同步通知上下游服务方
```

### patterns.md 模板

```markdown
# 代码模式

> 由 prd-mr-init 自动提取，人工确认后供 prd-assess/prd-codegen 参考
> 技术栈：Go + Hertz（HTTP API）+ Kitex（RPC）
> 最后更新：YYYY-MM-DD

## 模式 1：Hertz Handler → Service → Converter（最常见，HTTP API）

**适用场景：** 标准 HTTP API 接口（Hertz 框架）

**目录结构：**
\`\`\`
module/
├── handler/
│   └── get_xxx.go          # Hertz handler，参数校验，调 service
├── service/
│   └── xxx_service.go      # 业务逻辑，调下游 Kitex RPC
├── converter/
│   └── xxx_converter.go    # 数据转换（Kitex RPC response → Hertz API response）
└── model/
    └── xxx.go              # 数据结构定义
\`\`\`

**典型代码骨架（Hertz handler）：**

\`\`\`go
// handler/get_product_detail.go
func GetProductDetail(ctx context.Context, c *app.RequestContext) {
    var req api.GetProductDetailRequest
    if err := c.BindAndValidate(&req); err != nil {
        // 参数校验失败
        c.JSON(http.StatusBadRequest, errno.ParamError)
        return
    }
    // 调 service
    result, err := service.GetProductDetail(ctx, req.ProductId)
    if err != nil {
        c.JSON(http.StatusInternalServerError, err)
        return
    }
    // 转换 + 返回
    c.JSON(http.StatusOK, converter.ConvertProductDetailResponse(result))
}
\`\`\`

**新增字段时的改动链路：**

1. http_idl/ 中改 proto 定义 → 重新生成
2. converter/ 加字段映射（最常见的改动点）
3. handler/ 通常不用改（除非加新参数）

## 模式 2：Kitex Service Impl（RPC 服务端）

**适用场景：** 提供 RPC 接口给其他服务调用（Kitex 框架）

**目录结构：**
\`\`\`
module/
├── handler/
│   └── xxx_impl.go         # Kitex 生成的 service impl，实现 thrift 定义的接口
├── service/
│   └── xxx_service.go      # 业务逻辑
├── converter/
│   └── xxx_converter.go    # 数据转换
└── kitex_gen/               # Kitex 自动生成的代码（不手动修改）
\`\`\`

**典型代码骨架（Kitex service impl）：**

\`\`\`go
// handler/xxx_impl.go
func (s *XxxServiceImpl) GetProductDetail(ctx context.Context, req *rpc.GetProductDetailRequest) (*rpc.GetProductDetailResponse, error) {
    // 1. 参数校验
    if req.ProductId == 0 {
        return nil, errno.ParamError
    }
    // 2. 调 service
    result, err := service.GetProductDetail(ctx, req.ProductId)
    if err != nil {
        return nil, err
    }
    // 3. 转换 + 返回
    return converter.ConvertProductDetailResponse(result), nil
}
\`\`\`

## 模式 3：Event Consumer（异步消费）

**适用场景：** 消息队列消费

**典型结构：**
\`\`\`
module/
├── consumer/
│   └── xxx_consumer.go     # 消息消费入口
├── handler/
│   └── handle_xxx.go       # 消息处理逻辑
└── model/
    └── xxx_event.go        # 事件数据结构
\`\`\`

## 模式 4：Cron Job（定时任务）

**适用场景：** 定时数据同步、清理

**典型结构：**
\`\`\`
module/
├── cron/
│   └── xxx_job.go          # 定时任务入口
└── service/
    └── xxx_service.go      # 复用 service 层逻辑
\`\`\`
```

### conventions.md 模板

```markdown
# 团队编码约定

> 由 prd-mr-init 从代码库自动提取，人工确认
> AI 编码时必须遵守以下约定，保持代码风格一致
> 技术栈：Go + Hertz（HTTP）+ Kitex（RPC）+ proto（HTTP IDL）+ thrift（RPC IDL）
> 最后更新：YYYY-MM-DD

## 命名规范

- **文件名：** snake_case（如 `product_detail.go`）
- **函数名：** CamelCase（如 `GetProductDetail`）
- **变量名：** camelCase（如 `productId`）
- **常量：** CamelCase 或 ALL_CAPS（视上下文）
- **package 名：** 全小写，单词（如 `converter`，不用 `product_converter`）

## 框架相关约定

### Hertz（HTTP API）

- 路由注册统一在 `router.go` / `register.go` 中
- Handler 签名：`func XxxHandler(ctx context.Context, c *app.RequestContext)`
- 参数绑定用 `c.BindAndValidate(&req)`
- 中间件注册在路由组上，不在 handler 内部处理

### Kitex（RPC）

- Service Impl 实现 thrift 生成的 interface
- `kitex_gen/` 目录为自动生成，禁止手动修改
- Client 初始化统一放在 `client/` 或 `init.go` 中
- 超时、重试等配置通过 Kitex Option 设置

## IDL 编写规范

### proto（HTTP IDL）

- 文件位于 `http_idl/` 目录
- message 命名用 CamelCase，字段用 snake_case
- 字段编号不可复用已删除的编号
- 新增字段加在最后，编号递增

### thrift（RPC IDL）

- 文件位于 `rpc_idl/` 目录
- struct 命名用 CamelCase，字段用 snake_case
- 字段 ID 必须递增，不可跳号后回填
- required/optional 明确标注

## 错误处理

\`\`\`go
// 标准模式：直接返回 error，不吞错
result, err := service.DoSomething(ctx, req)
if err != nil {
    logs.CtxError(ctx, "DoSomething failed, req=%v, err=%v", req, err)
    return nil, err
}

// 业务错误码模式：用 errno 包
if !valid {
    return nil, errno.New(errno.ParamError, "invalid product_id")
}
\`\`\`

## 日志规范

\`\`\`go
// 统一用 logs.Ctx* 系列，必须带 ctx
logs.CtxInfo(ctx, "message, key=%v", value)
logs.CtxWarn(ctx, "message, key=%v", value)
logs.CtxError(ctx, "message, key=%v, err=%v", value, err)

// 不要用：
// fmt.Println(...) ← 禁止
// log.Printf(...) ← 禁止
// logs.Info(...) ← 缺 ctx，禁止
\`\`\`

## Import 分组

\`\`\`go
import (
    // 标准库
    "context"
    "fmt"

    // 公司公共库
    "code.byted.org/xxx/common"

    // 当前项目内部包
    "code.byted.org/xxx/myproject/model"
)
\`\`\`

## 注释规范

- 导出函数必须有注释（golint 要求）
- 注释用中文或英文均可，但同一模块内保持一致
- TODO 格式：`// TODO(username): 描述`

## 测试规范

- 测试文件：`xxx_test.go`，与被测文件同目录
- 测试函数：`TestXxx(t *testing.T)`
- Mock：使用 `mockgen` 生成，放在 `mock/` 子目录
```

### dependencies.md 模板

```markdown
# 下游服务与接口速查

> 由 prd-mr-init 从 import + RPC 调用自动提取
> 调用方式：通过 Kitex 生成的 client 调用，IDL 为 thrift
> 最后更新：YYYY-MM-DD

## 服务依赖总览

| 下游服务        | 用途     | Thrift Service 名   | Client 包路径                     | 常用方法                 |
| --------------- | -------- | -------------------- | --------------------------------- | ------------------------ |
| product-service | 商品信息 | ProductService       | code.byted.org/xxx/product-client | GetProduct, ListProducts |
| order-service   | 订单管理 | OrderService         | code.byted.org/xxx/order-client   | CreateOrder, GetOrder    |
| user-service    | 用户信息 | UserService          | code.byted.org/xxx/user-client    | GetUser, BatchGetUsers   |

## 接口详情

### product-service（Thrift Service: ProductService）

| 方法         | 用途             | 调用方                | 入参关键字段  | 出参关键字段                             | Thrift IDL 位置              |
| ------------ | ---------------- | --------------------- | ------------- | ---------------------------------------- | ---------------------------- |
| GetProduct   | 获取单个商品详情 | live/product/service/ | product_id    | ProductInfo{name, price, auction_config} | rpc_idl/product_service.thrift |
| ListProducts | 批量获取商品     | live/shelf/service/   | product_ids[] | []ProductInfo                            | rpc_idl/product_service.thrift |

### 中间件

| 中间件 | 用途     | 包路径                          |
| ------ | -------- | ------------------------------- |
| Redis  | 缓存     | code.byted.org/xxx/redis-client |
| Kafka  | 消息队列 | code.byted.org/xxx/mq-client    |
| MySQL  | 持久化   | code.byted.org/xxx/dal          |
```

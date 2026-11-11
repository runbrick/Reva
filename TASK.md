你是一位资深的 Go 桌面应用架构师和全栈工程师，请帮我从零设计并开发一个类似 Postman 的本地 API 客户端桌面应用。

### 项目目标

开发一款**纯本地、无需账号、不连云端**的桌面 API 调试工具，用 Go 编写，体积小、启动快，作为 Postman 的轻量替代品。优先支持 REST，后续可扩展 GraphQL / WebSocket。

### 技术栈要求（必须遵守）

- 后端 &amp; 桌面壳 &amp; 前端：Go + viva
- 数据存储：全部本地文件，集合用 YAML 或 JSON 存储在用户目录，方便 Git 管理
- 禁止使用 Electron
- 目标体积：release 包尽量控制在 15MB 以内
- 支持 windows、mac 多端开发

### 核心功能（按优先级）

**MVP（第一阶段必须完成）：**

1. 发送 HTTP 请求（GET/POST/PUT/DELETE/PATCH 等）
2. 支持 Headers、Query Params、Body（JSON / form-data / raw / binary）
3. 响应展示：状态码、Headers、Body（支持 JSON 格式化 + 原始文本切换）
4. 请求耗时统计
5. 历史记录（最近发送的请求）
6. 简单的请求保存（本地文件）

**第二阶段：**

- 集合（Collection）管理，支持文件夹嵌套
- 环境变量（Environment）+ 变量替换（{{variable}}）
- 导入 Postman Collection / curl 命令
- 常用认证：Bearer Token、Basic Auth、API Key

**第三阶段（可选）：**

- 多标签页
- 预请求脚本 / 测试脚本（简单 JS 支持即可）
- 暗色/亮色主题切换
- 中文界面

### 架构要求

- Go 后端负责所有网络请求、文件读写、业务逻辑
- 前端只负责 UI 展示和用户交互
- 使用 Wails 自动生成的绑定，让前端可以类型安全地调用 Go 方法
- 代码结构清晰，按 service 分层（例如：RequestService、CollectionService、EnvironmentService）

### 输出要求

请按以下顺序推进：

1. 先给出推荐的项目目录结构
2. 给出 Wails 项目初始化命令和基础配置
3. 实现核心的「发送 HTTP 请求」Go Service + 对应的前端调用示例
4. 设计本地存储的数据结构（请求、集合、环境变量）
5. 逐步完善 MVP 功能

现在请从第 1 步开始：给出清晰的项目目录结构和初始化建议。
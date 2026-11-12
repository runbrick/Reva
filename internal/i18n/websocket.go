package i18n

// WebSocket holds all labels of a WebSocket session.
type WebSocket struct {
	URL, Headers, Message, Messages, Connect, Disconnect, Send, Save string
	Connected, Disconnected, Connecting, Sent, Received, Binary      string
	Empty, URLHint, Error, Clear, Truncated                          string
}

var websocketZh = WebSocket{
	URL: "WebSocket 地址", Headers: "请求头", Message: "消息", Messages: "消息记录",
	Connect: "连接", Disconnect: "断开连接", Send: "发送", Save: "保存",
	Connected: "已连接", Disconnected: "未连接", Connecting: "连接中",
	Sent: "已发送", Received: "已接收", Binary: "二进制消息",
	Empty: "暂无消息", URLHint: "ws:// 或 wss://", Error: "连接错误", Clear: "清空消息", Truncated: "…（已截断）",
}

var websocketEn = WebSocket{
	URL: "WebSocket URL", Headers: "Headers", Message: "Message", Messages: "Messages",
	Connect: "Connect", Disconnect: "Disconnect", Send: "Send", Save: "Save",
	Connected: "Connected", Disconnected: "Disconnected", Connecting: "Connecting",
	Sent: "Sent", Received: "Received", Binary: "Binary message",
	Empty: "No messages yet", URLHint: "ws:// or wss://", Error: "Connection error", Clear: "Clear messages", Truncated: "... (truncated)",
}

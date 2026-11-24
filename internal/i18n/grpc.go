package i18n

type GRPC struct {
	Payload, RequestContent                                                                   string
	NewRequest, URL, URLHint, Invoke, Cancel, Save, Calling, Cancelled                        string
	Input, InputHint, Metadata, MetadataHint, MetadataKey, MetadataValue, AddMetadata         string
	Response, Output, CopyHex, PreviewLimit, Waiting, NoPayload, EmptyPayload                 string
	Status, HTTPStatus, Time, Size, ExportProtocol                                            string
	InvalidURL, InvalidHex, InvalidMetadata, Limit, Transport, Protocol, Compression, Timeout string
}

var grpcZh = GRPC{
	Payload: "请求消息", RequestContent: "gRPC 请求内容",
	NewRequest: "新建 gRPC 请求", URL: "gRPC 地址", URLHint: "https://host:50051/package.Service/Method",
	Invoke: "调用", Cancel: "取消调用", Save: "保存", Calling: "调用中…", Cancelled: "已取消 gRPC 调用",
	Input: "请求 Protobuf 十六进制", InputHint: "每条消息最多 1 MiB；忽略空格、制表符和换行。留空表示零字节消息。",
	Metadata: "元数据", MetadataHint: "元数据值使用 ASCII；名称以 -bin 结尾时使用 Base64。",
	MetadataKey: "元数据名称", MetadataValue: "元数据值", AddMetadata: "添加元数据",
	Response: "gRPC 响应", Output: "响应 Protobuf 十六进制", CopyHex: "复制完整十六进制", PreviewLimit: "仅显示前 4 KiB。",
	Waiting: "暂无响应", NoPayload: "没有收到消息。", EmptyPayload: "收到零字节消息。",
	Status: "gRPC 状态 %d", HTTPStatus: "HTTP 状态 %d", Time: "%s", Size: "%d 字节",
	ExportProtocol:  "此格式仅支持普通 HTTP(S) 请求，排除 WebSocket、gRPC 和 GraphQL 编辑器请求。",
	InvalidURL:      "请输入包含 /服务/方法 的完整 HTTP(S) 地址。地址不能包含凭据、查询参数或片段。",
	InvalidHex:      "Protobuf 十六进制必须包含成对的 0–9 或 A–F 字符。",
	InvalidMetadata: "元数据无效。请检查名称、值及保留字段。",
	Limit:           "Protobuf 消息超过 1 MiB。", Transport: "无法完成 HTTP/2 连接。请检查地址、网络和 TLS 设置。",
	Protocol: "服务器未返回有效的 gRPC 一元响应。", Compression: "不支持压缩的 gRPC 消息。", Timeout: "gRPC 调用超时。",
}

var grpcEn = GRPC{
	Payload: "Message", RequestContent: "gRPC request content",
	NewRequest: "New gRPC Request", URL: "gRPC URL", URLHint: "https://host:50051/package.Service/Method",
	Invoke: "Invoke", Cancel: "Cancel call", Save: "Save", Calling: "Calling…", Cancelled: "gRPC call cancelled",
	Input: "Request protobuf hex", InputHint: "1 MiB per message. Spaces, tabs, and line breaks are ignored. Blank sends a zero-byte message.",
	Metadata: "Metadata", MetadataHint: "Use ASCII values, or Base64 for names ending in -bin.",
	MetadataKey: "Metadata name", MetadataValue: "Metadata value", AddMetadata: "Add metadata",
	Response: "gRPC response", Output: "Response protobuf hex", CopyHex: "Copy full hex", PreviewLimit: "Showing the first 4 KiB.",
	Waiting: "No response yet", NoPayload: "No message received.", EmptyPayload: "Received a zero-byte message.",
	Status: "gRPC status %d", HTTPStatus: "HTTP status %d", Time: "%s", Size: "%d bytes",
	ExportProtocol:  "This format supports ordinary HTTP(S) requests and excludes WebSocket, gRPC, and GraphQL editor requests.",
	InvalidURL:      "Enter a full HTTP(S) URL with /Service/Method and no credentials, query, or fragment.",
	InvalidHex:      "Protobuf hex must contain pairs of 0–9 or A–F characters.",
	InvalidMetadata: "Invalid metadata. Check the names, values, and reserved fields.",
	Limit:           "Protobuf message exceeds 1 MiB.", Transport: "Could not complete the HTTP/2 connection. Check the URL, network, and TLS settings.",
	Protocol: "The server did not return a valid unary gRPC response.", Compression: "Compressed gRPC messages are not supported.", Timeout: "The gRPC call timed out.",
}

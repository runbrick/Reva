package i18n

// Response holds the strings of the response viewer. Owned by that feature's PR.
type Response struct {
	Title        string
	EmptyHint    string
	Sending      string
	ErrorTitle   string
	Duration     string
	Size         string
	Truncated    string
	Body         string
	Headers      string
	Tests        string
	TestsCount   string
	Pretty       string
	Raw          string
	Copy         string
	CopyError    string
	DisplayLimit string
	HeaderName   string
	HeaderValue  string
	NoHeaders    string
}

var responseZh = Response{
	Title: "响应", EmptyHint: "点击发送或按 Ctrl+Enter 查看响应。",
	Sending: "正在发送请求…", ErrorTitle: "请求失败",
	Duration: "耗时", Size: "大小", Truncated: "响应内容已被截断。",
	Body: "正文", Headers: "响应头", Tests: "测试", TestsCount: "测试 (%d)",
	Pretty: "格式化", Raw: "原始", Copy: "复制正文", CopyError: "无法复制正文",
	DisplayLimit: "仅显示正文的前 2 MiB；复制按钮可复制完整的已接收正文。",
	HeaderName:   "名称", HeaderValue: "值", NoHeaders: "没有响应头。",
}

var responseEn = Response{
	Title: "Response", EmptyHint: "Press Send or Ctrl+Enter to view a response.",
	Sending: "Sending request…", ErrorTitle: "Request failed",
	Duration: "Time", Size: "Size", Truncated: "The response body was truncated.",
	Body: "Body", Headers: "Headers", Tests: "Tests", TestsCount: "Tests (%d)",
	Pretty: "Pretty", Raw: "Raw", Copy: "Copy body", CopyError: "Could not copy body",
	DisplayLimit: "Showing only the first 2 MiB of the body; Copy copies the full received body.",
	HeaderName:   "Name", HeaderValue: "Value", NoHeaders: "No response headers.",
}

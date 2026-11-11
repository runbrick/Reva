package i18n

// Response holds the strings of the response viewer. Owned by that feature's PR.
type Response struct {
	Title              string
	EmptyHint          string
	Sending            string
	ErrorTitle         string
	Duration           string
	Size               string
	Truncated          string
	Body               string
	Headers            string
	Tests              string
	TestsCount         string
	Pretty             string
	Raw                string
	Copy               string
	CopyError          string
	DisplayLimit       string
	HeaderName         string
	HeaderValue        string
	NoHeaders          string
	Cookies            string
	Timeline           string
	Preview            string
	Wrap               string
	CodeBody           string
	Search             string
	SearchHint         string
	NextMatch          string
	PreviousMatch      string
	CloseSearch        string
	MatchCount         string
	NoMatches          string
	NoCookies          string
	CookieDomain       string
	CookiePath         string
	CookieExpires      string
	CookieHTTPOnly     string
	CookieSecure       string
	Yes                string
	No                 string
	SessionCookie      string
	NoTimeline         string
	Total              string
	PhaseDNS           string
	PhaseConnect       string
	PhaseTLS           string
	PhaseSend          string
	PhaseWait          string
	PhaseDownload      string
	PhaseOther         string
	PhaseDuration      string
	PreviewUnavailable string
	EmptyPreview       string
}

var responseZh = Response{
	Title: "响应", EmptyHint: "点击发送或按 Ctrl+Enter 查看响应。",
	Sending: "正在发送请求…", ErrorTitle: "请求失败",
	Duration: "耗时", Size: "大小", Truncated: "响应内容已被截断。",
	Body: "响应结果", Headers: "响应头", Tests: "测试", TestsCount: "测试 (%d)",
	Pretty: "美化", Raw: "原生", Copy: "复制正文", CopyError: "无法复制正文",
	DisplayLimit: "仅显示正文的前 2 MiB；复制按钮可复制完整的已接收正文。",
	HeaderName:   "名称", HeaderValue: "值", NoHeaders: "没有响应头。",
	Cookies: "Cookie", Timeline: "时间线", Preview: "预览", Wrap: "自动换行", CodeBody: "响应正文",
	Search: "查找正文", SearchHint: "查找…", NextMatch: "下一个匹配", PreviousMatch: "上一个匹配", CloseSearch: "关闭查找", MatchCount: "%d / %d", NoMatches: "没有匹配项",
	NoCookies: "没有响应 Cookie。", CookieDomain: "域名", CookiePath: "路径", CookieExpires: "过期时间", CookieHTTPOnly: "HttpOnly", CookieSecure: "Secure",
	Yes: "是", No: "否", SessionCookie: "会话", NoTimeline: "没有时间线数据。", Total: "总耗时",
	PhaseDNS: "DNS 查询", PhaseConnect: "建立连接", PhaseTLS: "TLS 握手", PhaseSend: "发送请求", PhaseWait: "等待响应", PhaseDownload: "下载响应", PhaseOther: "其他",
	PhaseDuration:      "%.2f ms",
	PreviewUnavailable: "此响应无法预览。", EmptyPreview: "HTML 中没有可显示的文本。",
}

var responseEn = Response{
	Title: "Response", EmptyHint: "Press Send or Ctrl+Enter to view a response.",
	Sending: "Sending request…", ErrorTitle: "Request failed",
	Duration: "Time", Size: "Size", Truncated: "The response body was truncated.",
	Body: "Body", Headers: "Headers", Tests: "Tests", TestsCount: "Tests (%d)",
	Pretty: "Pretty", Raw: "Raw", Copy: "Copy body", CopyError: "Could not copy body",
	DisplayLimit: "Showing only the first 2 MiB of the body; Copy copies the full received body.",
	HeaderName:   "Name", HeaderValue: "Value", NoHeaders: "No response headers.",
	Cookies: "Cookies", Timeline: "Timeline", Preview: "Preview", Wrap: "Word wrap", CodeBody: "Response body",
	Search: "Find in body", SearchHint: "Find…", NextMatch: "Next match", PreviousMatch: "Previous match", CloseSearch: "Close find", MatchCount: "%d / %d", NoMatches: "No matches",
	NoCookies: "No response cookies.", CookieDomain: "Domain", CookiePath: "Path", CookieExpires: "Expires", CookieHTTPOnly: "HttpOnly", CookieSecure: "Secure",
	Yes: "Yes", No: "No", SessionCookie: "Session", NoTimeline: "No timeline data.", Total: "Total",
	PhaseDNS: "DNS lookup", PhaseConnect: "Connect", PhaseTLS: "TLS handshake", PhaseSend: "Send request", PhaseWait: "Wait for response", PhaseDownload: "Download", PhaseOther: "Other",
	PhaseDuration:      "%.2f ms",
	PreviewUnavailable: "Preview is not available for this response.", EmptyPreview: "This HTML has no visible text.",
}

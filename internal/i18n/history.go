package i18n

// History holds the strings of the History page.
type History struct {
	Title, Search, SearchPlaceholder            string
	Open, Delete, Clear                         string
	ConfirmTitle, ConfirmBody                   string
	ConfirmClear, Cancel                        string
	Empty, NoMatches                            string
	Today, Yesterday, Error                     string
	DateFormat, TimeFormat                      string
	ClearSearch, Method, Status                 string
	AllMethods, Other, AllStatuses              string
	Time, URL, Duration, Actions                string
	Detail, SelectEntry, Resend                 string
	Headers, Body, NoHeaders, NoBody            string
	BodyFields, BodyFile, BodyText              string
	CopyURL, CopyError                          string
	Success, Redirect, ClientError, ServerError string
}

var historyZh = History{
	Title: "历史", Search: "搜索历史", SearchPlaceholder: "按网址或方法搜索",
	Open: "打开请求", Delete: "删除记录", Clear: "清空历史",
	ConfirmTitle: "清空所有历史记录？", ConfirmBody: "所有历史记录都将被删除，此操作无法撤销。",
	ConfirmClear: "确认清空", Cancel: "取消",
	Empty: "暂无历史记录。发送请求后会显示在这里。", NoMatches: "没有匹配的历史记录。",
	Today: "今天", Yesterday: "昨天", Error: "错误",
	DateFormat: "2006-01-02", TimeFormat: "15:04",
	ClearSearch: "清除搜索", Method: "方法", Status: "状态",
	AllMethods: "全部方法", Other: "其他", AllStatuses: "全部状态",
	Time: "时间", URL: "网址", Duration: "耗时", Actions: "操作",
	Detail: "请求详情", SelectEntry: "选择一条历史记录查看详情", Resend: "重新发送",
	Headers: "请求头", Body: "请求体", NoHeaders: "无请求头", NoBody: "无请求体",
	BodyFields: "%d 个字段", BodyFile: "文件：%s", BodyText: "%s · %d 字节",
	CopyURL: "复制网址", CopyError: "复制失败",
	Success: "2xx", Redirect: "3xx", ClientError: "4xx", ServerError: "5xx",
}

var historyEn = History{
	Title: "History", Search: "Search history", SearchPlaceholder: "Search URL or method",
	Open: "Open request", Delete: "Delete entry", Clear: "Clear history",
	ConfirmTitle: "Clear all history?", ConfirmBody: "All history entries will be deleted. This cannot be undone.",
	ConfirmClear: "Confirm clear", Cancel: "Cancel",
	Empty: "No history yet. Sent requests will appear here.", NoMatches: "No matching history entries.",
	Today: "Today", Yesterday: "Yesterday", Error: "Error",
	DateFormat: "2006-01-02", TimeFormat: "15:04",
	ClearSearch: "Clear search", Method: "Method", Status: "Status",
	AllMethods: "All methods", Other: "Other", AllStatuses: "All statuses",
	Time: "Time", URL: "URL", Duration: "Duration", Actions: "Actions",
	Detail: "Request details", SelectEntry: "Select a history entry to view its details", Resend: "Re-send",
	Headers: "Request headers", Body: "Request body", NoHeaders: "No request headers", NoBody: "No request body",
	BodyFields: "%d fields", BodyFile: "File: %s", BodyText: "%s · %d bytes",
	CopyURL: "Copy URL", CopyError: "Copy failed",
	Success: "2xx", Redirect: "3xx", ClientError: "4xx", ServerError: "5xx",
}

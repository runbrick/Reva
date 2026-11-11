package i18n

// History holds the strings of the history sidebar. Owned by that feature's PR.
type History struct {
	Title, Search, SearchPlaceholder string
	Open, Delete, Clear              string
	ConfirmTitle, ConfirmBody        string
	ConfirmClear, Cancel             string
	Empty, NoMatches                 string
	Today, Yesterday, Error          string
	DateFormat, TimeFormat           string
}

var historyZh = History{
	Title: "历史", Search: "搜索历史", SearchPlaceholder: "按网址或方法搜索",
	Open: "打开请求", Delete: "删除记录", Clear: "清空历史",
	ConfirmTitle: "清空所有历史记录？", ConfirmBody: "所有历史记录都将被删除，此操作无法撤销。",
	ConfirmClear: "确认清空", Cancel: "取消",
	Empty: "暂无历史记录。发送请求后会显示在这里。", NoMatches: "没有匹配的历史记录。",
	Today: "今天", Yesterday: "昨天", Error: "错误",
	DateFormat: "2006-01-02", TimeFormat: "15:04",
}

var historyEn = History{
	Title: "History", Search: "Search history", SearchPlaceholder: "Search URL or method",
	Open: "Open request", Delete: "Delete entry", Clear: "Clear history",
	ConfirmTitle: "Clear all history?", ConfirmBody: "All history entries will be deleted. This cannot be undone.",
	ConfirmClear: "Confirm clear", Cancel: "Cancel",
	Empty: "No history yet. Sent requests will appear here.", NoMatches: "No matching history entries.",
	Today: "Today", Yesterday: "Yesterday", Error: "Error",
	DateFormat: "2006-01-02", TimeFormat: "15:04",
}

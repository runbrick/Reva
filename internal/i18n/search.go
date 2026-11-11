package i18n

// Search holds the strings of the global search and command palette. Owned
// by the search PR.
type Search struct {
	Title, Placeholder, Field, Empty, Results, Navigation string
	Requests, Tabs, History, Environments, Commands       string
	NewRequest, NewCollection, Import, Settings           string
	ToggleTheme, SwitchLanguage, Untitled                 string
	MacHint, OtherHint                                    string
}

var searchZh = Search{
	Title: "搜索", Placeholder: "搜索接口、集合或文档...", Field: "搜索接口与命令",
	Empty: "没有匹配的结果", Results: "搜索结果", Navigation: "↑ ↓ 选择 · Enter 打开 · Esc 关闭",
	Requests: "集合中的接口", Tabs: "已打开的标签页", History: "历史记录", Environments: "环境", Commands: "命令",
	NewRequest: "新建请求", NewCollection: "新建集合", Import: "导入", Settings: "设置",
	ToggleTheme: "切换主题", SwitchLanguage: "切换语言", Untitled: "未命名请求",
	MacHint: "⌘ K", OtherHint: "Ctrl K",
}

var searchEn = Search{
	Title: "Search", Placeholder: "Search APIs, collections or docs...", Field: "Search requests and commands",
	Empty: "No matching results", Results: "Search results", Navigation: "↑ ↓ Select · Enter Open · Esc Close",
	Requests: "Requests in collections", Tabs: "Open tabs", History: "History", Environments: "Environments", Commands: "Commands",
	NewRequest: "New request", NewCollection: "New collection", Import: "Import", Settings: "Settings",
	ToggleTheme: "Toggle theme", SwitchLanguage: "Switch language", Untitled: "Untitled request",
	MacHint: "⌘ K", OtherHint: "Ctrl K",
}

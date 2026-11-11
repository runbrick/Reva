package i18n

// Shell holds the strings of the window shell: the navigation rail and the
// page names. Owned by the shell (app.go); feature PRs do not edit it.
type Shell struct {
	// Rail items.
	APIs, Collections, Environments, Settings string
	// Titlebar navigation items.
	Workbench, History string
	// Accessible names of the navigation regions.
	RailLabel, NavLabel, SidebarLabel, SplitLabel string
}

var shellZh = Shell{
	APIs: "接口", Collections: "集合", Environments: "环境", Settings: "设置",
	Workbench: "工作台", History: "历史记录",
	RailLabel: "主导航", NavLabel: "页面导航", SidebarLabel: "调整侧栏宽度", SplitLabel: "调整请求与响应高度",
}

var shellEn = Shell{
	APIs: "APIs", Collections: "Collections", Environments: "Environments", Settings: "Settings",
	Workbench: "Workbench", History: "History",
	RailLabel: "Main navigation", NavLabel: "Page navigation", SidebarLabel: "Resize sidebar", SplitLabel: "Resize request and response",
}

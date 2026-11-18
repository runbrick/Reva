package i18n

// Titlebar holds the window controls' accessible strings.
type Titlebar struct {
	Title, Minimize, Maximize, Restore, Close string
	LightTheme, DarkTheme, NewRequestTab      string
}

var titlebarZh = Titlebar{
	Title: "标题栏", Minimize: "最小化", Maximize: "最大化", Restore: "还原", Close: "关闭",
	LightTheme: "切换到浅色主题", DarkTheme: "切换到深色主题", NewRequestTab: "打开新请求标签页",
}

var titlebarEn = Titlebar{
	Title: "Title bar", Minimize: "Minimize", Maximize: "Maximize", Restore: "Restore", Close: "Close",
	LightTheme: "Switch to light theme", DarkTheme: "Switch to dark theme", NewRequestTab: "Open a new request tab",
}

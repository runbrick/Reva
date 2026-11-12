package i18n

// Titlebar holds the window controls' accessible strings.
type Titlebar struct {
	Title, Minimize, Maximize, Restore, Close string
}

var titlebarZh = Titlebar{
	Title: "标题栏", Minimize: "最小化", Maximize: "最大化", Restore: "还原", Close: "关闭",
}

var titlebarEn = Titlebar{
	Title: "Title bar", Minimize: "Minimize", Maximize: "Maximize", Restore: "Restore", Close: "Close",
}

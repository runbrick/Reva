package i18n

// Titlebar holds the window controls and avatar menu's accessible strings.
type Titlebar struct {
	Title, Minimize, Maximize, Restore, Close string
	UserMenu, Settings, About                 string
	Light, Dark, System, Chinese, English     string
}

var titlebarZh = Titlebar{
	Title: "标题栏", Minimize: "最小化", Maximize: "最大化", Restore: "还原", Close: "关闭",
	UserMenu: "用户菜单", Settings: "设置", About: "关于",
	Light: "浅色", Dark: "深色", System: "跟随系统", Chinese: "中文", English: "English",
}

var titlebarEn = Titlebar{
	Title: "Title bar", Minimize: "Minimize", Maximize: "Maximize", Restore: "Restore", Close: "Close",
	UserMenu: "User menu", Settings: "Settings", About: "About",
	Light: "Light", Dark: "Dark", System: "Follow system", Chinese: "中文", English: "English",
}

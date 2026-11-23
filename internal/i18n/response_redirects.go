package i18n

// ResponseRedirects holds labels for the Timeline's redirect diagnostics.
type ResponseRedirects struct {
	Title, Copy, CopyError, Hop, MethodChange, PlannedMethod, HiddenMethod, NoOrigin string
	Outcome, AuthRemoved, AuthNotRemoved, Followed, Disabled, Limit, Policy          string
	MissingTarget, MalformedTarget, NotSent, TransportError                          string
}

func ResponseRedirectsFor(lang Lang) ResponseRedirects {
	if lang == En {
		return responseRedirectsEn
	}
	return responseRedirectsZh
}

var responseRedirectsZh = ResponseRedirects{
	Title: "重定向链", Copy: "复制诊断摘要", CopyError: "无法复制重定向诊断摘要",
	Hop: "第 %d 跳 · %s · %d · %.2f ms", MethodChange: "%s → %s", PlannedMethod: "%s → %s（计划）",
	HiddenMethod: "方法已隐藏", NoOrigin: "目标源不可显示", Outcome: "%s · %s",
	AuthRemoved: "已移除认证", AuthNotRemoved: "未移除认证", Followed: "已跟随",
	Disabled: "未跟随（已禁用重定向）", Limit: "未跟随（已达跳数上限）", Policy: "未跟随（重定向策略拒绝）",
	MissingTarget: "未跟随（缺少 Location）", MalformedTarget: "未跟随（Location 格式无效）",
	NotSent: "未跟随（下一跳未进入传输）", TransportError: "下一跳传输失败，未收到响应",
}

var responseRedirectsEn = ResponseRedirects{
	Title: "Redirect chain", Copy: "Copy diagnostics", CopyError: "Could not copy redirect diagnostics",
	Hop: "Hop %d · %s · %d · %.2f ms", MethodChange: "%s → %s", PlannedMethod: "%s → %s (planned)",
	HiddenMethod: "Method hidden", NoOrigin: "Target origin unavailable", Outcome: "%s · %s",
	AuthRemoved: "Auth removed", AuthNotRemoved: "Auth not removed", Followed: "Followed",
	Disabled: "Not followed (redirects disabled)", Limit: "Not followed (hop limit reached)", Policy: "Not followed (redirect policy declined)",
	MissingTarget: "Not followed (Location missing)", MalformedTarget: "Not followed (malformed Location)",
	NotSent: "Not followed (next request did not reach transport)", TransportError: "Next hop failed without a response",
}

package i18n

type SOAP struct {
	NewRequest, New12Request, Action, ActionOverridden string
}

var soapZh = SOAP{
	NewRequest:       "新建 SOAP 请求",
	New12Request:     "新建 SOAP 1.2 请求",
	Action:           "SOAP 操作",
	ActionOverridden: "SOAP 操作已被 Content-Type 请求头覆盖。",
}

var soapEn = SOAP{
	NewRequest:       "New SOAP Request",
	New12Request:     "New SOAP 1.2 Request",
	Action:           "SOAP Action",
	ActionOverridden: "SOAP Action is overridden by the Content-Type header.",
}

package gui

import (
	"mime"
	"strconv"
	"strings"

	"github.com/runbrick/reva/internal/gui/icons"
	"github.com/runbrick/reva/internal/model"
	"github.com/runbrick/reva/internal/storage"
	"github.com/vivaui/viva/ui"
)

const (
	soapActionHeader = "SOAPAction"
	soap11MediaType  = "text/xml"
	soap12MediaType  = "application/soap+xml"
)

type soapVersion uint8

const (
	soapNone soapVersion = iota
	soap11
	soap12
)

func (a *App) soapMenuItems(c ui.C, menu *ui.MenuItems, node collectionNode) {
	if !node.branch {
		node.id = node.parentID
	}
	if menu.Item(a.S.SOAP.NewRequest).Icon(icons.Package).Clicked() {
		c.After(func() { a.newSOAPRequest(node, soap11) })
	}
	if menu.Item(a.S.SOAP.New12Request).Icon(icons.Package).Clicked() {
		c.After(func() { a.newSOAPRequest(node, soap12) })
	}
}

func (a *App) newSOAPRequest(node collectionNode, version soapVersion) {
	if node.collectionID == "" {
		a.newCollectionTab()
		tab := a.Active()
		a.initSOAPRequest(&tab.Request, version)
		tab.Dirty = true
		return
	}
	req := model.NewRequest(storage.NewID())
	req.Auth.Type = model.AuthInherit
	a.initSOAPRequest(&req, version)
	a.saveNewCollectionRequest(node, req)
}

func (a *App) initSOAPRequest(req *model.Request, version soapVersion) {
	name, mediaType, namespace := a.S.SOAP.NewRequest, soap11MediaType, "http://schemas.xmlsoap.org/soap/envelope/"
	if version == soap12 {
		name, mediaType, namespace = a.S.SOAP.New12Request, soap12MediaType, "http://www.w3.org/2003/05/soap-envelope"
	} else {
		req.Headers = append(req.Headers, model.KV{Key: soapActionHeader, Disabled: true})
	}
	req.Name, req.Method = name, model.MethodPost
	req.Body = model.Body{
		Mode:        model.BodyRaw,
		ContentType: mediaType + "; charset=utf-8",
		Text: `<?xml version="1.0" encoding="UTF-8"?>
<soap:Envelope xmlns:soap="` + namespace + `">
  <soap:Body>
  </soap:Body>
</soap:Envelope>`,
	}
}

func soapBodyInfo(req *model.Request) (soapVersion, map[string]string) {
	if req.Body.Mode != model.BodyRaw {
		return soapNone, nil
	}
	mediaType, params, err := mime.ParseMediaType(req.Body.ContentType)
	if err != nil {
		return soapNone, nil
	}
	switch mediaType {
	case soap12MediaType:
		return soap12, params
	case soap11MediaType:
		if soapActionHeaderIndex(req.Headers) >= 0 {
			return soap11, nil
		}
	}
	return soapNone, nil
}

func soapActionHeaderIndex(headers []model.KV) int {
	disabled := -1
	for index, header := range headers {
		if strings.EqualFold(header.Key, soapActionHeader) {
			if !header.Disabled {
				return index
			}
			if disabled < 0 {
				disabled = index
			}
		}
	}
	return disabled
}

func (a *App) soapBodyControls(c ui.C, tab *Tab) {
	req := &tab.Request
	labels := a.S.SOAP
	version, params := soapBodyInfo(req)
	switch version {
	case soap11:
		ui.Text(c, "SOAP 1.1").Bold()
		header := &req.Headers[soapActionHeaderIndex(req.Headers)]
		var action string
		if !header.Disabled {
			action = header.Value
			if value, err := strconv.Unquote(action); err == nil && strings.HasPrefix(action, `"`) {
				action = value
			}
		}
		if ui.TextField(c.Key("soap11-action"), &action, labels.Action, "").FillWidth().Changed() {
			action = strings.TrimSpace(action)
			header.Value, header.Disabled = "", action == ""
			if !header.Disabled {
				header.Value = strconv.Quote(action)
			}
			tab.Dirty = true
		}
	case soap12:
		ui.Text(c, "SOAP 1.2").Bold()
		action := params["action"]
		if ui.TextField(c.Key("soap12-action"), &action, labels.Action, "").FillWidth().Changed() {
			delete(params, "action")
			req.Body.ContentType = mime.FormatMediaType(soap12MediaType, params)
			if action = strings.TrimSpace(action); action != "" {
				req.Body.ContentType += "; action=" + strconv.Quote(action)
			}
			tab.Dirty = true
		}
	}
}

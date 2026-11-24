package gui

import (
	"encoding/xml"
	"io"
	"mime"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/runbrick/reva/internal/model"
	"github.com/runbrick/reva/internal/service/fake"
	"github.com/runbrick/reva/internal/service/request"
	"github.com/runbrick/reva/internal/service/store"
	"github.com/runbrick/reva/internal/storage"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

func TestSOAPTemplatesSendXMLWithVersionSpecificAction(t *testing.T) {
	for _, test := range []struct {
		label, version, namespace, mediaType string
	}{
		{"New SOAP Request", "SOAP 1.1", "http://schemas.xmlsoap.org/soap/envelope/", "text/xml"},
		{"New SOAP 1.2 Request", "SOAP 1.2", "http://www.w3.org/2003/05/soap-envelope", "application/soap+xml"},
	} {
		t.Run(test.version, func(t *testing.T) {
			type exchange struct {
				method string
				header http.Header
				body   string
			}
			calls := make(chan exchange, 3)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				body, err := io.ReadAll(req.Body)
				if err != nil {
					t.Error(err)
				}
				calls <- exchange{req.Method, req.Header.Clone(), string(body)}
				w.Header().Set("Content-Type", "text/xml")
				io.WriteString(w, "<result>ok</result>")
			}))
			defer server.Close()
			svc := fake.New()
			svc.Requests = request.New()
			env, err := svc.Environments.Create("SOAP variables")
			if err != nil {
				t.Fatal(err)
			}
			env.Variables = []model.KV{{Key: "operation", Value: "echo"}}
			if err := svc.Environments.Save(env); err != nil {
				t.Fatal(err)
			}
			settings := svc.Settings.Get()
			settings.ActiveEnvID = env.ID
			if err := svc.Settings.Save(settings); err != nil {
				t.Fatal(err)
			}
			h := newHarness(t, svc)
			requestEnglish(h)
			h.click(h.App.S.Collections.Add)
			collectionsOption(h, test.label)
			tab := h.App.Active()
			body := tab.Request.Body
			var envelope struct {
				XMLName xml.Name
				Body    *struct{} `xml:"Body"`
			}
			if err := xml.Unmarshal([]byte(body.Text), &envelope); err != nil || envelope.XMLName != (xml.Name{Space: test.namespace, Local: "Envelope"}) || envelope.Body == nil {
				t.Fatalf("default envelope is invalid: %q (%v)", body.Text, err)
			}
			if tab.CollectionID != "" || !tab.Dirty || tab.Request.Method != model.MethodPost || body.Mode != model.BodyRaw || !h.shows(test.version) {
				t.Fatalf("new SOAP editor = %+v, controls = %q", tab.Request, h.hitLabels())
			}
			h.typeInto(h.App.S.Request.URL, server.URL)
			send := func(wantBody, wantAction string) {
				t.Helper()
				h.click(h.App.S.Request.Send)
				h.waitFor("SOAP response", func() bool { return !tab.Sending && tab.Response != nil })
				if tab.Response.StatusCode != http.StatusOK || string(tab.Response.Body) != "<result>ok</result>" {
					t.Fatalf("response = %+v", tab.Response)
				}
				select {
				case call := <-calls:
					mediaType, params, err := mime.ParseMediaType(call.header.Get("Content-Type"))
					if err != nil || mediaType != test.mediaType || params["charset"] != "utf-8" || call.method != http.MethodPost || call.body != wantBody {
						t.Fatalf("SOAP HTTP request = %+v (media type error %v)", call, err)
					}
					if test.version == "SOAP 1.1" && wantAction != "" {
						if got := call.header.Values("SOAPAction"); len(got) != 1 || got[0] != `"`+wantAction+`"` || params["action"] != "" {
							t.Fatalf("SOAP 1.1 action headers = %v", call.header)
						}
					} else if params["action"] != wantAction || len(call.header.Values("SOAPAction")) != 0 {
						t.Fatalf("unexpected action headers = %v", call.header)
					}
				default:
					t.Fatal("no SOAP request reached the server")
				}
			}
			send(body.Text, "")
			edited := strings.Replace(body.Text, "</soap:Body>", "  <operation>{{operation}}</operation>\n  </soap:Body>", 1)
			if edited == body.Text {
				t.Fatalf("no editable body element in %q", body.Text)
			}
			requestType(h, h.App.S.Request.BodyText, edited, true)
			requestType(h, "SOAP Action", "urn:services:{{operation}}", true)
			send(strings.ReplaceAll(edited, "{{operation}}", "echo"), "urn:services:echo")
			h.click("SOAP Action")
			if _, err := h.D.ShortcutChord(ui.Primary, 'a'); err != nil {
				t.Fatal(err)
			}
			if err := h.D.Key(gfx.KeyBackspace, 0); err != nil {
				t.Fatal(err)
			}
			send(strings.ReplaceAll(edited, "{{operation}}", "echo"), "")
		})
	}
}

func TestSOAPFolderRequestSavesAndReopens(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(storage.EnvHome, dir)
	disk, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	svc := fake.New()
	svc.Collections = disk.Collections()
	col, err := svc.Collections.Create("API")
	if err != nil {
		t.Fatal(err)
	}
	folder, err := svc.Collections.AddFolder(col.ID, "", "SOAP services")
	if err != nil {
		t.Fatal(err)
	}
	h := newHarness(t, svc)
	requestEnglish(h)
	const label = "New SOAP 1.2 Request"
	collectionsContext(h, col.Name, "New SOAP Request")
	root := h.App.Active().Request.ID
	collectionsContext(h, folder.Name, label)
	tab := h.App.Active()
	if tab.CollectionID != col.ID || tab.Dirty || tab.Request.Auth.Type != model.AuthInherit {
		t.Fatalf("folder request = %+v", tab)
	}
	requestType(h, h.App.S.Request.ContentType, "application/soap+xml; charset=utf-8; profile=custom", true)
	requestType(h, "SOAP Action", "urn:lookup", true)
	tab.Request.Headers = []model.KV{{Key: "X-Client", Value: "custom"}}
	h.typeInto(h.App.S.Request.URL, "https://example.test/soap")
	h.click(h.App.S.Request.Save)
	if tab.Dirty {
		t.Fatal("SOAP edits were not saved")
	}
	want := tab.Request.Clone()
	disk, err = store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	svc.Collections = disk.Collections()
	stored, ok := svc.Collections.Get(col.ID)
	if parent := model.FindItem(stored.Items, folder.ID); !ok || parent == nil || len(parent.Items) != 1 || parent.Items[0].ID != want.ID {
		t.Fatalf("SOAP request was not saved in its folder: %+v", stored)
	}
	if len(stored.Items) != 2 || stored.Items[1].ID != root || stored.Items[1].Request == nil {
		t.Fatalf("SOAP request was not saved at collection root: %+v", stored.Items)
	}
	reopened := newHarness(t, svc)
	requestEnglish(reopened)
	reopened.click(label)
	got := reopened.App.Active().Request
	if got.ID != want.ID || got.URL != want.URL || got.Body.Mode != model.BodyRaw || got.Body.ContentType != want.Body.ContentType {
		t.Fatalf("request did not reopen: %+v", got)
	}
	_, params, err := mime.ParseMediaType(got.Body.ContentType)
	if got.Body.Text != want.Body.Text || err != nil || params["action"] != "urn:lookup" || params["profile"] != "custom" || !reopened.shows("SOAP 1.2") {
		t.Fatalf("saved SOAP body editor = %+v; controls = %q", got.Body, reopened.hitLabels())
	}
	reopened.click(reopened.App.S.Collections.Add)
	collectionsOption(reopened, "New SOAP Request")
	siblingID := reopened.App.Active().Request.ID
	stored, _ = svc.Collections.Get(col.ID)
	parent := model.FindItem(stored.Items, folder.ID)
	if siblingID == want.ID || parent == nil || len(parent.Items) != 2 || parent.Items[1].ID != siblingID || !slices.Equal(parent.Items[0].Request.Headers, want.Headers) {
		t.Fatalf("SOAP sibling creation changed an existing request or used the wrong parent: %+v", parent)
	}
}

func TestSOAPDoesNotChangeOrdinaryRawXML(t *testing.T) {
	h := newHarness(t)
	requestEnglish(h)
	tab := h.App.Active()
	tab.Request.Body = model.Body{Mode: model.BodyRaw, ContentType: "text/xml; charset=utf-8", Text: "<note/>"}
	h.click(h.App.S.Request.Body)
	if h.shows("SOAP 1.1") || h.shows("SOAP 1.2") {
		t.Fatalf("unexpected ordinary raw editor controls: %q", h.hitLabels())
	}
	requestField(h, h.App.S.Request.ContentType, 0)
	requestType(h, h.App.S.Request.BodyText, "<other/>", true)
	if tab.Request.Body.Text != "<other/>" || tab.Request.Body.ContentType != "text/xml; charset=utf-8" {
		t.Fatalf("ordinary raw XML request was modified: %+v", tab.Request.Body)
	}
}

package main

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBuilderIntakeNavigationKeepsOriginalShellAndDefaultOff(t *testing.T) {
	for _, tc := range []struct {
		name    string
		enabled bool
		user    string
		visible bool
	}{
		{"disabled", false, users["alice"], false},
		{"anonymous", true, "", false},
		{"enabled-signed-in", true, users["alice"], true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := &Service{cfg: Config{PublicURL: "/credit", BuilderHubEnabled: tc.enabled}}
			w := httptest.NewRecorder()
			s.render(w, "<h1>Original body</h1>", map[string]any{
				"Title": "Original shell", "User": tc.user, "Path": "/ui/",
				// Request/template data cannot enable a disabled deployment.
				"BuilderHubEnabled": true,
			})
			body := w.Body.String()
			if w.Code != 200 || !strings.Contains(body, "Original body") || !strings.Contains(body, "graph-flow.js") {
				t.Fatal("original shell or asset changed")
			}
			if strings.Contains(body, `href="/credit/ui/builder"`) != tc.visible {
				t.Fatal("Builder navigation must follow deployment and login state")
			}
		})
	}
}

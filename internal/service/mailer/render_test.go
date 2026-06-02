package mailer

import (
	htmltemplate "html/template"
	"strings"
	"testing"
)

// TestRenderHTMLTemplateEscapesData pins that the HTML email body escapes data
// interpolations — an IdP-controlled DisplayName / UPN can carry markup, and a
// text/template body let it inject into the email. Fields explicitly typed as
// html/template.HTML (e.g. the pre-escaped AnnouncementBodyHTML) pass through.
func TestRenderHTMLTemplateEscapesData(t *testing.T) {
	out, err := renderHTMLTemplate("t",
		`<p>Hi {{.DisplayName}}</p>{{.AnnouncementBodyHTML}}`,
		map[string]any{
			"DisplayName":          `<script>alert(1)</script>`,
			"AnnouncementBodyHTML": htmltemplate.HTML("<p>safe<br>html</p>"),
		})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if strings.Contains(out, "<script>") {
		t.Errorf("DisplayName not escaped — injection survived: %q", out)
	}
	if !strings.Contains(out, "&lt;script&gt;") {
		t.Errorf("expected escaped script tag, got %q", out)
	}
	if !strings.Contains(out, "<p>safe<br>html</p>") {
		t.Errorf("template.HTML field must pass through unescaped, got %q", out)
	}
}

// TestRenderTemplatePlainSubjectNotEscaped pins that the subject renderer stays
// plain text (no HTML escaping) — subjects are header text, escaping would
// surface literal &lt; / &amp; in them.
func TestRenderTemplatePlainSubjectNotEscaped(t *testing.T) {
	out, err := renderTemplate("s", `Alert for {{.Name}}`, map[string]any{"Name": "A & B <x>"})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if out != "Alert for A & B <x>" {
		t.Errorf("subject should be raw text, got %q", out)
	}
}

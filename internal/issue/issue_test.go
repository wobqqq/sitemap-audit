package issue

import (
	"encoding/json"
	"testing"
)

func TestCatalogueIsConsistent(t *testing.T) {
	groups := map[Group]bool{}
	for _, g := range Groups() {
		groups[g] = true
	}
	seen := map[Code]bool{}
	for _, d := range Catalogue() {
		if seen[d.Code] {
			t.Errorf("%s is defined twice", d.Code)
		}
		seen[d.Code] = true
		if !groups[d.Group] {
			t.Errorf("%s has unknown group %q", d.Code, d.Group)
		}
		if d.Severity < Notice || d.Severity > Error {
			t.Errorf("%s has no severity", d.Code)
		}
		if d.Scope != ScopeSite && d.Scope != ScopeSitemap && d.Scope != ScopeURL {
			t.Errorf("%s has no scope", d.Code)
		}
		if d.Title == "" || d.Description == "" {
			t.Errorf("%s needs a title and a description", d.Code)
		}
		if got, ok := Lookup(d.Code); !ok || got.Code != d.Code {
			t.Errorf("Lookup(%s) failed", d.Code)
		}
	}
	if Known("NOPE") {
		t.Error("unknown code reported as known")
	}
}

func TestSeverity(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want Severity
	}{{"notice", Notice}, {" Warning ", Warning}, {"ERROR", Error}} {
		got, err := ParseSeverity(tc.in)
		if err != nil || got != tc.want {
			t.Errorf("ParseSeverity(%q) = %v, %v", tc.in, got, err)
		}
	}
	if _, err := ParseSeverity("fatal"); err == nil {
		t.Error("expected an error for an unknown severity")
	}
	if Severity(0).String() != "none" {
		t.Error("zero severity should print as none")
	}
	b, err := json.Marshal(Issue{Code: Soft404, Severity: Warning})
	if err != nil || string(b) != `{"code":"SOFT_404","severity":"warning","message":""}` {
		t.Errorf("marshal = %s, %v", b, err)
	}
	var s Severity
	if err := json.Unmarshal([]byte(`"error"`), &s); err != nil || s != Error {
		t.Errorf("unmarshal = %v, %v", s, err)
	}
	if err := json.Unmarshal([]byte(`"bad"`), &s); err == nil {
		t.Error("expected an unmarshal error")
	}
}

func TestPolicy(t *testing.T) {
	p := DefaultPolicy()
	if !p.Enabled(Soft404) || !p.GroupEnabled(GroupSoft404) {
		t.Fatal("everything is enabled by default")
	}
	p.Groups[GroupSoft404] = false
	if p.Enabled(Soft404) || p.GroupEnabled(GroupSoft404) {
		t.Error("a disabled group must disable its codes")
	}
	p.Disabled[Noindex] = true
	if p.Enabled(Noindex) {
		t.Error("a disabled code must not be reported")
	}
	if p.Enabled("UNKNOWN") {
		t.Error("an unknown code must not be reported")
	}
	p.Override[TrailingSlash] = Error
	i, ok := p.New(TrailingSlash, "ends with %s", "/")
	if !ok || i.Severity != Error || i.Message != "ends with /" {
		t.Errorf("New = %+v, %v", i, ok)
	}
	if _, ok := p.New(Noindex, "x"); ok {
		t.Error("New must refuse a disabled code")
	}
	plain, _ := p.New(URLRedirect, "plain text")
	if plain.Message != "plain text" {
		t.Errorf("a message without arguments must stay as is, got %q", plain.Message)
	}
	empty := Policy{}
	if !empty.Enabled(Soft404) || !empty.GroupEnabled(GroupSoft404) {
		t.Error("an empty policy enables everything")
	}
}

func TestListHelpers(t *testing.T) {
	list := []Issue{
		{Code: TrailingSlash, Severity: Notice},
		{Code: Soft404, Severity: Error},
		{Code: TrailingSlash, Severity: Notice},
		{Code: URLRedirect, Severity: Warning},
	}
	if Max(list) != Error || Max(nil) != 0 {
		t.Error("Max is wrong")
	}
	if got := Codes(list); len(got) != 3 || got[0] != TrailingSlash {
		t.Errorf("Codes = %v", got)
	}
	SortBySeverity(list)
	if list[0].Code != Soft404 || list[1].Code != URLRedirect {
		t.Errorf("SortBySeverity = %v", list)
	}
}

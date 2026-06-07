package projects

import "testing"

func TestParseRepo(t *testing.T) {
	owner, name, err := ParseRepo("NSExceptional/home")
	if err != nil || owner != "NSExceptional" || name != "home" {
		t.Fatalf("got (%q,%q,%v)", owner, name, err)
	}
	for _, bad := range []string{"", "noslash", "/name", "owner/"} {
		if _, _, err := ParseRepo(bad); err == nil {
			t.Errorf("ParseRepo(%q) expected error", bad)
		}
	}
}

func TestOptionByName(t *testing.T) {
	f := Field{Name: "Status", Options: []Option{{ID: "1", Name: "Todo"}, {ID: "2", Name: "Done"}}}
	o, err := f.OptionByName("done") // case-insensitive
	if err != nil || o.ID != "2" {
		t.Fatalf("got (%+v,%v)", o, err)
	}
	if _, err := f.OptionByName("nope"); err == nil {
		t.Error("expected error for unknown option")
	}
}

func TestFieldValueInput(t *testing.T) {
	tests := []struct {
		name     string
		field    Field
		value    string
		wantFrag string
		wantVar  any
		wantErr  bool
	}{
		{"single-select", Field{DataType: "SINGLE_SELECT", Options: []Option{{ID: "opt1", Name: "Todo"}}}, "todo", "{singleSelectOptionId:$v}", "opt1", false},
		{"single-select unknown", Field{DataType: "SINGLE_SELECT", Options: []Option{{ID: "opt1", Name: "Todo"}}}, "nope", "", nil, true},
		{"text", Field{DataType: "TEXT"}, "hello", "{text:$v}", "hello", false},
		{"number", Field{DataType: "NUMBER"}, "42", "{number:$v}", 42.0, false},
		{"number invalid", Field{DataType: "NUMBER"}, "abc", "", nil, true},
		{"date", Field{DataType: "DATE"}, "2026-01-15", "{date:$v}", "2026-01-15", false},
		{"iteration", Field{DataType: "ITERATION", Iterations: []Iteration{{ID: "it1", Title: "Sprint 1"}}}, "sprint 1", "{iterationId:$v}", "it1", false},
		{"iteration unknown", Field{DataType: "ITERATION"}, "nope", "", nil, true},
		{"unsupported", Field{DataType: "ASSIGNEES"}, "x", "", nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			frag, vars, err := fieldValueInput(tt.field, tt.value)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if frag != tt.wantFrag {
				t.Errorf("frag = %q, want %q", frag, tt.wantFrag)
			}
			if vars["v"] != tt.wantVar {
				t.Errorf("var v = %#v, want %#v", vars["v"], tt.wantVar)
			}
		})
	}
}

func TestValVarType(t *testing.T) {
	cases := map[string]string{"NUMBER": "Float!", "DATE": "Date!", "TEXT": "String!", "SINGLE_SELECT": "String!"}
	for dt, want := range cases {
		if got := valVarType(dt); got != want {
			t.Errorf("valVarType(%q) = %q, want %q", dt, got, want)
		}
	}
}

func TestIssueURLRegex(t *testing.T) {
	cases := []struct {
		in          string
		owner, repo string
		number      string
		shouldMatch bool
	}{
		{"https://github.com/NSExceptional/home/issues/42", "NSExceptional", "home", "42", true},
		{"https://github.com/o/r/pull/7", "o", "r", "7", true},
		{"not a url", "", "", "", false},
		{"https://github.com/o/r", "", "", "", false},
	}
	for _, c := range cases {
		m := issueURLRe.FindStringSubmatch(c.in)
		if !c.shouldMatch {
			if m != nil {
				t.Errorf("%q unexpectedly matched", c.in)
			}
			continue
		}
		if m == nil {
			t.Errorf("%q did not match", c.in)
			continue
		}
		if m[1] != c.owner || m[2] != c.repo || m[3] != c.number {
			t.Errorf("%q -> (%q,%q,%q), want (%q,%q,%q)", c.in, m[1], m[2], m[3], c.owner, c.repo, c.number)
		}
	}
}

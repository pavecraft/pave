package project

import (
	"reflect"
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		in   string
		want []Feature
	}{
		{
			name: "empty",
			in:   "",
			want: nil,
		},
		{
			name: "no checkboxes",
			in:   "# Features\n\nSome prose with no items.\n",
			want: nil,
		},
		{
			name: "single pending",
			in:   "- [ ] Config loader\n",
			want: []Feature{
				{ID: "config-loader", Title: "Config loader", Status: StatusPending},
			},
		},
		{
			name: "single implemented",
			in:   "- [x] Config loader\n",
			want: []Feature{
				{ID: "config-loader", Title: "Config loader", Status: StatusImplemented},
			},
		},
		{
			name: "uppercase X",
			in:   "- [X] Done thing\n",
			want: []Feature{
				{ID: "done-thing", Title: "Done thing", Status: StatusImplemented},
			},
		},
		{
			name: "mixed with asterisk bullets",
			in:   "* [ ] Alpha\n* [x] Beta\n",
			want: []Feature{
				{ID: "alpha", Title: "Alpha", Status: StatusPending},
				{ID: "beta", Title: "Beta", Status: StatusImplemented},
			},
		},
		{
			name: "em dash description",
			in:   "- [ ] Config loader — Load and validate pave.yaml\n",
			want: []Feature{
				{ID: "config-loader", Title: "Config loader", Description: "Load and validate pave.yaml", Status: StatusPending},
			},
		},
		{
			name: "colon description",
			in:   "- [ ] Parser: reads markdown\n",
			want: []Feature{
				{ID: "parser", Title: "Parser", Description: "reads markdown", Status: StatusPending},
			},
		},
		{
			name: "metadata priority and depends",
			in:   "- [ ] Run loop — orchestrate (priority: 2, depends: f01, f02)\n",
			want: []Feature{
				{
					ID:          "run-loop",
					Title:       "Run loop",
					Description: "orchestrate",
					Status:      StatusPending,
					Priority:    2,
					DependsOn:   []string{"f01", "f02"},
				},
			},
		},
		{
			name: "indented and surrounded by prose",
			in:   "# Plan\n\nIntro.\n\n  - [ ] Indented item\n\nOutro.\n",
			want: []Feature{
				{ID: "indented-item", Title: "Indented item", Status: StatusPending},
			},
		},
		{
			name: "malformed checkbox ignored",
			in:   "- [] missing space\n- [ ]\n- [ ] Valid\n",
			want: []Feature{
				{ID: "valid", Title: "Valid", Status: StatusPending},
			},
		},
		{
			name: "duplicate titles get suffixes",
			in:   "- [ ] Same\n- [ ] Same\n",
			want: []Feature{
				{ID: "same", Title: "Same", Status: StatusPending},
				{ID: "same-2", Title: "Same", Status: StatusPending},
			},
		},
		{
			name: "symbol-only title skipped",
			in:   "- [ ] !!!\n- [ ] Real\n",
			want: []Feature{
				{ID: "real", Title: "Real", Status: StatusPending},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := Parse(strings.NewReader(tt.in))
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Parse() =\n  %#v\nwant\n  %#v", got, tt.want)
			}
		})
	}
}

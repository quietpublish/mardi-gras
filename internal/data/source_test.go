package data

import "testing"

func TestSourceLabelJSONL(t *testing.T) {
	tests := []struct {
		name string
		src  Source
		want string
	}{
		{
			name: "JSONL with path",
			src:  Source{Mode: SourceJSONL, Path: "/foo/.beads/issues.jsonl"},
			want: "issues.jsonl",
		},
		{
			name: "JSONL empty path",
			src:  Source{Mode: SourceJSONL},
			want: "issues.jsonl",
		},
		{
			name: "CLI mode",
			src:  Source{Mode: SourceCLI},
			want: "bd list",
		},
		{
			name: "CLI mode ignores path",
			src:  Source{Mode: SourceCLI, Path: "/foo/bar"},
			want: "bd list",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.src.Label()
			if got != tt.want {
				t.Errorf("Source.Label() = %q, want %q", got, tt.want)
			}
		})
	}
}

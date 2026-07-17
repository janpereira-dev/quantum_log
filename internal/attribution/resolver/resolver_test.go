package resolver

import "testing"

func TestResolvePrecedence(t *testing.T) {
	t.Parallel()

	registered := map[string]string{"C:/repos/a": "project-a"}
	tests := []struct {
		name  string
		input Input
		want  ProjectResolution
	}{
		{
			name:  "explicit wins over environment",
			input: Input{ExplicitProject: "project-explicit", EnvironmentProject: "project-env", CWD: "C:/repos/a"},
			want:  ProjectResolution{ProjectSlug: "project-explicit", Method: Explicit, Confidence: Exact},
		},
		{
			name:  "explicit wins over adapter",
			input: Input{ExplicitProject: "project-explicit", AdapterProject: "project-adapter"},
			want:  ProjectResolution{ProjectSlug: "project-explicit", Method: Explicit, Confidence: Exact},
		},
		{
			name:  "adapter signal wins over environment",
			input: Input{AdapterProject: "project-adapter", EnvironmentProject: "project-env"},
			want:  ProjectResolution{ProjectSlug: "project-adapter", Method: Adapter, Confidence: High},
		},
		{
			name:  "environment wins over working directory",
			input: Input{EnvironmentProject: "project-env", CWD: "C:/repos/a"},
			want:  ProjectResolution{ProjectSlug: "project-env", Method: Environment, Confidence: High},
		},
		{
			name:  "working directory resolves registered path",
			input: Input{CWD: "C:/repos/a/subdir"},
			want:  ProjectResolution{ProjectSlug: "project-a", Method: CWD, Confidence: High},
		},
		{
			name:  "no evidence stays unattributed",
			input: Input{CWD: "C:/elsewhere"},
			want:  ProjectResolution{Method: Unresolved, Confidence: Unknown},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Resolve(tt.input, registered)
			if got.ProjectSlug != tt.want.ProjectSlug || got.Method != tt.want.Method || got.Confidence != tt.want.Confidence {
				t.Fatalf("Resolve() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

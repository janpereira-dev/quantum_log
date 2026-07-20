package resolver

import "testing"

func TestResolvePrecedence(t *testing.T) {
	t.Parallel()

	registered := map[string]string{"C:/repos/a": "project-a"}
	tests := []struct {
		name       string
		input      Input
		wantSlug   string
		want       Method
		confidence Confidence
		evidence   string
	}{
		{
			name:       "explicit beats environment",
			input:      Input{ExplicitProject: "project-explicit", EnvironmentProject: "project-env", CWD: "C:/repos/a"},
			wantSlug:   "project-explicit",
			want:       Explicit,
			confidence: Exact,
			evidence:   "explicit project",
		},
		{
			name:       "environment beats cwd",
			input:      Input{EnvironmentProject: "project-env", CWD: "C:/repos/a"},
			wantSlug:   "project-env",
			want:       Environment,
			confidence: High,
			evidence:   "QLOG_PROJECT",
		},
		{
			name:       "cwd beats git root",
			input:      Input{CWD: "C:/repos/a", GitRoot: "C:/repos/b"},
			wantSlug:   "project-a",
			want:       CWD,
			confidence: High,
			evidence:   "c:/repos/a",
		},
		{
			name:       "git root beats registered path fallback",
			input:      Input{CWD: "C:/elsewhere", GitRoot: "C:/repos/a", AdapterProject: "project-adapter"},
			wantSlug:   "project-a",
			want:       GitRoot,
			confidence: High,
			evidence:   "c:/repos/a",
		},
		{
			name:       "registered path beats adapter hint",
			input:      Input{CWD: "C:/repos/a/nested", AdapterProject: "project-adapter"},
			wantSlug:   "project-a",
			want:       Path,
			confidence: High,
			evidence:   "c:/repos/a",
		},
		{
			name:       "adapter is final hint",
			input:      Input{AdapterProject: "project-adapter"},
			wantSlug:   "project-adapter",
			want:       Adapter,
			confidence: High,
			evidence:   "adapter project signal",
		},
		{
			name:       "unknown is unattributed",
			input:      Input{},
			want:       Unresolved,
			confidence: Unknown,
			evidence:   "no project evidence",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Resolve(tt.input, registered)
			if got.ProjectSlug != tt.wantSlug || got.Method != tt.want || got.Confidence != tt.confidence || got.Evidence != tt.evidence {
				t.Fatalf("Resolve() = %#v, want slug=%q method=%q confidence=%q evidence=%q", got, tt.wantSlug, tt.want, tt.confidence, tt.evidence)
			}
		})
	}
}

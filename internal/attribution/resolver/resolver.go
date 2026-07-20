// Package resolver resolves a project without depending on storage or CLI.
package resolver

import (
	"path/filepath"
	"strings"
)

type Method string

const (
	Explicit    Method = "explicit"
	Adapter     Method = "adapter"
	Environment Method = "environment"
	CWD         Method = "cwd"
	GitRoot     Method = "git_root"
	Path        Method = "registered_path"
	Unresolved  Method = "unresolved"
)

type Confidence string

const (
	Exact   Confidence = "exact"
	High    Confidence = "high"
	Unknown Confidence = "unknown"
)

type Input struct {
	ExplicitProject    string
	AdapterProject     string
	EnvironmentProject string
	CWD                string
	GitRoot            string
}

type ProjectResolution struct {
	ProjectSlug string     `json:"project_slug,omitempty"`
	Method      Method     `json:"method"`
	Confidence  Confidence `json:"confidence"`
	Evidence    string     `json:"evidence"`
}

func Resolve(input Input, registeredPaths map[string]string) ProjectResolution {
	if slug := strings.TrimSpace(input.ExplicitProject); slug != "" {
		return ProjectResolution{ProjectSlug: slug, Method: Explicit, Confidence: Exact, Evidence: "explicit project"}
	}
	if slug := strings.TrimSpace(input.EnvironmentProject); slug != "" {
		return ProjectResolution{ProjectSlug: slug, Method: Environment, Confidence: High, Evidence: "QLOG_PROJECT"}
	}
	if slug, path := exactMatch(input.CWD, registeredPaths); slug != "" {
		return ProjectResolution{ProjectSlug: slug, Method: CWD, Confidence: High, Evidence: path}
	}
	if slug, path := exactMatch(input.GitRoot, registeredPaths); slug != "" {
		return ProjectResolution{ProjectSlug: slug, Method: GitRoot, Confidence: High, Evidence: path}
	}
	if slug, path := longestRegisteredMatch(input.CWD, input.GitRoot, registeredPaths); slug != "" {
		return ProjectResolution{ProjectSlug: slug, Method: Path, Confidence: High, Evidence: path}
	}
	if slug := strings.TrimSpace(input.AdapterProject); slug != "" {
		return ProjectResolution{ProjectSlug: slug, Method: Adapter, Confidence: High, Evidence: "adapter project signal"}
	}
	return ProjectResolution{Method: Unresolved, Confidence: Unknown, Evidence: "no project evidence"}
}

func exactMatch(candidate string, registeredPaths map[string]string) (string, string) {
	candidate = normalizePath(candidate)
	for path, slug := range registeredPaths {
		path = normalizePath(path)
		if candidate == path {
			return slug, path
		}
	}
	return "", ""
}

func longestRegisteredMatch(cwd, gitRoot string, registeredPaths map[string]string) (string, string) {
	cwdSlug, cwdPath := longestMatch(cwd, registeredPaths)
	gitSlug, gitPath := longestMatch(gitRoot, registeredPaths)
	if len(gitPath) > len(cwdPath) {
		return gitSlug, gitPath
	}
	return cwdSlug, cwdPath
}

func longestMatch(candidate string, registeredPaths map[string]string) (string, string) {
	candidate = normalizePath(candidate)
	var matchedSlug, matchedPath string
	for path, slug := range registeredPaths {
		path = normalizePath(path)
		if candidate == path || strings.HasPrefix(candidate, path+"/") {
			if len(path) > len(matchedPath) {
				matchedSlug, matchedPath = slug, path
			}
		}
	}
	return matchedSlug, matchedPath
}

func normalizePath(path string) string {
	path = strings.ReplaceAll(strings.TrimSpace(path), "\\", "/")
	path = filepath.ToSlash(filepath.Clean(path))
	return strings.TrimSuffix(strings.ToLower(path), "/")
}

// Package app coordinates domain resolution and local infrastructure.
package app

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/janpereira-dev/quantum_log/internal/attribution/resolver"
	"github.com/janpereira-dev/quantum_log/internal/config"
	storepkg "github.com/janpereira-dev/quantum_log/internal/storage/sqlite"
)

type Service struct {
	Paths config.Paths
	Store *storepkg.Store
}

// ResolvedProject keeps central project resolution separate from capture adapters.
type ResolvedProject struct {
	Resolution resolver.ProjectResolution
	ProjectID  string
	LocationID string
	CWD        string
	GitRoot    string
}

func Initialize(ctx context.Context, home string) (*Service, error) {
	paths, err := config.Resolve(home)
	if err != nil {
		return nil, fmt.Errorf("resolve paths: %w", err)
	}
	if err := config.Ensure(paths); err != nil {
		return nil, fmt.Errorf("create configuration: %w", err)
	}
	store, err := storepkg.Open(ctx, paths.Database)
	if err != nil {
		return nil, err
	}
	return &Service{Paths: paths, Store: store}, nil
}

func Open(ctx context.Context, home string) (*Service, error) {
	paths, err := config.Resolve(home)
	if err != nil {
		return nil, fmt.Errorf("resolve paths: %w", err)
	}
	if _, err := os.Stat(paths.Database); err != nil {
		return nil, fmt.Errorf("open local database: %w; run qlog init first", err)
	}
	store, err := storepkg.Open(ctx, paths.Database)
	if err != nil {
		return nil, err
	}
	return &Service{Paths: paths, Store: store}, nil
}

func (s *Service) Close() error { return s.Store.Close() }

func (s *Service) ResolveCurrent(ctx context.Context, explicitProject string) (resolver.ProjectResolution, error) {
	resolved, err := s.ResolveProject(ctx, explicitProject, "", "")
	if err != nil {
		return resolver.ProjectResolution{}, err
	}
	return resolved.Resolution, nil
}

// ResolveProject is the sole application boundary that turns user and capture
// signals into project attribution. Adapters cannot resolve projects themselves.
func (s *Service) ResolveProject(ctx context.Context, explicitProject, adapterProject, cwd string) (ResolvedProject, error) {
	paths, err := s.Store.RegisteredPaths(ctx)
	if err != nil {
		return ResolvedProject{}, err
	}
	if strings.TrimSpace(cwd) == "" {
		cwd, err = os.Getwd()
		if err != nil {
			return ResolvedProject{}, err
		}
	}
	resolved := ResolvedProject{CWD: cwd, GitRoot: gitRoot(ctx, cwd)}
	resolved.Resolution = resolver.Resolve(resolver.Input{ExplicitProject: explicitProject, AdapterProject: adapterProject, EnvironmentProject: os.Getenv("QLOG_PROJECT"), CWD: cwd, GitRoot: resolved.GitRoot}, paths)
	if resolved.Resolution.ProjectSlug == "" {
		return resolved, nil
	}
	project, location, found, err := s.Store.ProjectBySlug(ctx, resolved.Resolution.ProjectSlug)
	if err != nil {
		return ResolvedProject{}, err
	}
	if !found {
		return ResolvedProject{}, fmt.Errorf("resolved project %q is not registered", resolved.Resolution.ProjectSlug)
	}
	resolved.ProjectID = project.ID
	resolved.LocationID = location.ID
	return resolved, nil
}

func gitRoot(ctx context.Context, cwd string) string {
	command := exec.CommandContext(ctx, "git", "-C", cwd, "rev-parse", "--show-toplevel")
	output, err := command.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

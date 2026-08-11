// Package app coordinates domain resolution and local infrastructure.
package app

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/janpereira-dev/quantum_log/internal/attribution/resolver"
	"github.com/janpereira-dev/quantum_log/internal/config"
	"github.com/janpereira-dev/quantum_log/internal/domain"
	storepkg "github.com/janpereira-dev/quantum_log/internal/storage/sqlite"
)

type Service struct {
	Paths config.Paths
	Store *storepkg.Store
}

// ResolvedProject keeps central project resolution separate from capture adapters.
type ResolvedProject struct {
	Resolution   resolver.ProjectResolution
	ProjectID    string
	LocationID   string
	LocationPath string
	CWD          string
	GitRoot      string
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

// OpenReadOnly opens an initialized local ledger without creating configuration or migrations.
func OpenReadOnly(ctx context.Context, home string) (*Service, error) {
	paths, err := config.Resolve(home)
	if err != nil {
		return nil, fmt.Errorf("resolve paths: %w", err)
	}
	if _, err := os.Stat(paths.Database); err != nil {
		return nil, fmt.Errorf("open local database: %w; run qlog init first", err)
	}
	store, err := storepkg.OpenReadOnly(ctx, paths.Database)
	if err != nil {
		return nil, err
	}
	return &Service{Paths: paths, Store: store}, nil
}

// OpenSnapshotReadOnly opens a WAL-aware read snapshot for supported live evidence queries.
func OpenSnapshotReadOnly(ctx context.Context, home string) (*Service, error) {
	paths, err := config.Resolve(home)
	if err != nil {
		return nil, fmt.Errorf("resolve paths: %w", err)
	}
	if _, err := os.Stat(paths.Database); err != nil {
		return nil, fmt.Errorf("open local database: %w; run qlog init first", err)
	}
	store, err := storepkg.OpenSnapshotReadOnly(ctx, paths.Database)
	if err != nil {
		return nil, err
	}
	return &Service{Paths: paths, Store: store}, nil
}

func Checkpoint(ctx context.Context, home string) error {
	paths, err := config.Resolve(home)
	if err != nil {
		return fmt.Errorf("resolve paths: %w", err)
	}
	if _, err := os.Stat(paths.Database); err != nil {
		return fmt.Errorf("open local database: %w; run qlog init first", err)
	}
	return storepkg.Checkpoint(ctx, paths.Database)
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
		// A Git root is an unambiguous local ownership boundary. Create its
		// project once; anything without one remains deliberately unattributed.
		if resolved.GitRoot == "" {
			return resolved, nil
		}
		name := filepath.Base(resolved.GitRoot)
		project, location, registerErr := s.Store.RegisterProject(ctx, name, name, resolved.GitRoot)
		if registerErr != nil {
			return resolved, nil
		}
		resolved.ProjectID, resolved.LocationID, resolved.LocationPath = project.ID, location.ID, location.AbsolutePath
		resolved.Resolution = resolver.ProjectResolution{ProjectSlug: project.Slug, Method: resolver.GitRoot, Confidence: resolver.High, Evidence: location.AbsolutePath}
		return resolved, nil
	}
	var projectLocationLookup bool
	switch resolved.Resolution.Method {
	case resolver.CWD, resolver.GitRoot, resolver.Path:
		projectLocationLookup = true
	}
	var project domain.Project
	var location domain.ProjectLocation
	var found bool
	if projectLocationLookup {
		project, location, found, err = s.Store.ProjectByLocation(ctx, resolved.Resolution.Evidence)
	} else {
		project, location, found, err = s.Store.ProjectBySlug(ctx, resolved.Resolution.ProjectSlug)
	}
	if err != nil {
		return ResolvedProject{}, err
	}
	if !found {
		return ResolvedProject{}, fmt.Errorf("resolved project %q is not registered", resolved.Resolution.ProjectSlug)
	}
	resolved.ProjectID = project.ID
	resolved.LocationID = location.ID
	resolved.LocationPath = location.AbsolutePath
	return resolved, nil
}

// ResolveProjectFromVerifiedGitContext accepts only a unique Git root and
// normalized remote already verified during project registration.
func (s *Service) ResolveProjectFromVerifiedGitContext(ctx context.Context, gitRoot, remote string) (ResolvedProject, error) {
	project, location, found, err := s.Store.ProjectByVerifiedGitContext(ctx, gitRoot, remote)
	if err != nil {
		return ResolvedProject{}, err
	}
	if !found {
		return ResolvedProject{CWD: "", GitRoot: gitRoot, Resolution: resolver.ProjectResolution{Method: resolver.Unresolved, Confidence: resolver.Unknown, Evidence: "no unique verified git context"}}, nil
	}
	return ResolvedProject{
		ProjectID: project.ID, LocationID: location.ID, LocationPath: location.AbsolutePath, GitRoot: gitRoot,
		Resolution: resolver.ProjectResolution{ProjectSlug: project.Slug, Method: resolver.Adapter, Confidence: resolver.Exact, Evidence: "verified Copilot Git context"},
	}, nil
}

func gitRoot(ctx context.Context, cwd string) string {
	command := exec.CommandContext(ctx, "git", "-C", cwd, "rev-parse", "--show-toplevel")
	output, err := command.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

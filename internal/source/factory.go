package source

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/go-git/go-git/v5"
	gitplumbing "github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/google/uuid"
	"oras.land/oras-go/v2/registry"
	"oras.land/oras-go/v2/registry/remote"
)

const (
	SchemeLocal = "local"
	SchemeGit   = "git"
	SchemeS3    = "s3"
	SchemeOCI   = "oci"
)

// Source fetches bytes from different backends.
type Source interface {
	Fetch(ctx context.Context) ([]byte, error)
}

// Factory creates Source implementations.
type Factory struct{}

// NewFactory constructs a default source factory.
func NewFactory() *Factory {
	return &Factory{}
}

// New returns a Source for the given URI.
func (f *Factory) New(raw string) (Source, error) {
	if strings.HasPrefix(raw, "s3://") {
		return newS3Source(raw)
	}
	if strings.HasPrefix(raw, "oci://") {
		return newOCISource(raw)
	}
	if strings.HasPrefix(raw, "git+") {
		return newGitSource(raw)
	}
	return nil, fmt.Errorf("unsupported source %s", raw)
}

// ParseScheme returns the scheme of the path if it has one of the supported prefixes.
func ParseScheme(path string) string {
	switch {
	case strings.HasPrefix(path, "git+"):
		return SchemeGit
	case strings.HasPrefix(path, "s3://"):
		return SchemeS3
	case strings.HasPrefix(path, "oci://"):
		return SchemeOCI
	default:
		return SchemeLocal
	}
}

type gitSource struct {
	url    *url.URL
	subdir string
	ref    string
	path   string
}

func newGitSource(raw string) (Source, error) {
	trimmed := strings.TrimPrefix(raw, "git+")
	u, err := url.Parse(trimmed)
	if err != nil {
		return nil, fmt.Errorf("parse git source %s: %w", raw, err)
	}

	ref := u.Fragment
	u.Fragment = ""

	subdir := u.Path
	u.Path = ""

	return &gitSource{
		url:    u,
		subdir: strings.TrimPrefix(subdir, "/"),
		ref:    ref,
		path:   raw,
	}, nil
}

func (g *gitSource) Fetch(ctx context.Context) ([]byte, error) {
	tempDir := filepath.Join(os.TempDir(), "tmpl-git-"+uuid.NewString())
	repo, err := git.PlainCloneContext(ctx, tempDir, false, &git.CloneOptions{
		URL: g.url.String(),
	})
	if err != nil {
		// Attempt to handle basic auth env.
		if auth := basicAuthFromEnv(); auth != nil {
			repo, err = git.PlainCloneContext(ctx, tempDir, false, &git.CloneOptions{
				URL:  g.url.String(),
				Auth: auth,
			})
		}
		if err != nil {
			return nil, fmt.Errorf("clone git source %s: %w", g.path, err)
		}
	}

	if g.ref != "" {
		wt, err := repo.Worktree()
		if err != nil {
			return nil, err
		}
		if err := wt.Checkout(&git.CheckoutOptions{Branch: gitplumbing.ReferenceName("refs/heads/" + g.ref)}); err != nil {
			if err := wt.Checkout(&git.CheckoutOptions{Hash: gitplumbing.NewHash(g.ref)}); err != nil {
				return nil, fmt.Errorf("checkout %s: %w", g.ref, err)
			}
		}
	}

	target := filepath.Join(tempDir, g.subdir)
	data, err := os.ReadFile(target)
	if err != nil {
		return nil, fmt.Errorf("read git file %s: %w", target, err)
	}
	return data, nil
}

func basicAuthFromEnv() *http.BasicAuth {
	user := os.Getenv("TMPL_GIT_USERNAME")
	pass := os.Getenv("TMPL_GIT_PASSWORD")
	if user == "" && pass == "" {
		return nil
	}
	return &http.BasicAuth{Username: user, Password: pass}
}

type s3Source struct {
	bucket string
	key    string
}

func newS3Source(raw string) (Source, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}
	return &s3Source{
		bucket: u.Host,
		key:    strings.TrimPrefix(u.Path, "/"),
	}, nil
}

func (s *s3Source) Fetch(ctx context.Context) ([]byte, error) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, err
	}
	client := s3.NewFromConfig(cfg)
	out, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: &s.bucket,
		Key:    &s.key,
	})
	if err != nil {
		return nil, err
	}
	defer out.Body.Close()
	return io.ReadAll(out.Body)
}

type ociSource struct {
	ref string
}

func newOCISource(raw string) (Source, error) {
	return &ociSource{ref: strings.TrimPrefix(raw, "oci://")}, nil
}

func (o *ociSource) Fetch(ctx context.Context) ([]byte, error) {
	repo, err := remote.NewRepository(o.ref)
	if err != nil {
		return nil, err
	}
	if err := registry.Login(ctx, repo, registry.LoginOption{}); err != nil {
		return nil, err
	}
	resolver := repo.Blobs()
	desc, err := repo.Resolve(ctx, o.ref)
	if err != nil {
		return nil, err
	}
	rc, err := resolver.Fetch(ctx, desc)
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(rc)
}

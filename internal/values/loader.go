package values

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"dario.cat/mergo"
	"gopkg.in/yaml.v3"

	"github.com/acebelowzero/tmpl/internal/env"
	"github.com/acebelowzero/tmpl/internal/sops"
	"github.com/acebelowzero/tmpl/internal/source"
)

// LoaderConfig controls optional behaviour of Loader.
type LoaderConfig struct {
	EnvFiles []string
}

// Loader merges values from default chart values, additional files, and remote sources.
type Loader struct {
	cfg           LoaderConfig
	env           *env.Resolver
	sopsDecryptor sops.Decryptor
	sourceFactory *source.Factory
}

// NewLoader constructs a Loader with the provided dependencies.
func NewLoader(cfg LoaderConfig) (*Loader, error) {
	resolver, err := env.NewResolver(env.Config{Files: cfg.EnvFiles})
	if err != nil {
		return nil, err
	}

	decryptor, err := sops.New()
	if err != nil {
		return nil, err
	}

	return &Loader{
		cfg:           cfg,
		env:           resolver,
		sopsDecryptor: decryptor,
		sourceFactory: source.NewFactory(),
	}, nil
}

// Load composes values from defaults, user-specified files, and remote sources.
func (l *Loader) Load(ctx context.Context, chartPath string, extraFiles ...string) (map[string]any, error) {
	if chartPath == "" {
		chartPath = "."
	}

	baseValues, err := l.readValuesFile(ctx, filepath.Join(chartPath, "values.yaml"))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	if baseValues == nil {
		baseValues = map[string]any{}
	}

	for _, file := range extraFiles {
		data, err := l.readValuesFile(ctx, file)
		if err != nil {
			return nil, err
		}
		if err := mergo.Merge(&baseValues, data, mergo.WithOverride); err != nil {
			return nil, fmt.Errorf("merge values from %s: %w", file, err)
		}
	}

	return baseValues, nil
}

func (l *Loader) readValuesFile(ctx context.Context, path string) (map[string]any, error) {
	if path == "" {
		return nil, errors.New("values file path is empty")
	}

	var data []byte
	var err error
	var baseDir string

	scheme := source.ParseScheme(path)
	if scheme == source.SchemeLocal {
		data, err = os.ReadFile(path)
		baseDir = filepath.Dir(path)
	} else {
		src, err := l.sourceFactory.New(path)
		if err != nil {
			return nil, err
		}
		data, err = src.Fetch(ctx)
		baseDir = ""
	}
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, err
		}
		return nil, fmt.Errorf("read values file %s: %w", path, err)
	}

	expanded, err := l.env.Expand(data)
	if err != nil {
		return nil, fmt.Errorf("expand environment in %s: %w", path, err)
	}

	decoded := map[string]any{}
	if err := yaml.Unmarshal(expanded, &decoded); err != nil {
		return nil, fmt.Errorf("decode yaml %s: %w", path, err)
	}

	processed, err := l.decryptValues(ctx, decoded, baseDir)
	if err != nil {
		return nil, fmt.Errorf("decrypt secrets in %s: %w", path, err)
	}

	result, ok := processed.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("values file %s must decode to an object", path)
	}
	return result, nil
}

func (l *Loader) decryptValues(ctx context.Context, node any, baseDir string) (any, error) {
	switch v := node.(type) {
	case map[string]any:
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		result := make(map[string]any, len(v))
		for _, key := range keys {
			processedValue, err := l.decryptValues(ctx, v[key], baseDir)
			if err != nil {
				return nil, err
			}
			newKey := strings.TrimSuffix(key, ".enc")
			result[newKey] = processedValue
		}
		return result, nil
	case []any:
		result := make([]any, len(v))
		for i := range v {
			processedValue, err := l.decryptValues(ctx, v[i], baseDir)
			if err != nil {
				return nil, err
			}
			result[i] = processedValue
		}
		return result, nil
	case string:
		if !strings.HasSuffix(v, ".enc") {
			return v, nil
		}
		decrypted, err := l.decryptValue(ctx, v, baseDir)
		if err != nil {
			return nil, err
		}
		return decrypted, nil
	default:
		return node, nil
	}
}

func (l *Loader) decryptValue(ctx context.Context, ref, baseDir string) (any, error) {
	path := ref
	if !filepath.IsAbs(path) && baseDir != "" {
		path = filepath.Join(baseDir, ref)
	}
	data, err := l.sopsDecryptor.DecryptFile(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("decrypt %s: %w", ref, err)
	}

	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return "", nil
	}

	var parsed any
	if err := yaml.Unmarshal(trimmed, &parsed); err == nil {
		return parsed, nil
	}
	return string(trimmed), nil
}

package configloader

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/lHumaNl/gomock/internal/domain/mapping"
	"github.com/lHumaNl/gomock/internal/files"
	"gopkg.in/yaml.v3"
)

type Loader struct {
	strict bool
}

func NewLoader(strict bool) *Loader {
	return &Loader{strict: strict}
}

func (l *Loader) LoadRoot(root string) ([]mapping.Mapping, error) {
	paths, err := mappingFiles(root)
	if err != nil {
		return nil, err
	}

	resolver := files.NewResolver(root)
	mappings := make([]mapping.Mapping, 0, len(paths))
	for _, path := range paths {
		item, err := l.loadFile(path, resolver)
		if err != nil {
			return nil, err
		}
		mappings = append(mappings, item)
	}

	return mappings, nil
}

func (l *Loader) loadFile(path string, resolver *files.Resolver) (mapping.Mapping, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return mapping.Mapping{}, configError(path, "file", err.Error())
	}

	raw, err := l.decode(path, data)
	if err != nil {
		return mapping.Mapping{}, configError(path, "file", err.Error())
	}

	return buildMapping(path, raw, resolver)
}

func (l *Loader) decode(path string, data []byte) (rawMapping, error) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".json":
		return decodeJSON(data, l.strict)
	case ".yaml", ".yml":
		return decodeYAML(data, l.strict)
	default:
		return rawMapping{}, fmt.Errorf("unsupported mapping extension")
	}
}

func mappingFiles(root string) ([]string, error) {
	mappingRoot := filepath.Join(root, "mappings")
	entries, err := os.ReadDir(mappingRoot)
	if err != nil {
		return nil, configError(mappingRoot, "mappings", err.Error())
	}

	paths := supportedMappingFiles(mappingRoot, entries)
	sort.Strings(paths)
	return paths, nil
}

func supportedMappingFiles(root string, entries []os.DirEntry) []string {
	paths := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() && hasMappingExtension(entry.Name()) {
			paths = append(paths, filepath.Join(root, entry.Name()))
		}
	}
	return paths
}

func hasMappingExtension(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	return ext == ".json" || ext == ".yaml" || ext == ".yml"
}

func decodeJSON(data []byte, strict bool) (rawMapping, error) {
	var raw rawMapping
	decoder := json.NewDecoder(bytes.NewReader(data))
	if strict {
		decoder.DisallowUnknownFields()
	}
	if err := decoder.Decode(&raw); err != nil {
		return rawMapping{}, err
	}
	return raw, nil
}

func decodeYAML(data []byte, strict bool) (rawMapping, error) {
	var raw rawMapping
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(strict)
	if err := decoder.Decode(&raw); err != nil {
		return rawMapping{}, err
	}
	return raw, nil
}

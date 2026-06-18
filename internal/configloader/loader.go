package configloader

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"github.com/lHumaNl/gomock/internal/domain/mapping"
	"github.com/lHumaNl/gomock/internal/files"
	"github.com/titanous/json5"
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
		items, err := l.loadFile(path, resolver)
		if err != nil {
			return nil, err
		}
		mappings = append(mappings, items...)
	}

	return mappings, nil
}

func (l *Loader) loadFile(path string, resolver *files.Resolver) ([]mapping.Mapping, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, configError(path, "file", err.Error())
	}

	raw, err := l.decode(path, data)
	if err != nil {
		return nil, configError(path, "file", err.Error())
	}

	if raw.Mappings != nil {
		return buildMappingArray(path, raw.Mappings, resolver)
	}
	item, err := buildMapping(path, raw.singleMapping(), resolver)
	if err != nil {
		return nil, err
	}
	return []mapping.Mapping{item}, nil
}

func (l *Loader) decode(path string, data []byte) (rawMappingFile, error) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".json":
		return decodeJSON(data, l.strict)
	case ".yaml", ".yml":
		return decodeYAML(data, l.strict)
	default:
		return rawMappingFile{}, fmt.Errorf("unsupported mapping extension")
	}
}

func buildMappingArray(path string, raws []rawMapping, resolver *files.Resolver) ([]mapping.Mapping, error) {
	items := make([]mapping.Mapping, 0, len(raws))
	for index, raw := range raws {
		itemPath := mappingArrayItemPath(path, index)
		if strings.TrimSpace(raw.ID) == "" {
			raw.ID = mappingArrayItemID(path, index, raw.Name)
		}
		item, err := buildMapping(itemPath, raw, resolver)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func mappingArrayItemPath(path string, index int) string {
	return fmt.Sprintf("%s:mappings[%d]", path, index)
}

func mappingArrayItemID(path string, index int, name string) string {
	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	suffix := itoa(index)
	if slug := slugMappingName(name); slug != "" {
		suffix += "-" + slug
	}
	return base + "-" + suffix
}

func slugMappingName(name string) string {
	var builder strings.Builder
	lastDash := false
	for _, char := range strings.ToLower(strings.TrimSpace(name)) {
		if unicode.IsLetter(char) || unicode.IsDigit(char) {
			builder.WriteRune(char)
			lastDash = false
			continue
		}
		if builder.Len() > 0 && !lastDash {
			builder.WriteByte('-')
			lastDash = true
		}
	}
	return strings.TrimSuffix(builder.String(), "-")
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

func decodeJSON(data []byte, strict bool) (rawMappingFile, error) {
	var raw rawMappingFile
	decoder := json5.NewDecoder(bytes.NewReader(data))
	if strict {
		decoder.DisallowUnknownFields()
	}
	if err := decoder.Decode(&raw); err != nil {
		return rawMappingFile{}, err
	}
	return raw, nil
}

func decodeYAML(data []byte, strict bool) (rawMappingFile, error) {
	var raw rawMappingFile
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(strict)
	if err := decoder.Decode(&raw); err != nil {
		return rawMappingFile{}, err
	}
	return raw, nil
}

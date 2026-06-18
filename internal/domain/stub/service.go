package stub

import (
	"errors"

	"github.com/lHumaNl/gomock/internal/domain/mapping"
	"github.com/lHumaNl/gomock/internal/domain/matcher"
	"github.com/lHumaNl/gomock/internal/domain/selector"
)

var ErrResponseNotConfigured = errors.New("response selector is not configured")

type Match struct {
	MappingID   string
	VariantName string
	Response    mapping.Response
}

type Service struct {
	mappings  []mapping.Mapping
	selectors []responseSelector
}

type responseSelector struct {
	selector selector.Selector
	err      error
	single   bool
}

func NewService(mappings []mapping.Mapping) *Service {
	copied := make([]mapping.Mapping, len(mappings))
	copy(copied, mappings)
	return &Service{mappings: copied, selectors: buildSelectors(copied)}
}

func (s *Service) Match(request matcher.Request) (Match, bool, error) {
	selection := matcher.Select(s.mappings, request)
	if !selection.Found() {
		return Match{}, false, nil
	}
	response, variant, err := s.selectResponse(selection.Index)
	if err != nil {
		return Match{MappingID: selection.Mapping.ID}, true, err
	}
	return Match{MappingID: selection.Mapping.ID, VariantName: variant, Response: response}, true, nil
}

func buildSelectors(mappings []mapping.Mapping) []responseSelector {
	selectors := make([]responseSelector, 0, len(mappings))
	for index := range mappings {
		selectors = append(selectors, newResponseSelector(mappings[index]))
	}
	return selectors
}

func newResponseSelector(item mapping.Mapping) responseSelector {
	if item.Response != nil {
		return responseSelector{selector: selector.NewSingle(*item.Response), single: true}
	}
	if item.Responses != nil {
		created, err := selector.NewSet(*item.Responses)
		return responseSelector{selector: created, err: err}
	}
	return responseSelector{err: ErrResponseNotConfigured}
}

func (s *Service) selectResponse(index int) (mapping.Response, string, error) {
	if index < 0 || index >= len(s.selectors) {
		return mapping.Response{}, "", ErrResponseNotConfigured
	}
	selected := s.selectors[index]
	if selected.err != nil {
		return mapping.Response{}, "", selected.err
	}
	response := selected.selector.Select()
	return response, selected.variantName(response), nil
}

func (s responseSelector) variantName(response mapping.Response) string {
	if s.single {
		return "default"
	}
	if response.Name == "" {
		return "default"
	}
	return response.Name
}

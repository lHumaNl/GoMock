package selector

import "github.com/lHumaNl/gomock/internal/domain/mapping"

type singleSelector struct {
	response mapping.Response
}

func (s singleSelector) Select() mapping.Response {
	return s.response
}

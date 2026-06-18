package configloader

type rawMapping struct {
	ID        string        `json:"id" yaml:"id"`
	Name      string        `json:"name" yaml:"name"`
	Priority  *int          `json:"priority" yaml:"priority"`
	Request   *rawRequest   `json:"request" yaml:"request"`
	Response  *rawResponse  `json:"response" yaml:"response"`
	Responses *rawResponses `json:"responses" yaml:"responses"`
}

type rawRequest struct {
	Method          string                 `json:"method" yaml:"method"`
	URL             string                 `json:"url" yaml:"url"`
	URLPath         string                 `json:"urlPath" yaml:"urlPath"`
	URLPattern      string                 `json:"urlPattern" yaml:"urlPattern"`
	QueryParameters map[string]rawOperator `json:"queryParameters" yaml:"queryParameters"`
	Headers         map[string]rawOperator `json:"headers" yaml:"headers"`
	BodyPatterns    []rawOperator          `json:"bodyPatterns" yaml:"bodyPatterns"`
}

type rawOperator map[string]any

type rawResponse struct {
	Name         string            `json:"name" yaml:"name"`
	Weight       int               `json:"weight" yaml:"weight"`
	Status       *int              `json:"status" yaml:"status"`
	Headers      map[string]string `json:"headers" yaml:"headers"`
	Body         *string           `json:"body" yaml:"body"`
	BodyFileName string            `json:"bodyFileName" yaml:"bodyFileName"`
	Delay        *rawDelay         `json:"delay" yaml:"delay"`
}

type rawResponses struct {
	Mode     string        `json:"mode" yaml:"mode"`
	Variants []rawResponse `json:"variants" yaml:"variants"`
}

type rawDelay struct {
	Type  string `json:"type" yaml:"type"`
	Value string `json:"value" yaml:"value"`
	Min   string `json:"min" yaml:"min"`
	Max   string `json:"max" yaml:"max"`
}

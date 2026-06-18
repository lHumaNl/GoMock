package mapping

type URLMatchKind string

const (
	URLMatchKindURL             URLMatchKind = "url"
	URLMatchKindURLPath         URLMatchKind = "urlPath"
	URLMatchKindURLPattern      URLMatchKind = "urlPattern"
	URLMatchKindURLPathTemplate URLMatchKind = "urlPathTemplate"
	URLMatchKindURLPathPattern  URLMatchKind = "urlPathPattern"
)

type Operator string

const (
	OperatorEqualTo         Operator = "equalTo"
	OperatorContains        Operator = "contains"
	OperatorMatches         Operator = "matches"
	OperatorDoesNotMatch    Operator = "doesNotMatch"
	OperatorDoesNotContain  Operator = "doesNotContain"
	OperatorAbsent          Operator = "absent"
	OperatorHasExactly      Operator = "hasExactly"
	OperatorIncludes        Operator = "includes"
	OperatorMatchesJSONPath Operator = "matchesJsonPath"
	OperatorMatchesXPath    Operator = "matchesXPath"
)

type Request struct {
	Method          string
	URLKind         URLMatchKind
	URLValue        string
	Headers         map[string]Matcher
	QueryParameters map[string]Matcher
	Cookies         map[string]Matcher
	PathParameters  map[string]Matcher
	BodyPatterns    []Matcher
	BasicAuth       *BasicAuth
}

type Matcher struct {
	Operator        Operator
	Value           string
	ValueMatchers   []Matcher
	CaseInsensitive bool
}

type BasicAuth struct {
	Username string
	Password string
}

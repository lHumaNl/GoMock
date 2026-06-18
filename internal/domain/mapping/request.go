package mapping

type URLMatchKind string

const (
	URLMatchKindURL        URLMatchKind = "url"
	URLMatchKindURLPath    URLMatchKind = "urlPath"
	URLMatchKindURLPattern URLMatchKind = "urlPattern"
)

type Operator string

const (
	OperatorEqualTo         Operator = "equalTo"
	OperatorContains        Operator = "contains"
	OperatorMatches         Operator = "matches"
	OperatorAbsent          Operator = "absent"
	OperatorMatchesJSONPath Operator = "matchesJsonPath"
)

type Request struct {
	Method          string
	URLKind         URLMatchKind
	URLValue        string
	Headers         map[string]Matcher
	QueryParameters map[string]Matcher
	BodyPatterns    []Matcher
}

type Matcher struct {
	Operator Operator
	Value    string
}

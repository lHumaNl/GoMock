package mapping

const DefaultPriority = 5

type Mapping struct {
	ID        string
	Name      string
	Priority  int
	Request   Request
	Response  *Response
	Responses *ResponseSet
}

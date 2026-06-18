package mapping

type ResponseMode string

const (
	ResponseModeSequential ResponseMode = "sequential"
	ResponseModeRandom     ResponseMode = "random"
	ResponseModeWeighted   ResponseMode = "weighted"

	TransformerResponseTemplate = "response-template"
)

type Response struct {
	Name            string
	Weight          int
	Status          int
	Headers         map[string]string
	Body            string
	BodyFileName    string
	BodyFileContent []byte
	Transformers    []string
	Delay           *Delay
}

func (r Response) HasTransformer(name string) bool {
	for _, transformer := range r.Transformers {
		if transformer == name {
			return true
		}
	}
	return false
}

type ResponseSet struct {
	Mode     ResponseMode
	Variants []Response
}

package mapping

type ResponseMode string

const (
	ResponseModeSequential ResponseMode = "sequential"
	ResponseModeRandom     ResponseMode = "random"
	ResponseModeWeighted   ResponseMode = "weighted"
)

type Response struct {
	Name            string
	Weight          int
	Status          int
	Headers         map[string]string
	Body            string
	BodyFileName    string
	BodyFileContent []byte
	Delay           *Delay
}

type ResponseSet struct {
	Mode     ResponseMode
	Variants []Response
}

package responsetemplate

import "fmt"

func tokenize(input string) ([]string, error) {
	state := tokenState{}
	for _, char := range input {
		if err := state.accept(char); err != nil {
			return nil, err
		}
	}
	return state.finish()
}

type tokenState struct {
	tokens  []string
	current []rune
	quote   rune
	escape  bool
}

func (s *tokenState) accept(char rune) error {
	if s.escape {
		s.current = append(s.current, char)
		s.escape = false
		return nil
	}
	if char == '\\' && s.quote != 0 {
		s.escape = true
		return nil
	}
	if s.quote != 0 {
		return s.acceptQuoted(char)
	}
	return s.acceptUnquoted(char)
}

func (s *tokenState) acceptQuoted(char rune) error {
	if char == s.quote {
		s.quote = 0
		return nil
	}
	s.current = append(s.current, char)
	return nil
}

func (s *tokenState) acceptUnquoted(char rune) error {
	if char == '\'' || char == '"' {
		s.quote = char
		return nil
	}
	if char == ' ' || char == '\t' || char == '\n' || char == '\r' {
		s.flush()
		return nil
	}
	s.current = append(s.current, char)
	return nil
}

func (s *tokenState) finish() ([]string, error) {
	if s.quote != 0 {
		return nil, fmt.Errorf("unterminated quoted string")
	}
	s.flush()
	return s.tokens, nil
}

func (s *tokenState) flush() {
	if len(s.current) == 0 {
		return
	}
	s.tokens = append(s.tokens, string(s.current))
	s.current = nil
}

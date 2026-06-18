package wiremockregex

import "testing"

func TestMatchStringUsesStandardRegex(t *testing.T) {
	matched, err := MatchString(`^/users/\d+$`, "/users/42")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !matched {
		t.Fatalf("expected standard regex to match")
	}
}

func TestMatchStringSupportsNegativeLookahead(t *testing.T) {
	matched, err := MatchString(`^(?!UC0).+`, "UC123")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !matched {
		t.Fatalf("expected negative lookahead to match non-excluded prefix")
	}

	matched, err = MatchString(`^(?!UC0).+`, "UC012")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if matched {
		t.Fatalf("expected negative lookahead to reject excluded prefix")
	}
}

func TestValidateRejectsInvalidRegex(t *testing.T) {
	if err := Validate("["); err == nil {
		t.Fatalf("expected invalid regex error")
	}
}

package transformer

import (
	"testing"
)

func TestTransformer_Transform_Success(t *testing.T) {
	tr := New()

	payload := []byte(`{"event":"push","author":{"name":"john"},"commits":3}`)
	tmpl := `{"type":"alert","message":"{{.author.name}} pushed {{.commits}} commits!"}`

	output, err := tr.Transform(payload, tmpl)
	if err != nil {
		t.Fatalf("Transform failed: %v", err)
	}

	expected := `{"type":"alert","message":"john pushed 3 commits!"}`
	if string(output) != expected {
		t.Errorf("Expected %s, got %s", expected, output)
	}
}

func TestTransformer_Transform_Passthrough(t *testing.T) {
	tr := New()

	payload := []byte(`{"hello":"world"}`)
	tmpl := ""

	output, err := tr.Transform(payload, tmpl)
	if err != nil {
		t.Fatalf("Transform failed: %v", err)
	}

	if string(output) != string(payload) {
		t.Errorf("Expected %s, got %s", string(payload), output)
	}
}

func TestTransformer_Transform_InvalidInputJSON(t *testing.T) {
	tr := New()

	payload := []byte(`{invalid`)
	tmpl := `{{.foo}}`

	_, err := tr.Transform(payload, tmpl)
	if err == nil {
		t.Fatal("Expected error for invalid input JSON")
	}
}

func TestTransformer_Transform_InvalidOutputJSON(t *testing.T) {
	tr := New()

	payload := []byte(`{"user":"bob"}`)
	tmpl := `This is not valid json: {{.user}}`

	_, err := tr.Transform(payload, tmpl)
	if err == nil {
		t.Fatal("Expected error for invalid output JSON")
	}
}

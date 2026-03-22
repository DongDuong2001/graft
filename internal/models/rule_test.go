package models

import (
	"encoding/json"
	"testing"
)

func TestRule_JSONOmitsSignatureSecret(t *testing.T) {
	r := Rule{
		ID:              "1",
		Name:            "n",
		ListenPath:      "/hook/x",
		SignatureSecret: "must-not-leak",
		DestinationURL:  "https://x",
	}
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	if _, ok := m["signature_secret"]; ok {
		t.Fatalf("secret leaked in JSON: %s", b)
	}
}

func TestDelivery_JSON(t *testing.T) {
	d := Delivery{ID: "d", RuleID: "r", Success: true, StatusCode: 200}
	b, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	var out Delivery
	if err := json.Unmarshal(b, &out); err != nil || out.ID != "d" {
		t.Fatalf("%v %s", err, b)
	}
}

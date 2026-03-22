package httpapi

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"testing"
	"time"
)

func TestVerifySignatureHex(t *testing.T) {
	secret := "shh"
	payload := []byte(`{"x":1}`)
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(payload)
	sig := hex.EncodeToString(mac.Sum(nil))

	if !verifySignatureHex(payload, secret, sig) {
		t.Fatal("expected valid hex signature")
	}
	if !verifySignatureHex(payload, secret, "sha256="+sig) {
		t.Fatal("expected valid sha256= prefix")
	}
	if verifySignatureHex(payload, secret, "deadbeef") {
		t.Fatal("expected invalid signature")
	}
}

func TestVerifyStripeV1(t *testing.T) {
	secret := "whsec_test"
	payload := []byte(`{"id":"evt_1"}`)
	ts := time.Now().Unix()
	signed := strconv.FormatInt(ts, 10) + "." + string(payload)
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(signed))
	sig := hex.EncodeToString(mac.Sum(nil))
	header := "t=" + strconv.FormatInt(ts, 10) + ",v1=" + sig

	if !verifyStripeV1(payload, secret, header) {
		t.Fatal("expected valid stripe signature")
	}
	if verifyStripeV1(payload, secret, "t=1,v1=beef") {
		t.Fatal("expected invalid stripe signature")
	}
}

func TestVerifyTimestampSkew(t *testing.T) {
	now := time.Now().Unix()
	if err := verifyTimestampSkew("X-Ts", strconv.FormatInt(now, 10), 60); err != nil {
		t.Fatalf("expected ok, got %v", err)
	}
	if err := verifyTimestampSkew("X-Ts", "", 60); err == nil {
		t.Fatal("expected missing header error")
	}
	old := time.Now().Add(-400 * time.Second).Unix()
	if err := verifyTimestampSkew("X-Ts", strconv.FormatInt(old, 10), 60); err == nil {
		t.Fatal("expected skew error")
	}
}

package httpapi

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"
)

func verifySignatureHex(payload []byte, secret, headerValue string) bool {
	if strings.HasPrefix(headerValue, "sha256=") {
		headerValue = strings.TrimPrefix(headerValue, "sha256=")
	}

	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(payload)
	expectedMAC := hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(expectedMAC), []byte(headerValue))
}

func verifyStripeV1(payload []byte, secret, headerValue string) bool {
	var ts string
	var sigs []string
	for _, part := range strings.Split(headerValue, ",") {
		part = strings.TrimSpace(part)
		switch {
		case strings.HasPrefix(part, "t="):
			ts = strings.TrimPrefix(part, "t=")
		case strings.HasPrefix(part, "v1="):
			sigs = append(sigs, strings.TrimPrefix(part, "v1="))
		}
	}
	if ts == "" || len(sigs) == 0 {
		return false
	}

	signed := ts + "." + string(payload)
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(signed))
	expected := hex.EncodeToString(mac.Sum(nil))

	for _, s := range sigs {
		if hmac.Equal([]byte(expected), []byte(s)) {
			return true
		}
	}
	return false
}

func parseStripeTimestamp(headerValue string) (int64, bool) {
	for _, part := range strings.Split(headerValue, ",") {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, "t=") {
			v := strings.TrimPrefix(part, "t=")
			sec, err := strconv.ParseInt(v, 10, 64)
			if err != nil {
				return 0, false
			}
			return sec, true
		}
	}
	return 0, false
}

func verifyTimestampSkew(headerName, headerValue string, maxSkew int) error {
	if headerName == "" {
		return nil
	}
	raw := strings.TrimSpace(headerValue)
	if raw == "" {
		return fmt.Errorf("missing timestamp header %q", headerName)
	}
	sec, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid timestamp in %q", headerName)
	}
	if maxSkew <= 0 {
		maxSkew = 300
	}
	event := time.Unix(sec, 0)
	delta := time.Since(event)
	if delta < 0 {
		delta = -delta
	}
	if delta > time.Duration(maxSkew)*time.Second {
		return fmt.Errorf("timestamp outside allowed skew")
	}
	return nil
}

func verifyStripeReplay(headerValue string, maxSkew int) error {
	ts, ok := parseStripeTimestamp(headerValue)
	if !ok {
		return fmt.Errorf("stripe signature missing timestamp")
	}
	if maxSkew <= 0 {
		maxSkew = 300
	}
	event := time.Unix(ts, 0)
	delta := time.Since(event)
	if delta < 0 {
		delta = -delta
	}
	if delta > time.Duration(maxSkew)*time.Second {
		return fmt.Errorf("stripe timestamp outside allowed skew")
	}
	return nil
}

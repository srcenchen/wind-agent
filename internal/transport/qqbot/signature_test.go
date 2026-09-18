package qqbot

import (
	"encoding/json"
	"net/http"
	"testing"
)

// 官方文档回调验证示例：
// secret DG5g3B4j9X2KOErG，plain_token + event_ts 算出固定 signature。
func TestValidationSignatureMatchesOfficialExample(t *testing.T) {
	secret := "DG5g3B4j9X2KOErG"
	plainToken := "Arq0D5A61EgUu4OxUvOp"
	eventTs := "1725442341"
	want := "87befc99c42c651b3aac0278e71ada338433ae26fcb24307bdc5ad38c1adc2d01bcfcadc0842edac85e85205028a1132afe09280305f13aa6909ffc2d652c706"

	got, err := SignValidation(secret, eventTs, plainToken)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("signature mismatch\n got %s\nwant %s", got, want)
	}
}

func TestVerifyAcceptsGeneratedSignature(t *testing.T) {
	secret := "DG5g3B4j9X2KOErG"
	body := []byte(`{"op":0,"t":"C2C_MESSAGE_CREATE","d":{"id":"m1"}}`)
	ts := "1725442341"
	sig, err := Sign(secret, ts, body)
	if err != nil {
		t.Fatal(err)
	}
	h := http.Header{}
	h.Set(HeaderSignature, sig)
	h.Set(HeaderTimestamp, ts)
	ok, err := Verify(secret, h, body)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected signature to verify")
	}
}

func TestVerifyRejectsTamperedBody(t *testing.T) {
	secret := "DG5g3B4j9X2KOErG"
	body := []byte(`{"op":0}`)
	ts := "1725442341"
	sig, err := Sign(secret, ts, body)
	if err != nil {
		t.Fatal(err)
	}
	h := http.Header{}
	h.Set(HeaderSignature, sig)
	h.Set(HeaderTimestamp, ts)
	ok, err := Verify(secret, h, []byte(`{"op":1}`))
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("tampered body should fail verify")
	}
}

func TestValidationACKJSON(t *testing.T) {
	secret := "DG5g3B4j9X2KOErG"
	plainToken := "Arq0D5A61EgUu4OxUvOp"
	eventTs := "1725442341"
	raw, err := ValidationACK(secret, eventTs, plainToken)
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		PlainToken string `json:"plain_token"`
		Signature  string `json:"signature"`
	}
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if got.PlainToken != plainToken {
		t.Fatalf("plain_token=%s", got.PlainToken)
	}
	if got.Signature == "" {
		t.Fatal("missing signature")
	}
}

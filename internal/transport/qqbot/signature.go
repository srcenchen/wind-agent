package qqbot

import (
	"bytes"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

const (
	HeaderSignature = "X-Signature-Ed25519"
	HeaderTimestamp = "X-Signature-Timestamp"
)

func Sign(secret, timestamp string, body []byte) (string, error) {
	if timestamp == "" {
		return "", errors.New("empty timestamp")
	}
	var msg bytes.Buffer
	msg.WriteString(timestamp)
	msg.Write(body)
	return signBytes(secret, msg.Bytes())
}

func SignValidation(secret, eventTs, plainToken string) (string, error) {
	return signBytes(secret, []byte(eventTs+plainToken))
}

func Verify(secret string, header http.Header, body []byte) (bool, error) {
	sigHex := header.Get(HeaderSignature)
	ts := header.Get(HeaderTimestamp)
	if sigHex == "" || ts == "" {
		return false, nil
	}
	sig, err := hex.DecodeString(sigHex)
	if err != nil {
		return false, fmt.Errorf("decode signature: %w", err)
	}
	if len(sig) != ed25519.SignatureSize {
		return false, nil
	}
	priv, err := ed25519Key(secret)
	if err != nil {
		return false, err
	}
	var msg bytes.Buffer
	msg.WriteString(ts)
	msg.Write(body)
	return ed25519.Verify(priv.Public().(ed25519.PublicKey), msg.Bytes(), sig), nil
}

func ValidationACK(secret, eventTs, plainToken string) ([]byte, error) {
	sig, err := SignValidation(secret, eventTs, plainToken)
	if err != nil {
		return nil, err
	}
	return json.Marshal(struct {
		PlainToken string `json:"plain_token"`
		Signature  string `json:"signature"`
	}{PlainToken: plainToken, Signature: sig})
}

func signBytes(secret string, msg []byte) (string, error) {
	key, err := ed25519Key(secret)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(ed25519.Sign(key, msg)), nil
}

func ed25519Key(secret string) (ed25519.PrivateKey, error) {
	if secret == "" {
		return nil, errors.New("empty secret")
	}
	seed := secret
	for len(seed) < ed25519.SeedSize {
		seed = strings.Repeat(seed, 2)
	}
	seed = seed[:ed25519.SeedSize]
	_, priv, err := ed25519.GenerateKey(strings.NewReader(seed))
	if err != nil {
		return nil, err
	}
	return priv, nil
}

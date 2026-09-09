// Package applemusic 는 Apple Music API 에 붙는다.
//
// music 패키지가 내 Mac 의 Music.app 에 말을 건다면, 여기는 Apple 의 서버에
// 말을 건다. 둘의 관심사가 다르다 — 저쪽은 **내가 담은 곡**이고
// 이쪽은 **아직 담지 않은 곡**이다.
package applemusic

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"time"
)

// DeveloperToken 은 p8 개인키로 ES256 JWT 를 만든다.
//
// 라이브러리를 쓰지 않는다. ES256 서명은 표준 라이브러리로 30줄이고,
// 의존성 하나 늘리는 값이 그보다 비싸다.
func DeveloperToken(p8 []byte, keyID, teamID string, ttl time.Duration) (string, error) {
	block, _ := pem.Decode(p8)
	if block == nil {
		return "", errors.New("could not read the p8 file as PEM")
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return "", err
	}
	key, ok := parsed.(*ecdsa.PrivateKey)
	if !ok {
		return "", errors.New("p8 is not an ECDSA key — check it is a MusicKit key")
	}
	// Apple 은 6개월을 넘는 만료를 거부한다.
	if ttl > 180*24*time.Hour {
		return "", errors.New("expiry cannot exceed six months")
	}

	now := time.Now()
	// origin 클레임은 일부러 넣지 않는다.
	// 넣으면 그 출처에서만 동작하는데 우리는 localhost 임의 포트를 쓴다.
	head := map[string]string{"alg": "ES256", "kid": keyID, "typ": "JWT"}
	body := map[string]any{"iss": teamID, "iat": now.Unix(), "exp": now.Add(ttl).Unix()}

	signing, err := segment(head)
	if err != nil {
		return "", err
	}
	payload, err := segment(body)
	if err != nil {
		return "", err
	}
	signing += "." + payload

	sum := sha256.Sum256([]byte(signing))
	r, s, err := ecdsa.Sign(rand.Reader, key, sum[:])
	if err != nil {
		return "", err
	}
	// JOSE 는 DER 이 아니라 r||s 를 32바이트씩 이어붙인 raw 형식을 쓴다.
	sig := make([]byte, 64)
	r.FillBytes(sig[:32])
	s.FillBytes(sig[32:])

	return signing + "." + b64(sig), nil
}

func segment(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return b64(b), nil
}

func b64(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

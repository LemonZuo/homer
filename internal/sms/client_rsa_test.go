package sms

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"math/big"
	"testing"

	"github.com/LemonZuo/homer/internal/model"
)

// javaServerDecryptRequest 模拟 SmsForwarder 服务端 mode2 收请求：
// Cipher("RSA") = RSA/ECB/NoPadding：Base64.decode(body) -> 每 256 字节块裸 RSA
// 私钥解密(c^d mod n) -> 去左侧 0 填充 -> 拼接 -> Base64.decode
func javaServerDecryptRequest(t *testing.T, priv *rsa.PrivateKey, body []byte) []byte {
	ct, err := base64.StdEncoding.DecodeString(string(body))
	if err != nil {
		t.Fatalf("server: body base64 decode: %v", err)
	}
	k := priv.Size()
	if len(ct)%k != 0 {
		t.Fatalf("server: ciphertext len %d not multiple of %d", len(ct), k)
	}
	var b64 bytes.Buffer
	for off := 0; off < len(ct); off += k {
		m := new(big.Int).Exp(new(big.Int).SetBytes(ct[off:off+k]), priv.D, priv.N)
		blk := m.FillBytes(make([]byte, k))
		i := 0
		for i < len(blk) && blk[i] == 0x00 {
			i++
		}
		b64.Write(blk[i:])
	}
	plain, err := base64.StdEncoding.DecodeString(b64.String())
	if err != nil {
		t.Fatalf("server: inner base64 decode: %v", err)
	}
	return plain
}

// javaServerEncryptResponse 模拟服务端 mode2 回响应：
// Base64(respJson) -> 每 245 字节块裸 RSA 私钥加密(m^d mod n) -> 左侧 0 填充到 k -> 拼接 -> Base64
func javaServerEncryptResponse(t *testing.T, priv *rsa.PrivateKey, respJSON []byte) []byte {
	b64 := []byte(base64.StdEncoding.EncodeToString(respJSON))
	k := priv.Size()
	const maxChunk = 245
	var out bytes.Buffer
	for off := 0; off < len(b64); off += maxChunk {
		end := off + maxChunk
		if end > len(b64) {
			end = len(b64)
		}
		m := new(big.Int).SetBytes(b64[off:end])
		s := new(big.Int).Exp(m, priv.D, priv.N)
		out.Write(s.FillBytes(make([]byte, k)))
	}
	return []byte(base64.StdEncoding.EncodeToString(out.Bytes()))
}

func newRSAClientWithKey(t *testing.T, bits int) (*Client, *rsa.PrivateKey) {
	priv, err := rsa.GenerateKey(rand.Reader, bits)
	if err != nil {
		t.Fatalf("gen key: %v", err)
	}
	der, err := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	if err != nil {
		t.Fatalf("marshal pkix: %v", err)
	}
	pubB64 := base64.StdEncoding.EncodeToString(der)
	cli, err := New("http://x", model.SmsAuthRSA, "", pubB64, "", 5)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return cli, priv
}

func TestRSARoundTrip2048(t *testing.T) {
	cli, priv := newRSAClientWithKey(t, 2048)

	plain := []byte(`{"data":{"type":1,"page_num":1,"page_size":20,"keyword":""},"timestamp":1700000000000,"sign":""}`)
	body, ct, err := cli.encodeRequest(plain)
	if err != nil {
		t.Fatalf("encodeRequest: %v", err)
	}
	if ct != "text/plain; charset=utf-8" {
		t.Fatalf("content type = %q", ct)
	}
	got := javaServerDecryptRequest(t, priv, body)
	if !bytes.Equal(got, plain) {
		t.Fatalf("request roundtrip mismatch:\n got=%s\nwant=%s", got, plain)
	}

	respJSON := []byte(`{"code":200,"msg":"success","data":[{"number":"10086","content":"hi"}],"timestamp":1700000000001}`)
	respBody := javaServerEncryptResponse(t, priv, respJSON)
	out, err := cli.decodeResponse(respBody)
	if err != nil {
		t.Fatalf("decodeResponse: %v", err)
	}
	if !bytes.Equal(out, respJSON) {
		t.Fatalf("response roundtrip mismatch:\n got=%s\nwant=%s", out, respJSON)
	}
}

func TestRSARoundTripLargePayloadMultiBlock(t *testing.T) {
	cli, priv := newRSAClientWithKey(t, 2048)

	big := make([]byte, 1200)
	for i := range big {
		big[i] = byte('a' + i%26)
	}
	plain := append([]byte(`{"data":"`), append(big, []byte(`","timestamp":1,"sign":""}`)...)...)

	body, _, err := cli.encodeRequest(plain)
	if err != nil {
		t.Fatalf("encodeRequest: %v", err)
	}
	if got := javaServerDecryptRequest(t, priv, body); !bytes.Equal(got, plain) {
		t.Fatalf("multi-block request mismatch")
	}
	respBody := javaServerEncryptResponse(t, priv, plain)
	out, err := cli.decodeResponse(respBody)
	if err != nil {
		t.Fatalf("decodeResponse: %v", err)
	}
	if !bytes.Equal(out, plain) {
		t.Fatalf("multi-block response mismatch")
	}
}

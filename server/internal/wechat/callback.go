package wechat

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha1"
	"crypto/subtle"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
)

type OuterEnvelope struct {
	XMLName   xml.Name `xml:"xml"`
	ToUserName string  `xml:"ToUserName"`
	Encrypt   string   `xml:"Encrypt"`
}

type EventMessage struct {
	XMLName                      xml.Name `xml:"xml"`
	ToUserName                   string   `xml:"ToUserName"`
	FromUserName                 string   `xml:"FromUserName"`
	AppID                        string   `xml:"AppId"`
	CreateTime                   int64    `xml:"CreateTime"`
	InfoType                     string   `xml:"InfoType"`
	AuthorizerAppID              string   `xml:"AuthorizerAppid"`
	AuthorizationCode            string   `xml:"AuthorizationCode"`
	AuthorizationCodeExpiredTime int64    `xml:"AuthorizationCodeExpiredTime"`
	PreAuthCode                  string   `xml:"PreAuthCode"`
	ComponentVerifyTicket        string   `xml:"ComponentVerifyTicket"`
	MsgType                      string   `xml:"MsgType"`
	Event                        string   `xml:"Event"`
	Content                      string   `xml:"Content"`
	MsgID                        int64    `xml:"MsgId"`
}

func URLSignature(token, timestamp, nonce string) string {
	parts := []string{token, timestamp, nonce}
	sort.Strings(parts)
	sum := sha1.Sum([]byte(strings.Join(parts, "")))
	return hex.EncodeToString(sum[:])
}

func MessageSignature(token, timestamp, nonce, encrypt string) string {
	parts := []string{token, timestamp, nonce, encrypt}
	sort.Strings(parts)
	sum := sha1.Sum([]byte(strings.Join(parts, "")))
	return hex.EncodeToString(sum[:])
}

func VerifyURL(token, signature, timestamp, nonce string) bool {
	expected := URLSignature(token, timestamp, nonce)
	return subtle.ConstantTimeCompare([]byte(expected), []byte(signature)) == 1
}

func VerifyMessage(token, signature, timestamp, nonce, encrypt string) bool {
	expected := MessageSignature(token, timestamp, nonce, encrypt)
	return subtle.ConstantTimeCompare([]byte(expected), []byte(signature)) == 1
}

func ParseEnvelope(body []byte) (*OuterEnvelope, error) {
	var env OuterEnvelope
	if err := xml.Unmarshal(body, &env); err != nil {
		return nil, fmt.Errorf("parse callback xml: %w", err)
	}
	return &env, nil
}

func DecryptMessage(encodingAESKey, encrypted string) ([]byte, string, error) {
	key, err := decodeAESKey(encodingAESKey)
	if err != nil {
		return nil, "", err
	}
	ciphertext, err := base64.StdEncoding.DecodeString(encrypted)
	if err != nil {
		return nil, "", fmt.Errorf("decode encrypted message: %w", err)
	}
	if len(ciphertext) == 0 || len(ciphertext)%aes.BlockSize != 0 {
		return nil, "", errors.New("invalid ciphertext length")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, "", err
	}
	iv := key[:aes.BlockSize]
	plain := make([]byte, len(ciphertext))
	cipher.NewCBCDecrypter(block, iv).CryptBlocks(plain, ciphertext)
	plain, err = pkcs7Unpad(plain, 32)
	if err != nil {
		return nil, "", err
	}
	if len(plain) < 20 {
		return nil, "", errors.New("decrypted message too short")
	}
	msgLen := int(binary.BigEndian.Uint32(plain[16:20]))
	if msgLen < 0 || 20+msgLen > len(plain) {
		return nil, "", errors.New("invalid message length")
	}
	msg := plain[20 : 20+msgLen]
	appID := string(plain[20+msgLen:])
	return msg, appID, nil
}

func EncryptMessage(encodingAESKey, msg, appID string) (string, error) {
	key, err := decodeAESKey(encodingAESKey)
	if err != nil {
		return "", err
	}
	buf := new(bytes.Buffer)
	random := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, random); err != nil {
		return "", err
	}
	buf.Write(random)
	length := make([]byte, 4)
	binary.BigEndian.PutUint32(length, uint32(len(msg)))
	buf.Write(length)
	buf.WriteString(msg)
	buf.WriteString(appID)
	plain := pkcs7Pad(buf.Bytes(), 32)
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	iv := key[:aes.BlockSize]
	out := make([]byte, len(plain))
	cipher.NewCBCEncrypter(block, iv).CryptBlocks(out, plain)
	return base64.StdEncoding.EncodeToString(out), nil
}

func ParseEvent(body []byte) (*EventMessage, error) {
	var msg EventMessage
	if err := xml.Unmarshal(body, &msg); err != nil {
		return nil, fmt.Errorf("parse event xml: %w", err)
	}
	return &msg, nil
}

func decodeAESKey(encodingAESKey string) ([]byte, error) {
	if len(encodingAESKey) != 43 {
		return nil, fmt.Errorf("EncodingAESKey must be 43 characters, got %d", len(encodingAESKey))
	}
	key, err := base64.StdEncoding.DecodeString(encodingAESKey + "=")
	if err != nil {
		return nil, fmt.Errorf("decode EncodingAESKey: %w", err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("EncodingAESKey decoded to %d bytes, want 32", len(key))
	}
	return key, nil
}

func pkcs7Pad(data []byte, blockSize int) []byte {
	padding := blockSize - len(data)%blockSize
	if padding == 0 {
		padding = blockSize
	}
	return append(data, bytes.Repeat([]byte{byte(padding)}, padding)...)
}

func pkcs7Unpad(data []byte, blockSize int) ([]byte, error) {
	if len(data) == 0 || len(data)%blockSize != 0 {
		return nil, errors.New("invalid padded data length")
	}
	padding := int(data[len(data)-1])
	if padding < 1 || padding > blockSize || padding > len(data) {
		return nil, errors.New("invalid padding")
	}
	for _, b := range data[len(data)-padding:] {
		if int(b) != padding {
			return nil, errors.New("invalid padding bytes")
		}
	}
	return data[:len(data)-padding], nil
}

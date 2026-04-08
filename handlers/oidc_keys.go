package handlers

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"log"
	"math/big"
	"os"
	"sync"
)

var (
	oidcPrivateKey *rsa.PrivateKey
	oidcKeyID      string
	oidcKeyOnce    sync.Once
)

// 启动时调用，加载或生成 RSA 密钥
func InitOIDCKeys(keyPath string) error {
	var err error
	oidcKeyOnce.Do(func() {
		err = loadOrGenerateKey(keyPath)
		if err != nil {
			return
		}
		// 用公钥指纹做 kid
		pubDER, _ := x509.MarshalPKIXPublicKey(&oidcPrivateKey.PublicKey)
		hash := sha256.Sum256(pubDER)
		oidcKeyID = base64.RawURLEncoding.EncodeToString(hash[:8])
		log.Printf("OIDC RSA key loaded, kid=%s", oidcKeyID)
	})
	return err
}

func loadOrGenerateKey(keyPath string) error {
	// 尝试从文件加载
	data, err := os.ReadFile(keyPath)
	if err == nil {
		block, _ := pem.Decode(data)
		if block != nil {
			key, parseErr := x509.ParsePKCS1PrivateKey(block.Bytes)
			if parseErr == nil {
				oidcPrivateKey = key
				return nil
			}
		}
	}

	// 文件不存在或解析失败，生成新的
	log.Println("Generating new OIDC RSA key pair...")
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return fmt.Errorf("generate RSA key failed: %w", err)
	}
	oidcPrivateKey = key

	// 保存到文件
	dir := keyPath[:max(0, len(keyPath)-len("/oidc_rsa.key"))]
	if dir == "" {
		dir = "."
	}
	_ = os.MkdirAll(dir, 0755)

	pemData := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})
	if err := os.WriteFile(keyPath, pemData, 0600); err != nil {
		log.Printf("Failed to save OIDC key to %s: %v (key is in-memory only)", keyPath, err)
	}

	return nil
}

// 返回 JWKS 格式的公钥
func buildJWKS() map[string]interface{} {
	pub := &oidcPrivateKey.PublicKey
	return map[string]interface{}{
		"keys": []map[string]interface{}{
			{
				"kty": "RSA",
				"use": "sig",
				"alg": "RS256",
				"kid": oidcKeyID,
				"n":   base64URLEncodeBigInt(pub.N),
				"e":   base64URLEncodeBigInt(big.NewInt(int64(pub.E))),
			},
		},
	}
}

func base64URLEncodeBigInt(n *big.Int) string {
	return base64.RawURLEncoding.EncodeToString(n.Bytes())
}

package secret

import (
	"testing"

	"mayfly-go/internal/pkg/config"
)

func TestCodecEncryptsAndRestoresCredential(t *testing.T) {
	codec := NewAESCodec(config.Aes{Key: "0123456789abcdef"})
	ciphertext, err := codec.Encrypt("fixture-secret")
	if err != nil || ciphertext == "fixture-secret" || ciphertext == "" {
		t.Fatalf("凭证必须加密存储: %v", err)
	}
	plaintext, err := codec.Decrypt(ciphertext)
	if err != nil || plaintext != "fixture-secret" {
		t.Fatalf("凭证解密失败: %v", err)
	}
}

func TestCodecRejectsMissingEncryptionKey(t *testing.T) {
	codec := NewAESCodec(config.Aes{})
	if _, err := codec.Encrypt("fixture-secret"); err == nil {
		t.Fatal("没有 AES 密钥时不得保存明文凭证")
	}
	if _, err := codec.Decrypt("fixture-secret"); err == nil {
		t.Fatal("没有 AES 密钥时不得把密文当明文使用")
	}
}

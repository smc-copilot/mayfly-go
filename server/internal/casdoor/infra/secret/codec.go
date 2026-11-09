package secret

import (
	"fmt"

	"mayfly-go/internal/casdoor/domain/repository"
	"mayfly-go/internal/pkg/config"
)

type aesCodec struct {
	config config.Aes
}

func NewAESCodec(aes config.Aes) repository.Secrets {
	return &aesCodec{config: aes}
}

func (c *aesCodec) Encrypt(value string) (string, error) {
	if err := c.checkKey(); err != nil {
		return "", err
	}
	return c.config.EncryptBase64([]byte(value))
}

func (c *aesCodec) Decrypt(value string) (string, error) {
	if err := c.checkKey(); err != nil {
		return "", err
	}
	decoded, err := c.config.DecryptBase64(value)
	if err != nil {
		return "", err
	}
	return string(decoded), nil
}

func (c *aesCodec) checkKey() error {
	switch len(c.config.Key) {
	case 16, 24, 32:
		return nil
	default:
		return fmt.Errorf("casdoor credential encryption key is not configured")
	}
}

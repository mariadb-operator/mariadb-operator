package config

import (
	"errors"
	"fmt"
	"strings"
)

type kvOption struct {
	key    string
	value  string
	quoted bool
}

func newKvOption(key, value string, quoted bool) *kvOption {
	return &kvOption{
		key:    key,
		value:  value,
		quoted: quoted,
	}
}

func (k *kvOption) marshal() string {
	if k.quoted {
		return fmt.Sprintf("%s=\"%s\"", k.key, k.value)
	} else {
		return fmt.Sprintf("%s=%s", k.key, k.value)
	}
}

func (k *kvOption) unmarshal(text string) error {
	str := strings.TrimSpace(text)
	if len(str) == 0 {
		return errors.New("empty input")
	}

	parts := strings.SplitN(str, "=", 2)
	if len(parts) < 2 {
		return fmt.Errorf("invalid input: %s", str)
	}

	key := strings.TrimSpace(parts[0])
	value := strings.TrimSpace(parts[1])
	quoted := false

	if len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"' {
		value = value[1 : len(value)-1]
		quoted = true
	}

	k.key = key
	k.value = value
	k.quoted = quoted
	return nil
}

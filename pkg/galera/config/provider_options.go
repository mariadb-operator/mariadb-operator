package config

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

const providerOptsDelimiter = ";"

type providerOptions struct {
	opts map[string]string
}

func newProviderOptions(opts map[string]string) *providerOptions {
	return &providerOptions{
		opts: opts,
	}
}

func (p *providerOptions) marshal() string {
	keys := make([]string, 0)
	for key := range p.opts {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var opts []string
	for _, key := range keys {
		kvOpt := newKvOption(key, p.opts[key], false)
		opts = append(opts, kvOpt.marshal())
	}
	return strings.Join(opts, providerOptsDelimiter)
}

func (p *providerOptions) unmarshal(text string) error {
	str := strings.TrimSpace(text)
	if len(str) == 0 {
		return errors.New("empty input")
	}
	if p.opts == nil {
		p.opts = make(map[string]string, 0)
	}

	opts := strings.Split(str, providerOptsDelimiter)
	for _, opt := range opts {
		var kvOpt kvOption
		if err := kvOpt.unmarshal(opt); err != nil {
			return fmt.Errorf("error unmarshaling option: %v", err)
		}

		p.opts[kvOpt.key] = kvOpt.value
	}
	return nil
}

func (p *providerOptions) update(opts map[string]string) {
	if p.opts == nil {
		p.opts = make(map[string]string, 0)
	}

	for k, v := range opts {
		p.opts[k] = v
	}
}

package config

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
)

const providerOptsDelimiter = "; "

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

	buffer := new(bytes.Buffer)
	idx := 0
	for _, key := range keys {
		kvOpt := newKvOption(key, p.opts[key], false)

		fmt.Fprint(buffer, kvOpt.marshal())
		if idx++; idx != len(p.opts) {
			fmt.Fprint(buffer, providerOptsDelimiter)
		}
	}
	return buffer.String()
}

func (p *providerOptions) unmarshal(text string) error {
	if p.opts == nil {
		p.opts = make(map[string]string, 0)
	}

	kvOpts := strings.Split(string(text), providerOptsDelimiter)
	for _, kvo := range kvOpts {
		var kvOpt kvOption
		if err := kvOpt.unmarshal(kvo); err != nil {
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

package config

import (
	"bytes"
	"fmt"
	"text/template"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"k8s.io/utils/ptr"
)

type tplOpts struct {
	Threads                  string
	QueryClassifierCacheSize string
	PersistRuntimeChanges    bool
	LoadPersistentConfigs    bool
	AdminHost                string
	AdminPort                int32
	AdminGui                 bool
	AdminSecureGui           bool
	Params                   map[string]string
}

var existingConfigKeys = map[string]struct{}{
	"threads":                     {},
	"query_classifier_cache_size": {},
	"persist_runtime_changes":     {},
	"load_persisted_configs":      {},
	"admin_host":                  {},
	"admin_port":                  {},
	"admin_gui":                   {},
	"admin_secure_gui":            {},
}

func Config(mxs *mariadbv1alpha1.MaxScale) ([]byte, error) {
	tpl := createTpl(mxs.ConfigSecretKeyRef().Key, `[maxscale]
threads={{ .Threads }}
{{- if .QueryClassifierCacheSize }}
query_classifier_cache_size={{ .QueryClassifierCacheSize }}
{{- end }}
persist_runtime_changes={{ .PersistRuntimeChanges }}
load_persisted_configs={{ .LoadPersistentConfigs }}
admin_host={{ .AdminHost }}
admin_port={{ .AdminPort }}
admin_gui={{ .AdminGui }}
admin_secure_gui={{ .AdminSecureGui }}
{{ range $key,$value := .Params }}
{{- $key }}={{ $value }}
{{ end }}`)
	buf := new(bytes.Buffer)
	err := tpl.Execute(buf, tplOpts{
		Threads:                  configValueOrDefault("threads", mxs.Spec.Config.Params, threads(mxs)),
		QueryClassifierCacheSize: configValueOrDefault("query_classifier_cache_size", mxs.Spec.Config.Params, queryClassifierCacheSize(mxs)),
		PersistRuntimeChanges:    true,
		LoadPersistentConfigs:    true,
		AdminHost:                configValueOrDefault("admin_host", mxs.Spec.Config.Params, "0.0.0.0"),
		AdminPort:                mxs.Spec.Admin.Port,
		AdminGui:                 ptr.Deref(mxs.Spec.Admin.GuiEnabled, true),
		AdminSecureGui:           false,
		Params:                   filterExistingConfig(mxs.Spec.Config.Params),
	})
	if err != nil {
		return nil, fmt.Errorf("error rendering MaxScale config: %v", err)
	}
	return buf.Bytes(), nil
}

func configValueOrDefault[T any](key string, params map[string]string, defaultVal T) string {
	if v, ok := params[key]; v != "" && ok {
		return v
	}
	return fmt.Sprint(defaultVal)
}

func filterExistingConfig(params map[string]string) map[string]string {
	config := make(map[string]string)
	for k, v := range params {
		if _, ok := existingConfigKeys[k]; !ok {
			config[k] = v
		}
	}
	return config
}

func createTpl(name, t string) *template.Template {
	return template.Must(template.New(name).Parse(t))
}

package maxscale

import (
	"bytes"
	"fmt"
	"text/template"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
)

type tplOpts struct {
	Threads               string
	LoadPersistentConfigs bool
	AdminHost             string
	AdminPort             int32
	AdminGui              bool
	AdminSecureGui        bool
	Params                map[string]string
}

var existingConfigKeys = map[string]bool{
	"threads":                true,
	"load_persisted_configs": true,
	"admin_host":             true,
	"admin_gui":              true,
	"admin_secure_gui":       true,
}

func Config(maxscale *mariadbv1alpha1.MaxScale) ([]byte, error) {
	tpl := createTpl(maxscale.ConfigSecretKeyRef().Key, `[maxscale]
threads={{ .Threads }}
load_persisted_configs={{ .LoadPersistentConfigs }}
admin_host={{ .AdminHost }}
admin_port={{ .AdminPort }}
admin_gui={{ .AdminGui }}
admin_secure_gui={{ .AdminSecureGui }}
{{ range $key,$value := .Params }}
{{- $key }}={{ $value }}
{{- end }}`)
	buf := new(bytes.Buffer)
	err := tpl.Execute(buf, tplOpts{
		Threads:               configValueOrDefault("threads", maxscale.Spec.Config.Params, "auto"),
		LoadPersistentConfigs: true,
		AdminHost:             configValueOrDefault("admin_host", maxscale.Spec.Config.Params, "0.0.0.0"),
		AdminPort:             maxscale.Spec.Admin.Port,
		AdminGui:              *maxscale.Spec.Admin.GuiEnabled,
		AdminSecureGui:        false,
		Params:                filterExistingConfig(maxscale.Spec.Config.Params),
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
	config := make(map[string]string, 0)
	for k, v := range params {
		if !existingConfigKeys[k] {
			config[k] = v
		}
	}
	return config
}

func createTpl(name, t string) *template.Template {
	return template.Must(template.New(name).Parse(t))
}

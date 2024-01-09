package maxscale

import (
	"bytes"
	"fmt"
	"text/template"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
)

type tplOpts struct {
	Threads                  string
	QueryClassifierCacheSize string
	AdminGui                 bool
	AdminHost                string
	AdminPort                int
	AdminSecureGui           bool
}

func Config(maxscale *mariadbv1alpha1.MaxScale) ([]byte, error) {
	tpl := createTpl(maxscale.ConfigSecretKeyRef().Key, `[maxscale]
threads={{ .Threads }}
{{- with .QueryClassifierCacheSize }}
query_classifier_cache_size={{ . }}
{{- end }}
admin_gui={{ .AdminGui }}
admin_host={{ .AdminHost }}
admin_port={{ .AdminPort }}
admin_secure_gui={{ .AdminSecureGui }}`)
	buf := new(bytes.Buffer)
	err := tpl.Execute(buf, tplOpts{
		Threads:        "auto",
		AdminGui:       true,
		AdminHost:      "0.0.0.0",
		AdminPort:      8989,
		AdminSecureGui: false,
	})
	if err != nil {
		return nil, fmt.Errorf("error rendering MaxScale config: %v", err)
	}
	return buf.Bytes(), nil
}

func createTpl(name, t string) *template.Template {
	return template.Must(template.New(name).Parse(t))
}

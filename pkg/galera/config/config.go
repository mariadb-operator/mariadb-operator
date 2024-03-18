package config

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"maps"
	"net"
	"sort"
	"strings"
	"text/template"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/pkg/environment"
	options "github.com/mariadb-operator/mariadb-operator/pkg/galera/options"
	"github.com/mariadb-operator/mariadb-operator/pkg/galera/recovery"
	"github.com/mariadb-operator/mariadb-operator/pkg/statefulset"
	"k8s.io/utils/ptr"
)

const (
	ConfigFileName      = "0-galera.cnf"
	WsrepNodeAddressKey = "wsrep_node_address"
	BootstrapFileName   = recovery.BootstrapFileName
)

var BootstrapFile = []byte(`[galera]
wsrep_new_cluster="ON"`)

type ConfigFile struct {
	mariadb *mariadbv1alpha1.MariaDB
}

func NewConfigFile(mariadb *mariadbv1alpha1.MariaDB) *ConfigFile {
	return &ConfigFile{
		mariadb: mariadb,
	}
}

func (c *ConfigFile) Marshal(podEnv *environment.PodEnvironment) ([]byte, error) {
	if !c.mariadb.IsGaleraEnabled() {
		return nil, errors.New("MariaDB Galera not enabled, unable to render config file")
	}
	galera := ptr.Deref(c.mariadb.Spec.Galera, mariadbv1alpha1.Galera{})

	tpl := createTpl("galera", `[mariadb]
bind_address=*
default_storage_engine=InnoDB
binlog_format=row
innodb_autoinc_lock_mode=2

# Cluster configuration
wsrep_on=ON
wsrep_provider={{ .GaleraLibPath }}
wsrep_cluster_address="{{ .ClusterAddress }}"
wsrep_cluster_name=mariadb-operator
wsrep_slave_threads={{ .Threads }}

# Node configuration
{{ .NodeAddressKey }}="{{ .NodeAddress }}"
wsrep_node_name="{{ .Pod }}"
wsrep_sst_method="{{ .SST }}"
wsrep_provider_options = "{{ .ProviderOpts }}"
wsrep_sst_receive_address = "{{ .WrappedNodeAddress }}:4444"
{{- if .SSTAuth }}
wsrep_sst_auth="root:{{ .RootPassword }}"
{{- end }}
`)
	buf := new(bytes.Buffer)
	clusterAddr, err := c.clusterAddress()
	if err != nil {
		return nil, fmt.Errorf("error getting cluster address: %v", err)
	}
	sst, err := galera.SST.MariaDBFormat()
	if err != nil {
		return nil, fmt.Errorf("error getting SST: %v", err)
	}
	wrappedPodIP, err := c.wrapIPAddress(podEnv.PodIP)
	if err != nil {
		return nil, fmt.Errorf("error wrapping address: %v", err)
	}
	gcommListenAddress, err := c.thisHostIP(podEnv.PodIP)
	if err != nil {
		return nil, fmt.Errorf("error getting address: %v", err)
	}
	gcommListenAddress, err = c.wrapIPAddress(gcommListenAddress)
	if err != nil {
		return nil, fmt.Errorf("error wrapping address: %v", err)
	}
	providerOpts := map[string]string{
		options.WSREPOptGmcastListAddr: fmt.Sprintf("tcp://%s:4567", gcommListenAddress),
		options.WSREPOptISTRecvAddr:    fmt.Sprintf("%s:4568", wrappedPodIP),
	}
	maps.Copy(providerOpts, galera.ProviderOptions)
	providerOptsSemColSeparated := c.mapToSemColSeparated(providerOpts)

	err = tpl.Execute(buf, struct {
		ClusterAddress     string
		NodeAddressKey     string
		NodeAddress        string
		WrappedNodeAddress string
		ProviderOpts       string
		GaleraLibPath      string
		Threads            int
		Pod                string
		SST                string
		SSTAuth            bool
		RootPassword       string
	}{
		ClusterAddress:     clusterAddr,
		NodeAddressKey:     WsrepNodeAddressKey,
		NodeAddress:        podEnv.PodIP,
		WrappedNodeAddress: wrappedPodIP,
		ProviderOpts:       providerOptsSemColSeparated,
		GaleraLibPath:      galera.GaleraLibPath,
		Threads:            galera.ReplicaThreads,
		Pod:                podEnv.PodName,
		SST:                sst,
		SSTAuth:            galera.SST == mariadbv1alpha1.SSTMariaBackup || galera.SST == mariadbv1alpha1.SSTMysqldump,
		RootPassword:       podEnv.MariadbRootPassword,
	})
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (c *ConfigFile) clusterAddress() (string, error) {
	if c.mariadb.Spec.Replicas == 0 {
		return "", errors.New("at least one replica must be specified to get a valid cluster address")
	}
	pods := make([]string, c.mariadb.Spec.Replicas)
	for i := 0; i < int(c.mariadb.Spec.Replicas); i++ {
		pods[i] = statefulset.PodFQDNWithService(
			c.mariadb.ObjectMeta,
			i,
			c.mariadb.InternalServiceKey().Name,
		)
	}
	return fmt.Sprintf("gcomm://%s", strings.Join(pods, ",")), nil
}

func (c *ConfigFile) wrapIPAddress(ip string) (string, error) {
	parsedIp := net.ParseIP(ip)
	if parsedIp == nil {
		return "", fmt.Errorf("error in parsing ip address: %v", ip)
	}

	if parsedIp.To4() == nil {
		ip = fmt.Sprintf("[%s]", ip)
	}
	return ip, nil
}

func (c *ConfigFile) thisHostIP(ip string) (string, error) {
	parsedIp := net.ParseIP(ip)
	if parsedIp == nil {
		return "", fmt.Errorf("error in parsing ip address: %v", ip)
	}

	hostIP := ""
	if parsedIp.To4() != nil {
		hostIP = "0.0.0.0"
	} else {
		hostIP = "::"
	}

	return hostIP, nil
}

func (c *ConfigFile) mapToSemColSeparated(wsrepOpts map[string]string) string {
	keys := make([]string, 0)
	for key := range wsrepOpts {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	buffer := new(bytes.Buffer)
	idx := 0
	for _, key := range keys {
		fmt.Fprintf(buffer, "%s=%s", key, wsrepOpts[key])
		if idx++; idx != len(wsrepOpts) {
			fmt.Fprintf(buffer, "; ")
		}
	}
	return buffer.String()
}

func UpdateConfigFile(configBytes []byte, key, value string) ([]byte, error) {
	fileScanner := bufio.NewScanner(bytes.NewReader(configBytes))
	fileScanner.Split(bufio.ScanLines)

	var updatedLines []string
	matched := false

	for fileScanner.Scan() {
		line := fileScanner.Text()

		if strings.HasPrefix(line, key) {
			updatedLines = append(updatedLines, fmt.Sprintf("%s=\"%s\"", key, value))
			matched = true
		} else {
			updatedLines = append(updatedLines, line)
		}
	}
	if err := fileScanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading config: %v", err)
	}
	if !matched {
		return nil, fmt.Errorf("config key '%s' not found", key)
	}

	updatedConfig := []byte(strings.Join(updatedLines, "\n"))
	return updatedConfig, nil
}

func createTpl(name, t string) *template.Template {
	return template.Must(template.New(name).Parse(t))
}

package config

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"maps"
	"net"
	"strings"
	"text/template"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/pkg/environment"
	galerakeys "github.com/mariadb-operator/mariadb-operator/pkg/galera/config/keys"
	"github.com/mariadb-operator/mariadb-operator/pkg/galera/recovery"
	"github.com/mariadb-operator/mariadb-operator/pkg/statefulset"
	"k8s.io/utils/ptr"
)

const (
	ConfigFileName    = "0-galera.cnf"
	BootstrapFileName = recovery.BootstrapFileName
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
{{ .ProviderOptsKey }}="{{ .ProviderOpts }}"
{{ .SSTReceiveAddressKey }}="{{ .SSTReceiveAddress }}"
{{- if .SSTAuth }}
wsrep_sst_auth="root:{{ .RootPassword }}"
{{- end }}
`)
	buf := new(bytes.Buffer)
	clusterAddr, err := c.clusterAddress()
	if err != nil {
		return nil, fmt.Errorf("error getting cluster address: %v", err)
	}
	providerOptions, err := getProviderOptions(podEnv.PodIP, galera.ProviderOptions)
	if err != nil {
		return nil, fmt.Errorf("error getting provider options: %v", err)
	}
	sstReceiveAddress, err := getSSTReceiveAddress(podEnv.PodIP)
	if err != nil {
		return nil, fmt.Errorf("error getting SST receive address: %v", err)
	}
	sst, err := galera.SST.MariaDBFormat()
	if err != nil {
		return nil, fmt.Errorf("error getting SST: %v", err)
	}

	err = tpl.Execute(buf, struct {
		ClusterAddress string

		NodeAddressKey string
		NodeAddress    string

		ProviderOptsKey string
		ProviderOpts    string

		SSTReceiveAddressKey string
		SSTReceiveAddress    string

		GaleraLibPath string
		Threads       int
		Pod           string
		SST           string
		SSTAuth       bool
		RootPassword  string
	}{
		ClusterAddress: clusterAddr,

		NodeAddressKey: galerakeys.WsrepNodeAddressKey,
		NodeAddress:    podEnv.PodIP,

		ProviderOptsKey: galerakeys.WsrepProviderOptionsKey,
		ProviderOpts:    providerOptions,

		SSTReceiveAddressKey: galerakeys.WsrepSSTReceiveAddressKey,
		SSTReceiveAddress:    sstReceiveAddress,

		GaleraLibPath: galera.GaleraLibPath,
		Threads:       galera.ReplicaThreads,
		Pod:           podEnv.PodName,
		SST:           sst,
		SSTAuth:       galera.SST == mariadbv1alpha1.SSTMariaBackup || galera.SST == mariadbv1alpha1.SSTMysqldump,
		RootPassword:  podEnv.MariadbRootPassword,
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

func UpdateConfig(configBytes []byte, podEnv *environment.PodEnvironment) ([]byte, error) {
	fileScanner := bufio.NewScanner(bytes.NewReader(configBytes))
	fileScanner.Split(bufio.ScanLines)

	var updatedLines []string
	for fileScanner.Scan() {
		line, err := getUpdatedConfigLine(fileScanner.Text(), podEnv.PodIP)
		if err != nil {
			return nil, err
		}
		updatedLines = append(updatedLines, line)
	}
	if err := fileScanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading config: %v", err)
	}

	updatedConfig := []byte(strings.Join(updatedLines, "\n"))
	return updatedConfig, nil
}

func getProviderOptions(podIP string, options map[string]string) (string, error) {
	gmcastListenAddress, err := getGmcastListenAddress(podIP)
	if err != nil {
		return "", fmt.Errorf("error getting gcomm listden address: %v", err)
	}
	istReceiveAddress, err := getISTReceiveAddress(podIP)
	if err != nil {
		return "", fmt.Errorf("error getting IST receive address: %v", err)
	}

	wsrepOpts := map[string]string{
		galerakeys.WsrepOptGmcastListAddr: gmcastListenAddress,
		galerakeys.WsrepOptISTRecvAddr:    istReceiveAddress,
	}
	maps.Copy(wsrepOpts, options)

	providerOpts := newProviderOptions(wsrepOpts)
	return providerOpts.marshal(), nil
}

func getGmcastListenAddress(podIP string) (string, error) {
	gmcastListenAddress, err := thisHostIP(podIP)
	if err != nil {
		return "", fmt.Errorf("error getting address: %v", err)
	}
	gmcastListenAddress, err = wrapIPAddress(gmcastListenAddress)
	if err != nil {
		return "", fmt.Errorf("error wrapping address: %v", err)
	}
	return fmt.Sprintf("tcp://%s:4567", gmcastListenAddress), nil
}

func getISTReceiveAddress(podIP string) (string, error) {
	wrappedPodIP, err := wrapIPAddress(podIP)
	if err != nil {
		return "", fmt.Errorf("error wrapping address: %v", err)
	}
	return fmt.Sprintf("%s:4568", wrappedPodIP), nil
}

func getSSTReceiveAddress(podIP string) (string, error) {
	wrappedPodIP, err := wrapIPAddress(podIP)
	if err != nil {
		return "", fmt.Errorf("error wrapping address: %v", err)
	}
	return fmt.Sprintf("%s:4444", wrappedPodIP), nil
}

func getUpdatedConfigLine(line string, podIP string) (string, error) {
	if strings.HasPrefix(line, galerakeys.WsrepNodeAddressKey) {
		kvOpt := newKvOption(galerakeys.WsrepNodeAddressKey, podIP, true)
		return kvOpt.marshal(), nil
	}

	if strings.HasPrefix(line, galerakeys.WsrepProviderOptionsKey) {
		var kvOpt kvOption
		if err := kvOpt.unmarshal(line); err != nil {
			return "", err
		}
		var providerOpts providerOptions
		if err := providerOpts.unmarshal(kvOpt.value); err != nil {
			return "", err
		}

		gmcastListenAddress, err := getGmcastListenAddress(podIP)
		if err != nil {
			return "", fmt.Errorf("error getting gcomm listden address: %v", err)
		}
		istReceiveAddress, err := getISTReceiveAddress(podIP)
		if err != nil {
			return "", fmt.Errorf("error getting IST receive address: %v", err)
		}

		wsrepOpts := map[string]string{
			galerakeys.WsrepOptGmcastListAddr: gmcastListenAddress,
			galerakeys.WsrepOptISTRecvAddr:    istReceiveAddress,
		}
		providerOpts.update(wsrepOpts)

		updatedKvOpt := newKvOption(galerakeys.WsrepProviderOptionsKey, providerOpts.marshal(), true)
		return updatedKvOpt.marshal(), nil
	}

	if strings.HasPrefix(line, galerakeys.WsrepSSTReceiveAddressKey) {
		sstReceiveAddress, err := getSSTReceiveAddress(podIP)
		if err != nil {
			return "", err
		}

		kvOpt := newKvOption(galerakeys.WsrepSSTReceiveAddressKey, sstReceiveAddress, true)
		return kvOpt.marshal(), nil
	}

	return line, nil
}

func wrapIPAddress(ip string) (string, error) {
	parsedIp := net.ParseIP(ip)
	if parsedIp == nil {
		return "", fmt.Errorf("error in parsing ip address: %v", ip)
	}

	if parsedIp.To4() == nil {
		ip = fmt.Sprintf("[%s]", ip)
	}
	return ip, nil
}

func thisHostIP(ip string) (string, error) {
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

func createTpl(name, t string) *template.Template {
	return template.Must(template.New(name).Parse(t))
}

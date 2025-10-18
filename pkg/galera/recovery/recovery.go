package recovery

import (
	"bufio"
	"bytes"
	"fmt"
	"html/template"
	"strconv"
	"strings"

	"github.com/go-logr/logr"
	guuid "github.com/google/uuid"
)

const (
	BootstrapFileName = "1-bootstrap.cnf"
	BootstrapFile     = `[galera]
wsrep_new_cluster="ON"`
)

type GaleraRecoverer interface {
	GetUUID() string
	GetSeqno() int
	Compare(other GaleraRecoverer) int
}

type GaleraState struct {
	Version         string `json:"version"`
	UUID            string `json:"uuid"`
	Seqno           int    `json:"seqno"`
	SafeToBootstrap bool   `json:"safeToBootstrap"`
}

func (g *GaleraState) GetUUID() string {
	return g.UUID
}

func (g *GaleraState) GetSeqno() int {
	return g.Seqno
}

func (g *GaleraState) Compare(other GaleraRecoverer) int {
	if other == nil {
		return 1
	}
	if g.GetSeqno() < other.GetSeqno() {
		return -1
	}
	if g.GetSeqno() > other.GetSeqno() {
		return 1
	}
	return 0
}

func (g *GaleraState) Marshal() ([]byte, error) {
	if _, err := guuid.Parse(g.UUID); err != nil {
		return nil, fmt.Errorf("invalid uuid: %v", err)
	}
	type tplOpts struct {
		Version         string
		UUID            string
		Seqno           int
		SafeToBootstrap int
	}
	tpl := createTpl("grastate.dat", `version: {{ .Version }}
uuid: {{ .UUID }}
seqno: {{ .Seqno }}
safe_to_bootstrap: {{ .SafeToBootstrap }}`)
	buf := new(bytes.Buffer)
	err := tpl.Execute(buf, tplOpts{
		Version: g.Version,
		UUID:    g.UUID,
		Seqno:   g.Seqno,
		SafeToBootstrap: func() int {
			if g.SafeToBootstrap {
				return 1
			}
			return 0
		}(),
	})
	if err != nil {
		return nil, fmt.Errorf("error rendering template: %v", err)
	}
	return buf.Bytes(), nil
}

func (g *GaleraState) Unmarshal(text []byte) error {
	fileScanner := bufio.NewScanner(bytes.NewReader(text))
	fileScanner.Split(bufio.ScanLines)

	var version *string
	var uuid *string
	var seqno *int
	var safeToBootstrap *bool

	for fileScanner.Scan() {
		parts := strings.Split(fileScanner.Text(), ":")
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		switch key {
		case "version":
			version = &value
		case "uuid":
			if _, err := guuid.Parse(value); err != nil {
				return fmt.Errorf("error parsing uuid: %v", err)
			}
			uuid = &value
		case "seqno":
			i, err := strconv.Atoi(value)
			if err != nil {
				return fmt.Errorf("error parsing seqno: %v", err)
			}
			seqno = &i
		case "safe_to_bootstrap":
			b, err := parseBool(value)
			if err != nil {
				return fmt.Errorf("error parsing safe_to_bootstrap: %v", err)
			}
			safeToBootstrap = &b
		}
	}

	if version == nil || uuid == nil || seqno == nil || safeToBootstrap == nil {
		return fmt.Errorf(
			"invalid galera state file: version=%v uuid=%v seqno=%v safeToBootstrap=%v",
			version, uuid, seqno, safeToBootstrap,
		)
	}
	g.Version = *version
	g.UUID = *uuid
	g.Seqno = *seqno
	g.SafeToBootstrap = *safeToBootstrap
	return nil
}

type Bootstrap struct {
	UUID  string `json:"uuid"`
	Seqno int    `json:"seqno"`
}

func (b *Bootstrap) GetUUID() string {
	return b.UUID
}

func (b *Bootstrap) GetSeqno() int {
	return b.Seqno
}

func (b *Bootstrap) Compare(other GaleraRecoverer) int {
	if other == nil {
		return 1
	}
	if b.GetSeqno() < other.GetSeqno() {
		return -1
	}
	if b.GetSeqno() > other.GetSeqno() {
		return 1
	}
	return 0
}

func (b *Bootstrap) Validate() error {
	if _, err := guuid.Parse(b.UUID); err != nil {
		return fmt.Errorf("invalid uuid: %v", err)
	}
	return nil
}

func (b *Bootstrap) Unmarshal(text []byte, logger logr.Logger) error {
	fileScanner := bufio.NewScanner(bytes.NewReader(text))
	fileScanner.Split(bufio.ScanLines)

	var uuid *string
	var seqno *int

	for fileScanner.Scan() {
		parts := strings.Split(fileScanner.Text(), "WSREP: Recovered position: ")
		if len(parts) != 2 {
			continue
		}
		parts = strings.Split(parts[1], ":")
		if len(parts) != 2 {
			continue
		}

		currentUUID := strings.TrimSpace(parts[0])
		if _, err := guuid.Parse(currentUUID); err != nil {
			return fmt.Errorf("error parsing uuid: %v", err)
		}
		currentSeqno, err := parseSeqno(strings.TrimSpace(parts[1]), logger)
		if err != nil {
			return fmt.Errorf("error parsing seqno: %v", err)
		}
		uuid = &currentUUID
		seqno = &currentSeqno
	}
	if uuid == nil || seqno == nil {
		return fmt.Errorf(
			"unable to find uuid and seqno: uuid=%v seqno=%v",
			uuid, seqno,
		)
	}
	b.UUID = *uuid
	b.Seqno = *seqno
	return nil
}

func createTpl(name, t string) *template.Template {
	return template.Must(template.New(name).Parse(t))
}

func parseBool(s string) (bool, error) {
	i, err := strconv.Atoi(s)
	if err != nil {
		return false, fmt.Errorf("error parsing integer bool: %v", err)
	}
	if i != 0 && i != 1 {
		return false, fmt.Errorf("invalid integer bool: %d", i)
	}
	return i == 1, nil
}

func parseSeqno(rawSeqno string, logger logr.Logger) (int, error) {
	if !strings.Contains(rawSeqno, ",") {
		return strconv.Atoi(rawSeqno)
	}
	parts := strings.Split(rawSeqno, ",")

	for _, part := range parts {
		rawSeqno = strings.TrimSpace(part)
		if part == "" {
			logger.Info("Ignoring empty seqno")
			continue
		}
		seqno, err := strconv.Atoi(rawSeqno)
		if err != nil {
			logger.V(1).Info("Unable to parse seqno. Skipping...", "err", err)
			continue
		}
		return seqno, nil
	}
	return 0, fmt.Errorf("unable to parse seqno: %s", rawSeqno)
}

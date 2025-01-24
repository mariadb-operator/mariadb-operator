package client

import (
	"encoding/json"

	mdbhttp "github.com/mariadb-operator/mariadb-operator/pkg/http"
)

type ListenerParameters struct {
	Port                     int32     `json:"port"`
	Protocol                 string    `json:"protocol"`
	SSL                      bool      `json:"ssl,omitempty"`
	SSLCert                  string    `json:"ssl_cert,omitempty"`
	SSLKey                   string    `json:"ssl_key,omitempty"`
	SSLCA                    string    `json:"ssl_ca,omitempty"`
	SSLVersion               string    `json:"ssl_version,omitempty"`
	SSLVerifyPeerCertificate bool      `json:"ssl_verify_peer_certificate,omitempty"`
	SSLVerifyPeerHost        bool      `json:"ssl_verify_peer_host,omitempty"`
	Params                   MapParams `json:"-"`
}

func (l ListenerParameters) MarshalJSON() ([]byte, error) {
	type ListenerParametersInternal ListenerParameters // prevent recursion
	bytes, err := json.Marshal(ListenerParametersInternal(l))
	if err != nil {
		return nil, err
	}

	var rawMap map[string]json.RawMessage
	if err := json.Unmarshal(bytes, &rawMap); err != nil {
		return nil, err
	}

	for k, v := range l.Params {
		if _, ok := rawMap[k]; ok { // prevent overriding
			continue
		}
		bytes, err := json.Marshal(v)
		if err != nil {
			return nil, err
		}
		rawMap[k] = bytes
	}

	return json.Marshal(rawMap)
}

type ListenerAttributes struct {
	State      string             `json:"state,omitempty"`
	Parameters ListenerParameters `json:"parameters"`
}

type ListenerClient struct {
	GenericClient[ListenerAttributes]
}

func NewListenerClient(client *mdbhttp.Client) *ListenerClient {
	return &ListenerClient{
		GenericClient: NewGenericClient[ListenerAttributes](
			client,
			"listeners",
			ObjectTypeListeners,
		),
	}
}

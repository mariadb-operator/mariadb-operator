package config

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("kvOption marshal", func() {
	DescribeTable("marshaling a kvOption",
		func(kvOpt *kvOption, wantString string) {
			gotString := kvOpt.marshal()
			Expect(gotString).To(Equal(wantString))
		},
		Entry(
			"unquoted",
			newKvOption("gmcast.listen_addr", "tcp://0.0.0.0:4567", false),
			"gmcast.listen_addr=tcp://0.0.0.0:4567",
		),
		Entry(
			"unquoted with multiple values",
			newKvOption("wsrep_provider_options", "gcache.size=1G;gcs.fc_limit=128;gmcast.listen_addr=tcp://[::]:4567;ist.recv_addr=[2001:db8::a1]:4568", false),
			"wsrep_provider_options=gcache.size=1G;gcs.fc_limit=128;gmcast.listen_addr=tcp://[::]:4567;ist.recv_addr=[2001:db8::a1]:4568",
		),
		Entry(
			"quoted",
			newKvOption("wsrep_node_address", "10.244.0.33", true),
			"wsrep_node_address=\"10.244.0.33\"",
		),
		Entry(
			"quoted with multiple values",
			newKvOption("wsrep_provider_options", "gcache.size=1G;gcs.fc_limit=128;gmcast.listen_addr=tcp://[::]:4567;ist.recv_addr=[2001:db8::a1]:4568", true),
			"wsrep_provider_options=\"gcache.size=1G;gcs.fc_limit=128;gmcast.listen_addr=tcp://[::]:4567;ist.recv_addr=[2001:db8::a1]:4568\"",
		),
	)
})

var _ = Describe("kvOption unmarshal", func() {
	DescribeTable("unmarshaling a kvOption",
		func(rawKvOpt string, wantKvOpt kvOption, wantErr bool) {
			var kvOpt kvOption
			err := kvOpt.unmarshal(rawKvOpt)
			if !wantErr {
				Expect(err).NotTo(HaveOccurred())
			}
			Expect(kvOpt).To(Equal(wantKvOpt))
		},
		Entry(
			"empty",
			" ",
			kvOption{},
			true,
		),
		Entry(
			"invalid",
			"wsrep_node_address: 2001:db8::a2",
			kvOption{},
			true,
		),
		Entry(
			"unquoted",
			"wsrep_cluster_name=mariadb-operator",
			kvOption{
				key:    "wsrep_cluster_name",
				value:  "mariadb-operator",
				quoted: false,
			},
			false,
		),
		Entry(
			"quoted with multiple values",
			"wsrep_provider_options=gcache.size=1G;gcs.fc_limit=128;gmcast.listen_addr=tcp://[::]:4567;ist.recv_addr=[2001:db8::a1]:4568",
			kvOption{
				key:    "wsrep_provider_options",
				value:  "gcache.size=1G;gcs.fc_limit=128;gmcast.listen_addr=tcp://[::]:4567;ist.recv_addr=[2001:db8::a1]:4568",
				quoted: false,
			},
			false,
		),
		Entry(
			"quoted",
			"wsrep_sst_receive_address=\"[2001:db8::a2]:4444\"",
			kvOption{
				key:    "wsrep_sst_receive_address",
				value:  "[2001:db8::a2]:4444",
				quoted: true,
			},
			false,
		),
		Entry(
			"quoted with multiple values",
			"wsrep_provider_options=\"gcache.size=1G;gcs.fc_limit=128;gmcast.listen_addr=tcp://[::]:4567;ist.recv_addr=[2001:db8::a1]:4568\"",
			kvOption{
				key:    "wsrep_provider_options",
				value:  "gcache.size=1G;gcs.fc_limit=128;gmcast.listen_addr=tcp://[::]:4567;ist.recv_addr=[2001:db8::a1]:4568",
				quoted: true,
			},
			false,
		),
	)
})

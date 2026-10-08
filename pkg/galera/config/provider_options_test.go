package config

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("providerOptions marshal", func() {
	DescribeTable("marshaling providerOptions",
		func(providerOpts *providerOptions, wantString string) {
			gotString := providerOpts.marshal()
			Expect(gotString).To(Equal(wantString))
		},
		Entry(
			"empty",
			newProviderOptions(map[string]string{}),
			"",
		),
		Entry(
			"single option",
			newProviderOptions(map[string]string{
				"gmcast.listen_addr": "tcp://0.0.0.0:4567",
			}),
			"gmcast.listen_addr=tcp://0.0.0.0:4567",
		),
		Entry(
			"multiple options",
			newProviderOptions(map[string]string{
				"gcache.size":   "1G",
				"gcs.fc_limit":  "128",
				"ist.recv_addr": "[2001:db8::a1]:4568",
			}),
			"gcache.size=1G;gcs.fc_limit=128;ist.recv_addr=[2001:db8::a1]:4568",
		),
	)
})

var _ = Describe("providerOptions unmarshal", func() {
	DescribeTable("unmarshaling providerOptions",
		func(rawProviderOpts string, wantProviderOpts providerOptions, wantErr bool) {
			var providerOpts providerOptions
			err := providerOpts.unmarshal(rawProviderOpts)
			if !wantErr {
				Expect(err).NotTo(HaveOccurred())
			}
			Expect(providerOpts).To(Equal(wantProviderOpts))
		},
		Entry(
			"empty",
			" ",
			providerOptions{},
			true,
		),
		Entry(
			"single invalid option",
			"gmcast.listen_addr",
			providerOptions{
				make(map[string]string, 0),
			},
			true,
		),
		Entry(
			"multiple invalid options",
			"gmcast.listen_addr;ist.recv_addr",
			providerOptions{
				make(map[string]string, 0),
			},
			true,
		),
		Entry(
			"some invalid options",
			"gmcast.listen_addr=tcp://[::]:4567;gcs.fc_limit;ist.recv_addr",
			providerOptions{
				opts: map[string]string{
					"gmcast.listen_addr": "tcp://[::]:4567",
				},
			},
			true,
		),
		Entry(
			"single option",
			"gmcast.listen_addr=tcp://[::]:4567",
			providerOptions{
				opts: map[string]string{
					"gmcast.listen_addr": "tcp://[::]:4567",
				},
			},
			false,
		),
		Entry(
			"multiple options",
			"gcache.size=1G;gcs.fc_limit=128;gmcast.listen_addr=tcp://[::]:4567;ist.recv_addr=[2001:db8::a1]:4568",
			providerOptions{
				opts: map[string]string{
					"gcache.size":        "1G",
					"gcs.fc_limit":       "128",
					"gmcast.listen_addr": "tcp://[::]:4567",
					"ist.recv_addr":      "[2001:db8::a1]:4568",
				},
			},
			false,
		),
	)
})

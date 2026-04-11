package environment

import (
	"context"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("WatchNamespaces", func() {
	DescribeTable("returns namespaces from env",
		func(env map[string]string, wantNamespaces []string, wantErr bool) {
			for k, v := range env {
				DeferCleanup(os.Setenv, k, os.Getenv(k))
				os.Setenv(k, v)
			}
			operatorEnv, err := GetOperatorEnv(context.Background())
			if !wantErr {
				Expect(err).NotTo(HaveOccurred())
			}
			if operatorEnv == nil {
				return
			}
			namespaces, err := operatorEnv.WatchNamespaces()
			Expect(namespaces).To(Equal(wantNamespaces))
			if wantErr {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
			}
		},
		Entry("no env", map[string]string{}, nil, true),
		Entry("single namespace", map[string]string{"WATCH_NAMESPACE": "ns1"}, []string{"ns1"}, false),
		Entry("multiple namespaces",
			map[string]string{"WATCH_NAMESPACE": "ns1,ns2,ns3"},
			[]string{"ns1", "ns2", "ns3"},
			false,
		),
	)
})

var _ = Describe("CurrentNamespaceOnly", func() {
	DescribeTable("returns whether operator watches only its own namespace",
		func(env map[string]string, wantBool bool) {
			for k, v := range env {
				DeferCleanup(os.Setenv, k, os.Getenv(k))
				os.Setenv(k, v)
			}
			operatorEnv, err := GetOperatorEnv(context.Background())
			Expect(err).NotTo(HaveOccurred())
			if operatorEnv == nil {
				return
			}
			currentNamespaceOnly, err := operatorEnv.CurrentNamespaceOnly()
			Expect(currentNamespaceOnly).To(Equal(wantBool))
			Expect(err).NotTo(HaveOccurred())
		},
		Entry("no env", map[string]string{}, false),
		Entry("same namespace",
			map[string]string{"WATCH_NAMESPACE": "ns1", "MARIADB_OPERATOR_NAMESPACE": "ns1"},
			true,
		),
		Entry("other namespace",
			map[string]string{"WATCH_NAMESPACE": "ns2", "MARIADB_OPERATOR_NAMESPACE": "ns1"},
			false,
		),
		Entry("multiple namespaces",
			map[string]string{"WATCH_NAMESPACE": "ns1,ns2,ns3", "MARIADB_OPERATOR_NAMESPACE": "ns1"},
			false,
		),
		Entry("all namespaces",
			map[string]string{"WATCH_NAMESPACE": "", "MARIADB_OPERATOR_NAMESPACE": "ns1"},
			false,
		),
	)
})

var _ = Describe("TLSEnabled", func() {
	DescribeTable("returns whether TLS is enabled",
		func(env map[string]string, wantBool, wantErr bool) {
			for k, v := range map[string]string{
				"CLUSTER_NAME":          "test",
				"POD_NAME":              "mariadb-0",
				"POD_NAMESPACE":         "default",
				"POD_IP":                "10.244.0.11",
				"MARIADB_NAME":          "mariadb",
				"MARIADB_ROOT_PASSWORD": "MariaDB11!",
				"MYSQL_TCP_PORT":        "3306",
			} {
				DeferCleanup(os.Setenv, k, os.Getenv(k))
				os.Setenv(k, v)
			}
			for k, v := range env {
				DeferCleanup(os.Setenv, k, os.Getenv(k))
				os.Setenv(k, v)
			}
			podEnv, err := GetPodEnv(context.Background())
			Expect(err).NotTo(HaveOccurred())
			if podEnv == nil {
				return
			}
			isTLSEnabled, err := podEnv.IsTLSEnabled()
			if wantErr {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
			}
			Expect(isTLSEnabled).To(Equal(wantBool))
		},
		Entry("no env", map[string]string{}, false, false),
		Entry("empty", map[string]string{"TLS_ENABLED": ""}, false, false),
		Entry("invalid", map[string]string{"TLS_ENABLED": "foo"}, false, true),
		Entry("valid bool", map[string]string{"TLS_ENABLED": "true"}, true, false),
		Entry("valid number", map[string]string{"TLS_ENABLED": "1"}, true, false),
	)
})

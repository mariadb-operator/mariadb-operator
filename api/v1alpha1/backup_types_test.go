package v1alpha1

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Backup types", func() {
	objMeta := metav1.ObjectMeta{
		Name:      "backup-obj",
		Namespace: "backup-obj",
	}
	Context("When creating a Backup object", func() {
		DescribeTable(
			"Should default",
			func(backup, expected *Backup) {
				backup.SetDefaults()
				Expect(backup).To(BeEquivalentTo(expected))
			},
			Entry(
				"Emtpty",
				&Backup{
					ObjectMeta: objMeta,
				},
				&Backup{
					ObjectMeta: objMeta,
					Spec: BackupSpec{
						MaxRetention: metav1.Duration{Duration: 30 * 24 * time.Hour},
						BackoffLimit: 5,
					},
				},
			),
			Entry(
				"Full",
				&Backup{
					ObjectMeta: objMeta,
					Spec: BackupSpec{
						MaxRetention: metav1.Duration{Duration: 10 * 24 * time.Hour},
						BackoffLimit: 3,
					},
				},
				&Backup{
					ObjectMeta: objMeta,
					Spec: BackupSpec{
						MaxRetention: metav1.Duration{Duration: 10 * 24 * time.Hour},
						BackoffLimit: 3,
					},
				},
			),
		)
	})
})

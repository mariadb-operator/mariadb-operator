package service

import (
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
)

func Test_updateServicePorts(t *testing.T) {
	t.Run("Nil services", func(t *testing.T) {
		updateServicePorts(nil, nil)
	})

	t.Run("Existing service has no ports", func(t *testing.T) {
		existingSvc := &corev1.Service{}
		desiredSvc := &corev1.Service{
			Spec: corev1.ServiceSpec{
				Ports: []corev1.ServicePort{
					{
						Port:     3306,
						Protocol: corev1.ProtocolTCP,
					},
				},
			},
		}

		expectedSvc := &corev1.Service{
			Spec: corev1.ServiceSpec{
				Ports: []corev1.ServicePort{
					{
						Port:     3306,
						Protocol: corev1.ProtocolTCP,
					},
				},
			},
		}

		updateServicePorts(existingSvc, desiredSvc)

		if !reflect.DeepEqual(existingSvc, expectedSvc) {
			t.Errorf("updateServicePorts() = %v, want %v", existingSvc, expectedSvc)
		}
	})

	t.Run("Existing service has ports with overlapping ports", func(t *testing.T) {
		existingSvc := &corev1.Service{
			Spec: corev1.ServiceSpec{
				Ports: []corev1.ServicePort{
					{
						Port:     80,
						Protocol: corev1.ProtocolTCP,
					},
				},
			},
		}

		desiredSvc := &corev1.Service{
			Spec: corev1.ServiceSpec{
				Ports: []corev1.ServicePort{
					{
						Port:     80,
						Protocol: corev1.ProtocolTCP,
					},
					{
						Port:     3306,
						Protocol: corev1.ProtocolTCP,
					},
				},
			},
		}

		expectedSvc := &corev1.Service{
			Spec: corev1.ServiceSpec{
				Ports: []corev1.ServicePort{
					{
						Port:     80,
						Protocol: corev1.ProtocolTCP,
					},
					{
						Port:     3306,
						Protocol: corev1.ProtocolTCP,
					},
				},
			},
		}

		updateServicePorts(existingSvc, desiredSvc)

		if !reflect.DeepEqual(existingSvc, expectedSvc) {
			t.Errorf("updateServicePorts() = %v, want %v", existingSvc, expectedSvc)
		}
	})

	t.Run("Desired service has NodePort type and existing port has non-zero NodePort", func(t *testing.T) {
		existingSvc := &corev1.Service{
			Spec: corev1.ServiceSpec{
				Ports: []corev1.ServicePort{
					{
						Port:     3306,
						Protocol: corev1.ProtocolTCP,
						NodePort: 30000,
					},
				},
			},
		}

		desiredSvc := &corev1.Service{
			Spec: corev1.ServiceSpec{
				Type: corev1.ServiceTypeNodePort,
				Ports: []corev1.ServicePort{
					{
						Port:     3306,
						Protocol: corev1.ProtocolTCP,
					},
				},
			},
		}

		expectedSvc := &corev1.Service{
			Spec: corev1.ServiceSpec{
				Ports: []corev1.ServicePort{
					{
						Port:     3306,
						Protocol: corev1.ProtocolTCP,
						NodePort: 30000,
					},
				},
			},
		}

		updateServicePorts(existingSvc, desiredSvc)

		if !reflect.DeepEqual(existingSvc, expectedSvc) {
			t.Errorf("updateServicePorts() = %v, want %v", existingSvc, expectedSvc)
		}
	})

	t.Run("Desired service has ClusterIP type and existing port has non-zero NodePort", func(t *testing.T) {
		existingSvc := &corev1.Service{
			Spec: corev1.ServiceSpec{
				Ports: []corev1.ServicePort{
					{
						Port:     3306,
						Protocol: corev1.ProtocolTCP,
						NodePort: 30000,
					},
					{
						Port:     80,
						Protocol: corev1.ProtocolTCP,
						NodePort: 30001,
					},
				},
			},
		}

		desiredSvc := &corev1.Service{
			Spec: corev1.ServiceSpec{
				Type: corev1.ServiceTypeClusterIP,
				Ports: []corev1.ServicePort{
					{
						Port:     3306,
						Protocol: corev1.ProtocolTCP,
					},
				},
			},
		}

		expectedSvc := &corev1.Service{
			Spec: corev1.ServiceSpec{
				Ports: []corev1.ServicePort{
					{
						Port:     3306,
						Protocol: corev1.ProtocolTCP,
					},
					{
						Port:     80,
						Protocol: corev1.ProtocolTCP,
					},
				},
			},
		}

		updateServicePorts(existingSvc, desiredSvc)

		if !reflect.DeepEqual(existingSvc, expectedSvc) {
			t.Errorf("updateServicePorts() = %v, want %v", existingSvc, expectedSvc)
		}
	})

	t.Run("Desired service has LoadBalancer type and existing port has non-zero NodePort", func(t *testing.T) {
		existingSvc := &corev1.Service{
			Spec: corev1.ServiceSpec{
				Ports: []corev1.ServicePort{
					{
						Port:     3306,
						Protocol: corev1.ProtocolTCP,
						NodePort: 30000,
					},
					{
						Port:     80,
						Protocol: corev1.ProtocolTCP,
						NodePort: 30001,
					},
				},
			},
		}

		desiredSvc := &corev1.Service{
			Spec: corev1.ServiceSpec{
				Type: corev1.ServiceTypeLoadBalancer,
				Ports: []corev1.ServicePort{
					{
						Port:     3306,
						Protocol: corev1.ProtocolTCP,
					},
				},
			},
		}

		expectedSvc := &corev1.Service{
			Spec: corev1.ServiceSpec{
				Ports: []corev1.ServicePort{
					{
						Port:     3306,
						Protocol: corev1.ProtocolTCP,
						NodePort: 30000,
					},
					{
						Port:     80,
						Protocol: corev1.ProtocolTCP,
						NodePort: 30001,
					},
				},
			},
		}

		updateServicePorts(existingSvc, desiredSvc)

		if !reflect.DeepEqual(existingSvc, expectedSvc) {
			t.Errorf("updateServicePorts() = %v, want %v", existingSvc, expectedSvc)
		}
	})
}

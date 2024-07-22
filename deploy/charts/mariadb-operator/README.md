

[//]: # (README.md generated by gotmpl. DO NOT EDIT.)

<p align="center">
<img src="https://mariadb-operator.github.io/mariadb-operator/assets/mariadb-operator_centered_whitebg.svg" alt="mariadb" width="100%"/>
</p>

![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square) ![Version: 0.29.0](https://img.shields.io/badge/Version-0.29.0-informational?style=flat-square) ![AppVersion: v0.0.29](https://img.shields.io/badge/AppVersion-v0.0.29-informational?style=flat-square)

Run and operate MariaDB in a cloud native way

## Installing
```bash
helm repo add mariadb-operator https://helm.mariadb.com/mariadb-operator
helm install mariadb-operator mariadb-operator/mariadb-operator
```

## Uninstalling
```bash
helm uninstall mariadb-operator
```

## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| affinity | object | `{}` | Affinity to add to controller Pod |
| certController.affinity | object | `{}` | Affinity to add to controller Pod |
| certController.caValidity | string | `"35064h"` | CA certificate validity. It must be greater than certValidity. |
| certController.certValidity | string | `"8766h"` | Certificate validity. |
| certController.enabled | bool | `true` | Specifies whether the cert-controller should be created. |
| certController.extrArgs | list | `[]` | Extra arguments to be passed to the cert-controller entrypoint |
| certController.extraVolumeMounts | list | `[]` | Extra volumes to mount to cert-controller container |
| certController.extraVolumes | list | `[]` | Extra volumes to pass to cert-controller Pod |
| certController.ha.enabled | bool | `false` | Enable high availability |
| certController.ha.replicas | int | `3` | Number of replicas |
| certController.image.pullPolicy | string | `"IfNotPresent"` |  |
| certController.image.repository | string | `"docker-registry3.mariadb.com/mariadb-operator/mariadb-operator"` |  |
| certController.image.tag | string | `""` | Image tag to use. By default the chart appVersion is used |
| certController.imagePullSecrets | list | `[]` |  |
| certController.lookaheadValidity | string | `"2160h"` | Duration used to verify whether a certificate is valid or not. |
| certController.nodeSelector | object | `{}` | Node selectors to add to controller Pod |
| certController.podAnnotations | object | `{}` | Annotations to add to cert-controller Pod |
| certController.podSecurityContext | object | `{}` | Security context to add to cert-controller Pod |
| certController.requeueDuration | string | `"5m"` | Requeue duration to ensure that certificate gets renewed. |
| certController.resources | object | `{}` | Resources to add to cert-controller container |
| certController.securityContext | object | `{}` | Security context to add to cert-controller container |
| certController.serviceAccount.annotations | object | `{}` | Annotations to add to the service account |
| certController.serviceAccount.automount | bool | `true` | Automounts the service account token in all containers of the Pod |
| certController.serviceAccount.enabled | bool | `true` | Specifies whether a service account should be created |
| certController.serviceAccount.extraLabels | object | `{}` | Extra Labels to add to the service account |
| certController.serviceAccount.name | string | `""` | The name of the service account to use. If not set and enabled is true, a name is generated using the fullname template |
| certController.serviceMonitor.additionalLabels | object | `{}` | Labels to be added to the cert-controller ServiceMonitor |
| certController.serviceMonitor.enabled | bool | `true` | Enable cert-controller ServiceMonitor. Metrics must be enabled |
| certController.serviceMonitor.interval | string | `"30s"` | Interval to scrape metrics |
| certController.serviceMonitor.scrapeTimeout | string | `"25s"` | Timeout if metrics can't be retrieved in given time interval |
| certController.tolerations | list | `[]` | Tolerations to add to controller Pod |
| clusterName | string | `"cluster.local"` | Cluster DNS name |
| extrArgs | list | `[]` | Extra arguments to be passed to the controller entrypoint |
| extraEnv | list | `[]` | Extra environment variables to be passed to the controller |
| extraVolumeMounts | list | `[]` | Extra volumes to mount to the container. |
| extraVolumes | list | `[]` | Extra volumes to pass to pod. |
| fullnameOverride | string | `""` |  |
| ha.enabled | bool | `false` | Enable high availability |
| ha.replicas | int | `3` | Number of replicas |
| image.pullPolicy | string | `"IfNotPresent"` |  |
| image.repository | string | `"docker-registry3.mariadb.com/mariadb-operator/mariadb-operator"` |  |
| image.tag | string | `""` | Image tag to use. By default the chart appVersion is used |
| imagePullSecrets | list | `[]` |  |
| logLevel | string | `"INFO"` | Controller log level |
| metrics.enabled | bool | `false` | Enable operator internal metrics. Prometheus must be installed in the cluster |
| metrics.serviceMonitor.additionalLabels | object | `{}` | Labels to be added to the controller ServiceMonitor |
| metrics.serviceMonitor.enabled | bool | `true` | Enable controller ServiceMonitor |
| metrics.serviceMonitor.interval | string | `"30s"` | Interval to scrape metrics |
| metrics.serviceMonitor.scrapeTimeout | string | `"25s"` | Timeout if metrics can't be retrieved in given time interval |
| nameOverride | string | `""` |  |
| nodeSelector | object | `{}` | Node selectors to add to controller Pod |
| podAnnotations | object | `{}` | Annotations to add to controller Pod |
| podSecurityContext | object | `{}` | Security context to add to controller Pod |
| rbac.enabled | bool | `true` | Specifies whether RBAC resources should be created |
| resources | object | `{}` | Resources to add to controller container |
| securityContext | object | `{}` | Security context to add to controller container |
| serviceAccount.annotations | object | `{}` | Annotations to add to the service account |
| serviceAccount.automount | bool | `true` | Automounts the service account token in all containers of the Pod |
| serviceAccount.enabled | bool | `true` | Specifies whether a service account should be created |
| serviceAccount.extraLabels | object | `{}` | Extra Labels to add to the service account |
| serviceAccount.name | string | `""` | The name of the service account to use. If not set and enabled is true, a name is generated using the fullname template |
| tolerations | list | `[]` | Tolerations to add to controller Pod |
| webhook.affinity | object | `{}` | Affinity to add to controller Pod |
| webhook.annotations | object | `{}` | Annotations for webhook configurations. |
| webhook.cert.ca.key | string | `""` | File under 'ca.path' that contains the full CA trust chain. |
| webhook.cert.ca.path | string | `""` | Path that contains the full CA trust chain. |
| webhook.cert.certManager.duration | string | `""` | Duration to be used in the Certificate resource, |
| webhook.cert.certManager.enabled | bool | `false` | Whether to use cert-manager to issue and rotate the certificate. If set to false, mariadb-operator's cert-controller will be used instead. |
| webhook.cert.certManager.issuerRef | object | `{}` | Issuer reference to be used in the Certificate resource. If not provided, a self-signed issuer will be used. |
| webhook.cert.certManager.renewBefore | string | `""` | Renew before duration to be used in the Certificate resource. |
| webhook.cert.path | string | `"/tmp/k8s-webhook-server/serving-certs"` | Path where the certificate will be mounted. 'tls.crt' and 'tls.key' certificates files should be under this path. |
| webhook.cert.secretAnnotations | object | `{}` | Annotatioms to be added to webhook TLS secret. |
| webhook.cert.secretLabels | object | `{}` | Labels to be added to webhook TLS secret. |
| webhook.extrArgs | list | `[]` | Extra arguments to be passed to the webhook entrypoint |
| webhook.extraVolumeMounts | list | `[]` | Extra volumes to mount to webhook container |
| webhook.extraVolumes | list | `[]` | Extra volumes to pass to webhook Pod |
| webhook.ha.enabled | bool | `false` | Enable high availability |
| webhook.ha.replicas | int | `3` | Number of replicas |
| webhook.hostNetwork | bool | `false` | Expose the webhook server in the host network |
| webhook.image.pullPolicy | string | `"IfNotPresent"` |  |
| webhook.image.repository | string | `"docker-registry3.mariadb.com/mariadb-operator/mariadb-operator"` |  |
| webhook.image.tag | string | `""` | Image tag to use. By default the chart appVersion is used |
| webhook.imagePullSecrets | list | `[]` |  |
| webhook.nodeSelector | object | `{}` | Node selectors to add to controller Pod |
| webhook.podAnnotations | object | `{}` | Annotations to add to webhook Pod |
| webhook.podSecurityContext | object | `{}` | Security context to add to webhook Pod |
| webhook.port | int | `9443` | Port to be used by the webhook server |
| webhook.resources | object | `{}` | Resources to add to webhook container |
| webhook.securityContext | object | `{}` | Security context to add to webhook container |
| webhook.serviceAccount.annotations | object | `{}` | Annotations to add to the service account |
| webhook.serviceAccount.automount | bool | `true` | Automounts the service account token in all containers of the Pod |
| webhook.serviceAccount.enabled | bool | `true` | Specifies whether a service account should be created |
| webhook.serviceAccount.extraLabels | object | `{}` | Extra Labels to add to the service account |
| webhook.serviceAccount.name | string | `""` | The name of the service account to use. If not set and enabled is true, a name is generated using the fullname template |
| webhook.serviceMonitor.additionalLabels | object | `{}` | Labels to be added to the webhook ServiceMonitor |
| webhook.serviceMonitor.enabled | bool | `true` | Enable webhook ServiceMonitor. Metrics must be enabled |
| webhook.serviceMonitor.interval | string | `"30s"` | Interval to scrape metrics |
| webhook.serviceMonitor.scrapeTimeout | string | `"25s"` | Timeout if metrics can't be retrieved in given time interval |
| webhook.tolerations | list | `[]` | Tolerations to add to controller Pod |

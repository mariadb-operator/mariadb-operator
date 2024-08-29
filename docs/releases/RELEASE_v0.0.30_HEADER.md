We’re beyond excited to unveil `{{ .ProjectName }}`🦭 __[v0.0.30](https://github.com/mariadb-operator/mariadb-operator/releases/tag/v0.0.30)__!

This release includes significant updates that notably enhance the robustness and reliability of Galera.

It has also been the release with the most external contributions so far, and we couldn't be more thrilled! A massive thank you to all the contributors for your time, dedication, and incredible effort. Your high-quality contributions are the hearbeat of projects like this, fueling our success and driving us forward! 🙏🏻🦭

To upgrade from older versions, be sure to follow the __[UPGRADE GUIDE](https://github.com/mariadb-operator/mariadb-operator/blob/main/docs/UPGRADE_v0.0.30.md)__.

### Galera enhanced recovery

We have notably polished the recovery process fixing issues and covering cases that before resulted in the recovery getting stucked.

The sequence numbers required for the recovery are now obtained by `Jobs`, which mount the `MariaDB` PVCs and run `mariadbd --wsrep-recover`. This is a game changer in terms of reliability and recovery speed, as the `MariaDB` `Pods` no longer need to be restarted to fetch the sequence numbers. You can get more details about this new `Jobs` in the [documentation](https://github.com/mariadb-operator/mariadb-operator/blob/main/docs/GALERA.md#galera-recovery-job).

To ensure that you have full control of the recovery process in exceptional circumstances, we are also introducing the `forceClusterBootstrapInPod` field, which allows you to manually specify which `Pod` to use to bootstrap the new Galera cluster and bypass the operator recovery process. Read more about this field in the [documentation](https://github.com/mariadb-operator/mariadb-operator/blob/release-v0.0.30/docs/GALERA.md#force-cluster-bootstrap).

Finally, we have created this [troubleshooting section](https://github.com/mariadb-operator/mariadb-operator/blob/release-v0.0.30/docs/GALERA.md#galera-cluster-recovery-not-progressing) to help you triaging and operating the Galera recovery process.

Read more about this in the [Galera documentation](https://github.com/mariadb-operator/mariadb-operator/blob/main/docs/GALERA.md).

### Bootstrap Galera cluster from existing PVCs

`mariadb-operator` will never delete your `MariaDB` PVCs. Whenever you delete a `MariaDB` resource, the PVCs will remain intact so you could reuse them to re-provision a new cluster.

That said, the operator is able now to bootstrap a new `MariaDB` Galera cluster when previous PVCs still exist. During the provisioning phase, the operator automatically triggers the Galera recovery process if pre-existing PVCs are detected.

This is a massive improvement in terms of usability, as you don't need to care about whether you have previous PVCs or not and enables seamless Galera cluster recreation without data loss.

Read more about this in the [Galera documentation](https://github.com/mariadb-operator/mariadb-operator/blob/main/docs/GALERA.md).

### Suspend

By leveraging the automation provided this operator, you can declaratively manage large fleets of databases using CRs. This also covers day two operations, such as upgrades, which can be risky when rolling out updates to thousands of instances simultaneously.

To mitigate this, and to give you more control on when these operations are performed, you are able to selectively suspend a subset of `MariaDB` resources, temporarily stopping the upgrades and other operations on them.

This has multiple applications, including:
- Progressive fleet upgrades
- Operator upgrades
- Maintenance

Read more about this new feature in the [Suspend documentation](https://github.com/mariadb-operator/mariadb-operator/blob/main/docs/SUSPEND.md).

Kudos to @harunkucuk5 for this contribution! 🙏🏻

### SQL resource cleanup policy

This operator enables you to manage SQL resources declaratively through CRs. By SQL resources, we refer to users, grants, and databases that are typically created using SQL statements. The key advantage of this approach is that, unlike executing SQL statements, which is a one-time operatopm, declaring a SQL resource via a CR ensures that the resource is periodically reconciled by the operator.

We have introduced a new `cleanupPolicy` field in our SQL CRs (`User`, `Grant`, `Database`) which allows you to control the termination logic of these resources. Whenever they are deleted, the operator takes into account this new field and determines whether the database resources should be deleted (`cleanupPolicy=Delete`) or not (`cleanupPolicy=Skip`).

Read more about this new feature in the [SQL resources documentation](https://github.com/mariadb-operator/mariadb-operator/blob/main/docs/SQL_RESOURCES.md).

Kudos to @ChrisHubinger for this contribution! 🙏🏻

### Authentication plugins

User passwords can be supplied using the `passwordSecretKeyRef` field in the `User` CR. This is a reference to a `Secret` that contains password in plain text. 

Alternatively, you can now use [MariaDB authentication plugins](https://mariadb.com/kb/en/authentication-plugins/) to avoid passing passwords in plain text and provide the password in a hashed format instead. This doesn't affect the end user experience, as they will still need to provide the password in plain text to authenticate.

Read more about this new feature in the [SQL resources documentation](https://github.com/mariadb-operator/mariadb-operator/blob/main/docs/SQL_RESOURCES.md).

Kudos to @NHellFire for this contribution! 🙏🏻

### Timezones

By default, MariaDB does not load timezone data on startup for performance reasons and defaults the timezone to `SYSTEM`, obtaining the timezone information from the environment where it runs. Starting from this version, you can explicitly configure a timezone in your `MariaDB` instance by setting the `timeZone` field.

`Backup` and `SqlJob` resources, which get reconciled into `CronJobs`, an also define a `timeZone` associated with their cron expression.

Read more about this new feature [here](https://github.com/mariadb-operator/mariadb-operator/blob/main/docs/CONFIGURATION.md#timezones).

Kudos to @mbezhanov for this contribution! 🙏🏻

### Job history limit

`Backup`, `Restore` and `SqlJob` resources support now history limits via the new `successfulJobsHistoryLimit` and `failedJobsHistoryLimit` fields.

Kudos to @mbezhanov for this contribution! 🙏🏻

### Initial CEL support

We have introduced new declarative validations leveraging the [Common Expression Language](https://kubernetes.io/docs/reference/using-api/cel/). The idea is progressively migrate the webhook validations to CEL in further releases. 

Some initial validations have already been merged (see #783), but there's still more work to be done. Check out #788 for the details. Contributions are welcome!

With the introduction of CEL in Kubernetes 1.26, this version now becomes our minimum supported version. Please ensure you update to Kubernetes 1.26 or later to stay compatible.

Kudos to @businessbean for this initiative! 🙏🏻

## Migrate your MariaDB instance to Kubernetes

This [migration guide](https://github.com/mariadb-operator/mariadb-operator/blob/main/docs/BACKUP.md#migrating-an-external-mariadb-to-a-mariadb-running-in-kubernetes) will streamline your onboarding process and assist you in migrating your data into a `MariaDB` instance running on Kubernetes.

### Helm chart improvements

- Support for `view` and `edit` aggregated `ClusterRoles`.
- New `extraEnvFrom` value: Inject environment variables to the controller via `ConfigMaps` or `Secrets`.
- Added `revisionHistoryLimit` to the webhook `Certificate` to avoid getting flooded with `CertificateRequests`.


Kudos do @gprossliner @kettil for the contributions! 🙏🏻

---

We value your feedback! If you encounter any issues or have suggestions, please [open an issue on GitHub](https://github.com/mariadb-operator/mariadb-operator/issues/new/choose). Your input is crucial to improve `{{ .ProjectName }}`🦭.

Join us on Slack: **[MariaDB Community Slack](https://r.mariadb.com/join-community-slack)**.
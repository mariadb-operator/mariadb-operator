We’re beyond excited to unveil `{{ .ProjectName }}`🦭 __[v0.0.30](https://github.com/mariadb-operator/mariadb-operator/releases/tag/v0.0.30)__!

This release includes significant updates that notably enhance the robustness and reliability of Galera.

It has also been the release with the most external contributions so far, and we couldn't be more thrilled! A massive thank you to all the contributors for your time, dedication, and incredible effort. Your high-quality contributions are the hearbeat of projects like this, fueling our success and driving us forward! 🙏🏻🦭

To upgrade from older versions, be sure to follow the __[UPGRADE GUIDE](https://github.com/mariadb-operator/mariadb-operator/blob/main/docs/UPGRADE_v0.0.30.md)__.

### Galera enhanced recovery

We've significantly refined the Galera recovery process, addressing issues and covering scenarios that previously caused the recovery to get stuck.

The sequence numbers needed for recovery are now retrieved by `Jobs` that mount the `MariaDB` PVCs and execute `mariadbd --wsrep-recover`. This is a game changer for reliability and recovery speed, as it eliminates the need to restart `MariaDB` `Pods` just to fetch sequence numbers. For more details on these new `Jobs`, check out the [documentation](https://github.com/mariadb-operator/mariadb-operator/blob/main/docs/GALERA.md#galera-recovery-job).

To give you full control over the recovery process in exceptional situations, we are introducing the `forceClusterBootstrapInPod` field. This allows you to manually select which `Pod` to use for bootstrapping a new Galera cluster, bypassing the operator’s recovery process. For more details on how to use this feature, check out the [documentation](https://github.com/mariadb-operator/mariadb-operator/blob/main/docs/GALERA.md#force-cluster-bootstrap).

Finally, we've added a [troubleshooting section](https://github.com/mariadb-operator/mariadb-operator/blob/main/docs/GALERA.md#galera-cluster-recovery-not-progressing) to assist you in diagnosing and managing the Galera recovery process.

Read more about this in the [Galera documentation](https://github.com/mariadb-operator/mariadb-operator/blob/main/docs/GALERA.md).

### Bootstrap Galera cluster from existing PVCs

`{{ .ProjectName }}` will never delete your `MariaDB` PVCs. Whenever you delete a `MariaDB` resource, the PVCs will remain intact so you could reuse them to re-provision a new cluster.

That said, the operator is able now to bootstrap a new `MariaDB` Galera cluster from previous PVCs. During the provisioning phase, the operator automatically triggers the Galera recovery process if pre-existing PVCs are detected. This is a massive improvement in terms of usability, as it enables seamless Galera cluster recreation without data loss.

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

This operator enables you to manage SQL resources declaratively through CRs. By SQL resources, we refer to users, grants, and databases that are typically created using SQL statements. The key advantage of this approach is that, unlike executing SQL statements, which is a one-time operation, declaring a SQL resource via a CR ensures that the resource is periodically reconciled by the operator.

We have introduced a new `cleanupPolicy` field to our SQL CRs (`User`, `Grant`, `Database`), giving you control over the termination logic for these resources. When a resource is deleted, the operator evaluates this field to decide whether the corresponding database resources should be deleted (`cleanupPolicy=Delete`) or preserved (`cleanupPolicy=Skip`).

Read more about this new feature in the [SQL resources documentation](https://github.com/mariadb-operator/mariadb-operator/blob/main/docs/SQL_RESOURCES.md).

Kudos to @ChrisHubinger for this contribution! 🙏🏻

### Authentication plugins

User passwords can be provided using the `passwordSecretKeyRef` field in the `User` CR, which references a `Secret` containing the password in plain text.

Alternatively, you can now use [MariaDB authentication plugins](https://mariadb.com/kb/en/authentication-plugins/) to supply passwords in a hashed format instead of plain text. This approach doesn't change the user experience, as users will still need to provide their passwords in plain text to authenticate.

Read more about this new feature in the [SQL resources documentation](https://github.com/mariadb-operator/mariadb-operator/blob/main/docs/SQL_RESOURCES.md).

Kudos to @NHellFire for this contribution! 🙏🏻

### Timezones

By default, MariaDB does not load timezone data on startup for performance reasons and defaults the timezone to `SYSTEM`, obtaining the timezone information from the environment where it runs. However, starting with this version, you can explicitly set a timezone for your `MariaDB` instance by configuring the `timeZone` field.

Additionally, `Backup` and `SqlJob` resources, which are reconciled into `CronJobs`, can also specify a `timeZone` for their cron expressions.

Read more about this new feature [here](https://github.com/mariadb-operator/mariadb-operator/blob/main/docs/CONFIGURATION.md#timezones).

Kudos to @mbezhanov for this contribution! 🙏🏻

### Job history limit

`Backup`, `Restore`, and `SqlJob` resources now support history limits through the newly introduced `successfulJobsHistoryLimit` and `failedJobsHistoryLimit` fields.

Kudos to @mbezhanov for this contribution! 🙏🏻

### Initial CEL support

We have introduced new declarative validations leveraging the [Common Expression Language](https://kubernetes.io/docs/reference/using-api/cel/). The idea is progressively migrating the webhook validations to CEL in further releases. 

Some initial validations have already been merged (see https://github.com/mariadb-operator/mariadb-operator/pull/783), but there's still more work to be done. Check out https://github.com/mariadb-operator/mariadb-operator/pull/788 for the details. Contributions are welcome.

With the [introduction of CEL in Kubernetes 1.26](https://kubernetes.io/blog/2022/12/20/validating-admission-policies-alpha/), this version now becomes our minimum supported version. Please ensure you update to Kubernetes 1.26 or later to stay compatible.

Kudos to @businessbean for this initiative! 🙏🏻

### Helm chart improvements

- Support for `view` and `edit` aggregated `ClusterRoles`.
- New `extraEnvFrom` value: Inject environment variables to the controller via `ConfigMaps` or `Secrets`.
- Added `revisionHistoryLimit` to the webhook `Certificate` to prevent being overwhelmed by `CertificateRequests`.
- Fix webhook `Certificate` rendering regressions in the context of `secretTemplate`. 
- Introduce helm chart unit testing to prevent regressions like the previous one. Contributions are welcome here, see https://github.com/mariadb-operator/mariadb-operator/issues/767.

Kudos do @gprossliner @kettil for the contributions! 🙏🏻

### Migrate your MariaDB instance to Kubernetes

This [migration guide](https://github.com/mariadb-operator/mariadb-operator/blob/main/docs/BACKUP.md#migrating-an-external-mariadb-to-a-mariadb-running-in-kubernetes) will streamline your onboarding process and assist you in migrating your data into a `MariaDB` instance running on Kubernetes.

---

We value your feedback! If you encounter any issues or have suggestions, please [open an issue on GitHub](https://github.com/mariadb-operator/mariadb-operator/issues/new/choose). Your input is crucial to improve `{{ .ProjectName }}`🦭.

Join us on Slack: **[MariaDB Community Slack](https://r.mariadb.com/join-community-slack)**.
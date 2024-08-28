We’re beyond excited to unveil `{{ .ProjectName }}`🦭 __[v0.0.30](https://github.com/mariadb-operator/mariadb-operator/releases/tag/v0.0.30)__!

This release includes significant updates that notably enhance the robustness and reliability of Galera.

It has also been the release with the most external contributions so far, and we couldn't be more thrilled! A massive thank you to all the contributors for your time, dedication, and incredible effort. Your high-quality contributions are the hearbeat of projects like this, fueling our success and driving us forward! 🙏🏻🦭🎉

To upgrade from older versions, be sure to follow the __[UPGRADE GUIDE](https://github.com/mariadb-operator/mariadb-operator/blob/main/docs/UPGRADE_v0.0.30.md)__.

## Galera enhanced recovery

We have notably polished the recovery process fixing issues and covering cases that before resulted in the recovery getting stucked.

The sequence numbers required for the recovery are now obtained by `Jobs`, which mount the `MariaDB` PVCs and run `mariadbd --wsrep-recover`. This is a game changer in terms of reliability and recovery speed, as the `MariaDB` `Pods` don't need to be restarted in order to achieve this. You can get more details about this new `Jobs` in the [documentation](https://github.com/mariadb-operator/mariadb-operator/blob/main/docs/GALERA.md#galera-recovery-job).

To ensure that you have full control of the recovery process in exceptional circumstances, we are also introducing the `forceClusterBootstrapInPod` field, which allows you to manually specify which `Pod` to use to bootstrap the new Galera cluster and bypass the operator recovery process. Read more about this field in the [documentation](https://github.com/mariadb-operator/mariadb-operator/blob/release-v0.0.30/docs/GALERA.md#force-cluster-bootstrap).

Finally, we have created this [troubleshooting section](https://github.com/mariadb-operator/mariadb-operator/blob/release-v0.0.30/docs/GALERA.md#galera-cluster-recovery-not-progressing) to help you triaging and operating the Galera recovery process.

Read more about this in the [Galera documentation](https://github.com/mariadb-operator/mariadb-operator/blob/main/docs/GALERA.md).

## Bootstrap Galera cluster from existing PVCs

`mariadb-operator` will never delete your `MariaDB` PVCs! Whenever you delete a `MariaDB` resource, the PVCs will remain intact so you could reuse them to re-provision a new cluster.

That said, the operator is able now to bootstrap a new `MariaDB` Galera cluster when previous PVCs still exist. During the provisioning phase, the operator automatically triggers the Galera recovery process if pre-existing PVCs are detected.

This is a massive improvement in terms of usability, as you don't need to care about whether you have previous PVCs or not and enables seamless Galera cluster recreation without data loss.

Read more about this in the [Galera documentation](https://github.com/mariadb-operator/mariadb-operator/blob/main/docs/GALERA.md).

## Suspend

By leveraging the automation provided this operator, you can declaratively manage large fleets of databases using CRs. This also covers day two operations, such as upgrades, which can be risky when rolling out updates to thousands of instances simultaneously.

To mitigate this, and to give you more control on when these operations are performed, you are able to selectively suspend a subset of `MariaDB` resources, temporarily stopping the upgrades and other operations on them.

This has multiple applications, including:
- Progressive fleet upgrades
- Operator upgrades
- Maintenance

Read more about this new feature in the [Suspend documentation](https://github.com/mariadb-operator/mariadb-operator/blob/main/docs/SUSPEND.md).

Kudos to @harunkucuk5 for this contribution! 🙏🏻

## SQL resource cleanup policy

This operator enables you to manage SQL resources declaratively through CRs. By SQL resources, we refer to users, grants, and databases that are typically created using SQL statements. The key advantage of this approach is that, unlike executing SQL statements, which is a one-time action, declaring a SQL resource via a CR ensures that the resource is periodically reconciled by the operator.

We have introduced a new `cleanupPolicy` field in our SQL CRs (`User`, `Grant`, `Database`) which allows you to control the termination logic of these resources. Whenever they are deleted, the operator takes into account this new field and determines whether the database resources should be deleted (`cleanupPolicy=Delete`) or not (`cleanupPolicy=Skip`).

Read more about this new feature in the [SQL resources documentation](https://github.com/mariadb-operator/mariadb-operator/blob/main/docs/SQL_RESOURCES.md).

Kudos to @ChrisHubinger for this contribution! 🙏🏻

## Migrate your MariaDB instance to Kubernetes

This [migration guide](https://github.com/mariadb-operator/mariadb-operator/blob/main/docs/BACKUP.md#migrating-an-external-mariadb-to-a-mariadb-running-in-kubernetes) will streamline your onboarding process and assist you in migrating your data into a `MariaDB` instance running on Kubernetes.

---

We value your feedback! If you encounter any issues or have suggestions, please [open an issue on GitHub](https://github.com/mariadb-operator/mariadb-operator/issues/new/choose). Your input is crucial to improve `{{ .ProjectName }}`🦭.

Join us on Slack: **[MariaDB Community Slack](https://r.mariadb.com/join-community-slack)**.
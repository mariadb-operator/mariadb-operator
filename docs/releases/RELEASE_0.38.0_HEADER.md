**`{{ .ProjectName }}` [0.38.0](https://github.com/mariadb-operator/mariadb-operator/releases/tag/0.38.0) is here!** 🦭

We're thrilled to announce this new release packed with multiple enhancements contributed by our community members. A community-driven release like this one is the best way to celebrate that we have now __600+ stars and 60+ contributors! 🎉__ 

If you're upgrading from previous versions, don't miss the __[UPGRADE GUIDE](https://github.com/mariadb-operator/mariadb-operator/blob/main/docs/releases/UPGRADE_0.38.0.md)__ for a smooth transition.

In this version, we started to prepare the ground to make our asynchronous replication feature more robust and ready for production in the following releases. For doing so, @hedgieinsocks did a great investigation and contribution in https://github.com/mariadb-operator/mariadb-operator/pull/1219. Read through the PR and the issue for further detail, and make sure you follow the migration guide above if you are currenly using asynchronous replication.

There are other notable changes in this release:

- Avoid reconciling `MariaDB` if it's marked for deletion (https://github.com/mariadb-operator/mariadb-operator/pull/1146 by @lsoica)
- Support for `command` and `args` on `MaxScale` resource (https://github.com/mariadb-operator/mariadb-operator/pull/1160 by @wfelipew)
- Preserve randomly generated `Secrets` (https://github.com/mariadb-operator/mariadb-operator/pull/1168 by @AlPepino)
- Support for `HostPath` in `VolumeSource` (https://github.com/mariadb-operator/mariadb-operator/pull/1192 by @kevinvalk)
- Add `Args` to `Exporter` (https://github.com/mariadb-operator/mariadb-operator/pull/1217 by @hedgieinsocks)
- Remove `Secret` webhook template in helm chart (see https://github.com/mariadb-operator/mariadb-operator/pull/1203)
- MariaDB 11.4.5 is now the default version
- Kubernetes 1.32 and controller-runtime 0.32.3 support
- Go 1.24 support

Huge thanks to our awesome contributors @hedgieinsocks, @lsoica, @wfelipew, @AlPepino and @kevinvalk! 🙇

---

We value your feedback! If you encounter any issues or have suggestions, please [open an issue on GitHub](https://github.com/mariadb-operator/mariadb-operator/issues/new/choose). Your input is crucial to improve `{{ .ProjectName }}`🦭.

Join us on Slack: **[MariaDB Community Slack](https://r.mariadb.com/join-community-slack)**.
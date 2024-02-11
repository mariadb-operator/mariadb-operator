---
name: Bug
about: Create a bug report to help us improve
title: "[Bug] "
labels: bug
assignees: 

---

<!--
Bugs should be filed for issues encountered whilst operating mariadb-operator.
Please provide as much detail as possible. 
-->

**Documentation**
- [ ] I acknowledge that I have read the relevant [documentation](https://github.com/mariadb-operator/mariadb-operator/tree/main/docs).

**Describe the bug**
<!--
A clear and concise description of what the bug is. 
Tip: you can use 
```
<code here>
```
for code blocks of your kubectl output or YAML files.
-->

**Expected behaviour**
<!--A concise description of what you expected to happen.-->

**Steps to reproduce the bug**
<!--Steps to reproduce the bug should be clear and easily reproducible to help people
gain an understanding of the problem.-->

1. ...
2. ...
3. ...

**Debug information**
- Related object events:
```bash
kubectl get events --field-selector involvedObject.name=<mariadb-resource-name>
kubectl get events --field-selector involvedObject.name=<backup-resource-name>
kubectl get events --field-selector involvedObject.name=<restore-resource-name>
```
- `mariadb-operator` logs. Set the `--log-level` to `debug` if needed.

**Environment details**:
- Kubernetes version: [Version number]
- Kubernetes distribution: [Vanilla, EKS, GKE, AKS, Rancher, OpenShift, k3s, KIND...]
- mariadb-operator version: [Version number]
- Install method: [helm, OLM, or static manifests]
- Install flavor: [minimal, recommended, or custom]

**Additional context**
<!--Add any other context  here.-->

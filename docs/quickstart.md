## Quickstart

Let's see `mariadb-operator`🦭 in action! First of all, install the following configuration manifests that will be referenced by the CRDs further:
```bash
kubectl apply -f examples/manifests/config
```

Next, you can proceed with the installation of a `MariaDB` instance:
```bash
kubectl apply -f examples/manifests/mariadb.yaml
```
```bash
kubectl get mariadbs
NAME      READY   STATUS    PRIMARY POD     AGE
mariadb   True    Running   mariadb-0       3m57s

kubectl get statefulsets
NAME      READY   AGE
mariadb   1/1     2m12s

kubectl get services
NAME         TYPE        CLUSTER-IP      EXTERNAL-IP   PORT(S)             AGE
mariadb      ClusterIP   10.96.235.145   <none>        3306/TCP,9104/TCP   2m17s
```
Up and running 🚀, we can now create our first logical database and grant access to users:
```bash
kubectl apply -f examples/manifests/database.yaml
kubectl apply -f examples/manifests/user.yaml
kubectl apply -f examples/manifests/grant.yaml
```
```bash
kubectl get databases
NAME        READY   STATUS    CHARSET   COLLATE           AGE
data-test   True    Created   utf8      utf8_general_ci   22s

kubectl get users
NAME              READY   STATUS    MAXCONNS   AGE
user              True    Created   20         29s

kubectl get grants
NAME              READY   STATUS    DATABASE   TABLE   USERNAME          GRANTOPT   AGE
user              True    Created   *          *       user              true       36s
```
At this point, we can run our database initialization scripts:
```bash
kubectl apply -f examples/manifests/sqljobs
```
```bash
kubectl get sqljobs
NAME       COMPLETE   STATUS    MARIADB   AGE
01-users   True       Success   mariadb   2m47s
02-repos   True       Success   mariadb   2m47s
03-stars   True       Success   mariadb   2m47s

kubectl get jobs
NAME                  COMPLETIONS   DURATION   AGE
01-users              1/1           10s        3m23s
02-repos              1/1           11s        3m13s
03-stars-28067562     1/1           10s        106s

kubectl get cronjobs
NAME       SCHEDULE      SUSPEND   ACTIVE   LAST SCHEDULE   AGE
03-stars   */1 * * * *   False     0        57s             2m33s
```

Now that the database has been initialized, let's take a backup:
```bash
kubectl apply -f examples/manifests/backup.yaml
``` 
```bash
kubectl get backups
NAME               COMPLETE   STATUS    MARIADB   AGE
backup             True       Success   mariadb   15m

kubectl get jobs
NAME               COMPLETIONS   DURATION   AGE
backup-27782894    1/1           4s         3m2s
```
Last but not least, let's provision a second `MariaDB` instance bootstrapping from the previous backup:
```bash
kubectl apply -f examples/manifests/mariadb_from_backup.yaml
``` 
```bash
kubectl get mariadbs
NAME                  READY   STATUS    PRIMARY POD             AGE
mariadb               True    Running   mariadb-0               7m47s
mariadb-from-backup   True    Running   mariadb-from-backup-0   53s

kubectl get restores
NAME                                         COMPLETE   STATUS    MARIADB               AGE
bootstrap-restore-mariadb-from-backup        True       Success   mariadb-from-backup   72s

kubectl get jobs
NAME                                         COMPLETIONS   DURATION   AGE
backup                                       1/1           9s         12m
bootstrap-restore-mariadb-from-backup        1/1           5s         84s
``` 

# Docker Images for MariaDB Operator

> [!WARNING]
> The Docker registries at `docker-registry*.mariadb.com` are __deprecated__ and will be removed in a future release. All images have been migrated to new registries as shown in the table below.

`mariadb-operator` defaults to the following Docker images when no `image` field is specifield in the `MariaDB` and `MaxScale` CRs:

<table width="100%">
  <thead>
    <tr>
      <th width="20%">Component</th>
      <th width="60%">Image</th>
      <th width="20%">Architecture</th>
    </tr>
  </thead>
  <tbody>
    <tr>
      <td>MariaDB Community Server</td>
      <td><code>mariadb:11.8.8</code></td>
      <td><code>amd64</code> <code>arm64</code></td>
    </tr>
	  <tr>
      <td>MaxScale</td>
      <td><code>mariadb/maxscale:23.08.5</code></td>
      <td><code>amd64</code> <code>arm64</code></td>
    </tr>
    <tr>
      <td>Mysqld Exporter</td>
	    <td><code>prom/mysqld-exporter:v0.15.1</code></td>
      <td><code>amd64</code> <code>arm64</code></td>
    </tr>
    <tr>
      <td>MaxScale Prometheus exporter</td>
	    <td><code>mariadb/maxscale-prometheus-exporter-ubi:v0.0.1</code></td>
      <td><code>amd64</code> <code>arm64</code></td>
    </tr>
    <tr>
      <td>MariaDB Operator</td>
	    <td><code>ghcr.io/mariadb-operator/mariadb-operator:26.6.0</code></td>
      <td><code>amd64</code> <code>arm64</code></td>
    </tr>
  </tbody>
</table>

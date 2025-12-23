# Docker Images for MariaDB Operator

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
      <td><code>docker-registry1.mariadb.com/library/mariadb:11.8.2</code></td>
      <td><code>amd64</code> <code>arm64</code></td>
    </tr>
	  <tr>
      <td>MaxScale</td>
      <td><code>docker-registry2.mariadb.com/mariadb/maxscale:23.08.5</code></td>
      <td><code>amd64</code> <code>arm64</code></td>
    </tr>
    <tr>
      <td>Mysqld Exporter</td>
	    <td><code>prom/mysqld-exporter:v0.15.1</code></td>
      <td><code>amd64</code> <code>arm64</code></td>
    </tr>
    <tr>
      <td>MaxScale Prometheus exporter</td>
	    <td><code>docker-registry2.mariadb.com/mariadb/maxscale-prometheus-exporter-ubi:v0.0.1</code></td>
      <td><code>amd64</code> <code>arm64</code></td>
    </tr>
    <tr>
      <td>MariaDB Operator</td>
	    <td><code>docker-registry3.mariadb.com/mariadb-operator/mariadb-operator:25.10.3-dev</code></td>
      <td><code>amd64</code> <code>arm64</code></td>
    </tr>
  </tbody>
</table>

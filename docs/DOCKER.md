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
      <td><code>docker-registry1.mariadb.com/library/mariadb:11.4.4</code></td>
      <td><code>amd64</code> <code>arm64</code></td>
    </tr>
	  <tr>
      <td>MariaDB MaxScale</td>
      <td><code>docker-registry2.mariadb.com/mariadb/maxscale:23.08.5</code></td>
      <td><code>amd64</code> <code>arm64</code></td>
    </tr>
    <tr>
      <td>MariaDB Prometheus Exporter</td>
	    <td><code>docker-registry2.mariadb.com/mariadb/mariadb-prometheus-exporter-ubi:v0.0.1</code></td>
      <td><code>amd64</code> <code>arm64</code></td>
    </tr>
    <tr>
      <td>MariaDB MaxScale prometheus exporter</td>
	    <td><code>docker-registry2.mariadb.com/mariadb/maxscale-prometheus-exporter-ubi:v0.0.1</code></td>
      <td><code>amd64</code> <code>arm64</code></td>
    </tr>
    <tr>
      <td>MariaDB Operator</td>
	    <td><code>docker-registry3.mariadb.com/mariadb-operator/mariadb-operator:0.37.1</code></td>
      <td><code>amd64</code> <code>arm64</code></td>
    </tr>
  </tbody>
</table>

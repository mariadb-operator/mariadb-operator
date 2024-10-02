# Docker Images for MariaDB Operator

The mariadb-operator uses a variety of docker images depending on how it’s configured and what mix of open source or commercial software you’d like to use. As only the latest version of MariaDB Community Server is supported, the community server version will increment frequently with only a best effort made to keep current with the latest release(s). Only MariaDB Enterprise Server offers support for older versions.

> [!NOTE]  
> Using Docker images other than the supported ones in this document is not recommended at this time.

<table width="100%">
  <thead>
    <tr>
      <th width="15%">Component</th>
      <th width="20%">Docker Registry</th>
      <th width="20%">Supported Tags</th>
      <th width="5%">CPU</th>
      <th width="40%">Pull Command</th>
    </tr>
  </thead>
  <tbody>
    <tr>
      <td>MariaDB Community Server</td>
      <td>Public</td>
      <td><code>11.4.3</code> (Used with Community Operator)<br/><code>11.4.3-ubi9</code> (Used with Enterprise Operator)</td>
      <td><code>amd64</code> <code>arm64</code> <code>ppc64le</code> <code>s390x</code></td>
	  <td><code>docker pull docker-registry1.mariadb.com/library/mariadb:11.4.3</code><br/><code>docker pull docker-registry1.mariadb.com/library/mariadb:11.4.3-ubi9</code></td>
    </tr>
    <tr>
      <td>MariaDB Enterprise Server</td>
      <td>Private<br><a href=https://docker.mariadb.com>docker.mariadb.com</a><br>Login required, click link for instructions</td>
      <td><code>10.6.18-14.1</code> <code>10.6.17-13.1</code> <code>10.5.25-19.1</code> <code>10.5.24-18.1</code></td>
      <td><code>amd64</code> <code>arm64</code></td>
	  <td><code>docker pull docker-registry.mariadb.com/enterprise-server:10.6.18-14</code></td>
    </tr>
        <tr>
      <td>MariaDB MaxScale UBI</td>
       <td>Public</td>
      <td><code>23.08.6-ubi-1</code> <code>24.02.2-ubi-1</code></td>
      <td><code>amd64</code> <code>arm64</code></td>
	  <td><code>docker pull docker-registry2.mariadb.com/mariadb/maxscale:23.08.6-ubi-1</code></td>
    </tr> 
	          <tr>
      <td>MariaDB MaxScale Rocky Linux</td>
          <td>Public</td>
      <td><code>mariadb/maxscale:23.08.5</code></td>
      <td><code>amd64</code> <code>arm64</code></td>
	  <td><code>docker pull docker-registry2.mariadb.com/mariadb/maxscale:23.08.5</code></td>
    </tr>
         <tr>
      <td>MariaDB Prometheus Exporter</td>
       <td>Public</td>
      <td><code>v0.0.1</code></td>
      <td><code>amd64</code> <code>arm64</code></td>
	  <td><code>docker pull docker-registry2.mariadb.com/mariadb/mariadb-prometheus-exporter-ubi:v0.0.1</code></td>
    </tr>
        <tr>
      <td>MariaDB MaxScale prometheus exporter</td>
       <td>Public</td>
      <td><code>v0.0.1</code></td>
      <td><code>amd64</code> <code>arm64</code></td>
	  <td><code>docker pull docker-registry2.mariadb.com/mariadb/maxscale-prometheus-exporter-ubi:v0.0.1</code></td>
    </tr>
        <tr>
      <td>Community Operator</td>
       <td>Public</td>
      <td><code>v0.0.33</code></td>
      <td><code>amd64</code> <code>arm64</code></td>
	  <td><code>docker pull docker-registry3.mariadb.com/mariadb-operator/mariadb-operator:v0.0.30</code></td>
    </tr>
         <tr>
      <td>Enterprise Operator</td>
       <td>Public</td>
      <td><code>v0.0.33</code></td>
      <td><code>amd64</code> <code>arm64</code></td>
	  <td><code>docker pull docker-registry2.mariadb.com/mariadb/mariadb-operator-enterprise:v0.0.30</code></td>
    </tr>
  </tbody>
</table>

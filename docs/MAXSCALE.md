# MaxScale

> [!WARNING]  
> This documentation applies to `mariadb-operator` version >= v0.0.25

MaxScale is a sophisticated database proxy, router, and load balancer designed specifically for MariaDB. It provides a range of features that ensure optimal high availability:
- Query based routing: Transparently route write queries to the primary nodes and read queries to the replica nodes.
- Connection based routing: Load balance connection between multiple servers.
- Automatic primary failover based on MariaDB internals.
- Replay pending transactions when a server goes down.

To better understand what MaxScale is capable of you may check the [product page](https://mariadb.com/docs/server/products/mariadb-maxscale/) and the [documentation](https://mariadb.com/kb/en/maxscale/). 

## MaxScale resources

#### Servers

https://mariadb.com/kb/en/mariadb-maxscale-2308-mariadb-maxscale-configuration-guide/#server_1

#### Monitors

https://mariadb.com/kb/en/mariadb-maxscale-2308-common-monitor-parameters/
https://mariadb.com/kb/en/mariadb-maxscale-2308-galera-monitor/#galera-monitor-optional-parameters
https://mariadb.com/kb/en/mariadb-maxscale-2308-mariadb-monitor/#configuration

#### Services

https://mariadb.com/kb/en/mariadb-maxscale-2308-mariadb-maxscale-configuration-guide/#service_1
https://mariadb.com/kb/en/mariadb-maxscale-2308-readwritesplit/#configuration
https://mariadb.com/kb/en/mariadb-maxscale-2308-readconnroute/#configuration

#### Listeners

https://mariadb.com/kb/en/mariadb-maxscale-2308-mariadb-maxscale-configuration-guide/#listener_1

## `MaxScale` CR

## `MariaDB` CR

## Configuration

https://mariadb.com/kb/en/mariadb-maxscale-2308-mariadb-maxscale-configuration-guide/

## Defaults

## Authentication

## Connection

## High availability

## Maintenance

## External MariaDBs

## Status

## MaxScale GUI


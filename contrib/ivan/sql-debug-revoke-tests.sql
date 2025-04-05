-- ✅ MariaDB GRANT/REVOKE test results by @ivanpiatnishin

-- Setup
CREATE DATABASE nova_cell0;
CREATE USER 'nova'@'%' IDENTIFIED BY 'password';
GRANT ALL PRIVILEGES ON nova_cell0.* TO 'nova'@'%' WITH GRANT OPTION;

-- Revoke valid
REVOKE ALL PRIVILEGES, GRANT OPTION FROM 'nova'@'%';
REVOKE GRANT OPTION ON *.* FROM 'nova'@'%';

-- Revoke invalid
REVOKE GRANT OPTION ON nova_cell0.* FROM 'nova'@'%'; -- fails
REVOKE ALL PRIVILEGES, GRANT OPTION ON nova_cell0.* FROM 'nova'@'%'; -- fails

-- Role test
CREATE ROLE journalist;
GRANT SELECT ON nova_cell0.* TO journalist;
GRANT journalist TO 'nova'@'%' WITH ADMIN OPTION;
REVOKE journalist FROM 'nova'@'%';
REVOKE ADMIN OPTION FOR journalist FROM 'nova'@'%';

-- Cleanup
DROP USER 'nova'@'%';
DROP ROLE journalist;
DROP DATABASE nova_cell0;


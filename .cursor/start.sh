#!/usr/bin/env bash
# Per-boot runtime initialization for the mariadb-operator Cloud Agent environment.
# Configures kernel limits and the Docker daemon, then starts Docker so KIND and
# image builds work. Runs as root (invoked via sudo from environment.json).
set -euo pipefail

# KIND runs a full Kubernetes node inside a container; the default inotify limits
# are too low for the control-plane components and cause bootstrap timeouts.
sysctl -w fs.inotify.max_user_watches=1048576 || true
sysctl -w fs.inotify.max_user_instances=8192 || true
sysctl -w fs.inotify.max_queued_events=16384 || true

# The pod root filesystem is itself an overlay mount, so Docker cannot use the
# kernel overlay2 driver (overlay-on-overlay returns EINVAL). fuse-overlayfs works
# in this nested setup. containerd-snapshotter is disabled so storage-driver applies.
mkdir -p /etc/docker
cat > /etc/docker/daemon.json <<'JSON'
{
  "features": { "containerd-snapshotter": false },
  "storage-driver": "fuse-overlayfs"
}
JSON

if ! docker info >/dev/null 2>&1; then
  if [ -x /etc/init.d/docker ]; then
    service docker restart || service docker start || true
  else
    ( dockerd >/var/log/dockerd.log 2>&1 & )
  fi
fi

for _ in $(seq 1 60); do
  if docker info >/dev/null 2>&1; then
    break
  fi
  sleep 1
done

if ! docker info >/dev/null 2>&1; then
  echo "ERROR: Docker daemon failed to start" >&2
  tail -n 50 /var/log/dockerd.log 2>/dev/null || true
  exit 1
fi

# Let the non-root ubuntu user talk to the daemon without sudo.
chmod 666 /var/run/docker.sock || true

echo "Docker is ready ($(docker info --format '{{.Driver}}' 2>/dev/null) storage driver)."

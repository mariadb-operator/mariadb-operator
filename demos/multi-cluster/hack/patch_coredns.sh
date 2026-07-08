#!/usr/bin/env bash

set -eo pipefail

COREFILE=$(kubectl get configmap coredns -n kube-system -o jsonpath='{.data.Corefile}')

if echo "$COREFILE" | grep -q "mariadb-eu-south.mariadb.com"; then
    echo "CoreDNS already patched, skipping."
else
    PATCHED=$(echo "$COREFILE" | awk '
    /forward \. \/etc\/resolv\.conf/ {
        print "    hosts {"
        print "        172.18.1.10 mariadb-eu-south.mariadb.com"
        print "        172.18.1.15 mariadb-eu-central.mariadb.com"
        print "        172.18.0.200 minio.mariadb.com"
        print "        fallthrough"
        print "    }"
    }
    { print }
    ')

    TMPFILE=$(mktemp)
    trap "rm -f $TMPFILE" EXIT

    {
        printf 'data:\n  Corefile: |\n'
        echo "$PATCHED" | sed 's/^/    /'
    } > "$TMPFILE"

    kubectl patch configmap coredns -n kube-system --patch-file "$TMPFILE"
fi

kubectl rollout restart deployment/coredns -n kube-system
kubectl rollout status deployment/coredns -n kube-system --timeout=60s

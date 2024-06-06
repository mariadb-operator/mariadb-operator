#!/bin/bash

set -eo pipefail

if [ -z "$BRANCH" ]; then 
  echo "BRANCH environment variable is mandatory"
  exit 1
fi
if [ -z "$BASE_URL" ]; then 
  echo "BASE_URL environment variable is mandatory"
  exit 1
fi
if [ -z "$GITHUB_TOKEN" ]; then
  echo "GITHUB_TOKEN environment variable is mandatory"
  exit 1
fi
export BASE_URL
export GITHUB_TOKEN

install_yq() {
  if ! command -v yq &> /dev/null; then
    echo "yq command not found, installing yq..."
    sudo curl -sSLo /usr/local/bin/yq https://github.com/mikefarah/yq/releases/download/v4.43.1/yq_linux_amd64
    sudo chmod +x /usr/local/bin/yq
  fi
}
install_yq

echo "Switching to \"$BRANCH\"."
git fetch --all
git checkout $BRANCH

echo "Updating index.yaml."
yq e -i '.entries.mariadb-operator[] |= . * {"urls": [env(BASE_URL) + .version]}' index.yaml
CURRENT_TIMESTAMP=$(date --utc +%Y-%m-%dT%H:%M:%SZ)
yq e -i ".generated = \"$CURRENT_TIMESTAMP\"" index.yaml

NEW_BRANCH="update-index-$(date +%s)"
echo "Pushing changes to \"$NEW_BRANCH\"."
git checkout -b $NEW_BRANCH
git add index.yaml
git commit -m "Update index.yaml"
git push origin $NEW_BRANCH

echo "Submitting PR."
gh pr create \
  --title "Update helm index.yaml" \
  --body "This PR has been raised automatically after releasing a new helm chart." \
  --assignee "mmontes11" \
  --base $BRANCH \
  --head $NEW_BRANCH

echo "Done!"
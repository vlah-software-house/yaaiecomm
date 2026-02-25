#!/bin/bash
# Creates the forgecommerce-env K8s secret from .secrets file
set -euo pipefail

SECRETS_FILE="${1:-../.secrets}"
NAMESPACE="yaaiecomm"
SECRET_NAME="forgecommerce-env"

if [ ! -f "$SECRETS_FILE" ]; then
    echo "Error: $SECRETS_FILE not found"
    exit 1
fi

# Build --from-literal args from .secrets (skip comments and blank lines)
ARGS=()
while IFS= read -r line; do
    # Skip comments and empty lines
    [[ -z "$line" || "$line" =~ ^[[:space:]]*# ]] && continue
    ARGS+=("--from-literal=$line")
done < "$SECRETS_FILE"

# Delete existing secret if present
kubectl delete secret "$SECRET_NAME" -n "$NAMESPACE" --ignore-not-found

# Create the secret
kubectl create secret generic "$SECRET_NAME" -n "$NAMESPACE" "${ARGS[@]}"

echo "Secret $SECRET_NAME created in namespace $NAMESPACE"

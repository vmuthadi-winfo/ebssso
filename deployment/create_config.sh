#!/bin/bash
set -e

TEMPLATE_FILE="deployment/config.yaml.ci_template"
OUTPUT_FILE="config.yaml"

if [ ! -f "$TEMPLATE_FILE" ]; then
    echo "Error: Template file $TEMPLATE_FILE not found."
    exit 1
fi

# Function to replace placeholder
replace_placeholder() {
    local key="$1"
    local value="$2"
    # Escape slashes in value for sed
    local escaped_value=$(echo "$value" | sed 's/\//\\\//g')
    sed -i "s/{{$key}}/$escaped_value/g" "$OUTPUT_FILE"
}

cp "$TEMPLATE_FILE" "$OUTPUT_FILE"

# Required inputs
replace_placeholder "EBS_URL" "${EBS_URL}"
replace_placeholder "OIDC_CLIENT_ID" "${OIDC_CLIENT_ID}"
replace_placeholder "OIDC_CLIENT_SECRET" "${OIDC_CLIENT_SECRET}"
replace_placeholder "OIDC_PROVIDER_URL" "${OIDC_PROVIDER_URL}"
replace_placeholder "OIDC_REDIRECT_URL" "${OIDC_REDIRECT_URL}"
replace_placeholder "COOKIE_DOMAIN" "${COOKIE_DOMAIN}"

# Database configuration
if [ -n "$DBC_FILE_PATH" ]; then
    replace_placeholder "USE_DBC" "true"
    replace_placeholder "DBC_FILE_PATH" "${DBC_FILE_PATH}"
    replace_placeholder "DB_USER" ""
    replace_placeholder "DB_PASSWORD" ""
    replace_placeholder "DB_CONNECTION_STRING" ""
else
    replace_placeholder "USE_DBC" "false"
    replace_placeholder "DBC_FILE_PATH" ""
    replace_placeholder "DB_USER" "${DB_USER:-apps}"
    replace_placeholder "DB_PASSWORD" "${DB_PASSWORD}"
    replace_placeholder "DB_CONNECTION_STRING" "${DB_CONNECTION_STRING}"
fi

replace_placeholder "TRUSTED_NODE_SECRET" "${TRUSTED_NODE_SECRET}"

echo "Generated $OUTPUT_FILE"

#!/bin/bash
exec env -u http_proxy -u https_proxy -u HTTP_PROXY -u HTTPS_PROXY -u all_proxy -u ALL_PROXY NO_PROXY='*' node "$(dirname "$0")/step.mjs" "$@"

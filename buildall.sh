#!/usr/bin/env bash

set -e

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"

build_module() {
	local source="$1"
	local output_dir="$2"
	local name="$3"

	mkdir -p "$output_dir"

	cd "$source"

	# Linux
	go build -o "$output_dir/$name" .

	# Windows
	GOOS=windows GOARCH=amd64 \
		go build -o "$output_dir/$name.exe" .
}

build_module \
	"$SCRIPT_DIR/basicModules/simple_chat" \
	"$SCRIPT_DIR/modules/ui" \
	"familiar_ui"

build_module \
	"$SCRIPT_DIR/basicModules/simple_history" \
	"$SCRIPT_DIR/modules/history" \
	"familiar_history"

build_module \
	"$SCRIPT_DIR/basicModules/simple_context" \
	"$SCRIPT_DIR/modules/context" \
	"familiar_context"

build_module \
	"$SCRIPT_DIR/basicModules/simple_knowledge" \
	"$SCRIPT_DIR/modules/knowledge" \
	"familiar_knowledge"

build_module \
	"$SCRIPT_DIR/basicModules/datetime" \
	"$SCRIPT_DIR/modules/datetime" \
	"familiar_datetime"

build_module \
	"$SCRIPT_DIR/basicModules/records" \
	"$SCRIPT_DIR/modules/records" \
	"familiar_records"

build_module \
	"$SCRIPT_DIR/openRouter_proxy" \
	"$SCRIPT_DIR/engines/openRouter" \
	"openRouter_proxy"

cd "$SCRIPT_DIR/src"

# Linux
go build -o "$SCRIPT_DIR/familiar" .

# Windows
GOOS=windows GOARCH=amd64 \
	go build -o "$SCRIPT_DIR/familiar.exe" .
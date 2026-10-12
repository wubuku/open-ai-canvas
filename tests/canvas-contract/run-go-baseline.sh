#!/usr/bin/env bash
set -euo pipefail

SUITE_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SUITE_ROOT/../.." && pwd)"
ROUNDS=1
PROFILES="core,share-public,project-integration,project-workbench-core,resource-integrity,import,legacy-assets,public-resource,restart-recovery,generation-boundary,generation-writeback"
BACKEND_PORT=18183
BROWSER=false
FRONTEND_PORT=18184

for argument in "$@"; do
    case "$argument" in
        --rounds=*) ROUNDS="${argument#*=}" ;;
        --profiles=*) PROFILES="${argument#*=}" ;;
        --backend-port=*) BACKEND_PORT="${argument#*=}" ;;
        --browser) BROWSER=true ;;
        --frontend-port=*) FRONTEND_PORT="${argument#*=}" ;;
        *) printf 'Unknown argument: %s\n' "$argument" >&2; exit 1 ;;
    esac
done
[[ "$FRONTEND_PORT" =~ ^[0-9]+$ ]] && (( FRONTEND_PORT >= 1024 && FRONTEND_PORT <= 65535 && FRONTEND_PORT != BACKEND_PORT )) || { echo 'Invalid frontend port' >&2; exit 1; }
[[ "$ROUNDS" =~ ^[1-9][0-9]*$ && "$BACKEND_PORT" =~ ^[0-9]+$ ]] || { echo 'Invalid rounds or port' >&2; exit 1; }
(( ROUNDS <= 10 && BACKEND_PORT >= 1024 && BACKEND_PORT <= 65535 )) || { echo 'Rounds must be 1..10; port must be 1024..65535' >&2; exit 1; }
for command in go docker bun curl lsof openssl shasum; do
    command -v "$command" >/dev/null || { printf 'Missing prerequisite: %s\n' "$command" >&2; exit 1; }
done
[[ -n "$PROFILES" && "$PROFILES" != *, && "$PROFILES" != ,* && "$PROFILES" != *,,* ]] || { echo 'Invalid profile list' >&2; exit 1; }
IFS=',' read -r -a SELECTED_PROFILES <<< "$PROFILES"
SEEN_PROFILES=','
for profile in "${SELECTED_PROFILES[@]}"; do
    case "$profile" in
        core|share-public|project-integration|project-workbench-core|resource-integrity|import|legacy-assets|public-resource|restart-recovery|generation-boundary|generation-writeback) ;;
        *) printf 'Unknown profile: %s\n' "$profile" >&2; exit 1 ;;
    esac
    [[ "$SEEN_PROFILES" != *",$profile,"* ]] || { echo 'Duplicate profile' >&2; exit 1; }
    SEEN_PROFILES="$SEEN_PROFILES$profile,"
done
if [[ "$BROWSER" == true ]]; then
    [[ "$SEEN_PROFILES" == *,core,* ]] || { echo 'Go browser baseline requires the core profile bootstrap' >&2; exit 1; }
    command -v python3 >/dev/null || { echo 'Browser baseline requires Python 3' >&2; exit 1; }
    python3 -c 'import playwright.sync_api' || { echo 'Install the isolated browser runner prerequisites first' >&2; exit 1; }
    [[ -f "$REPO_ROOT/web/node_modules/vite/bin/vite.js" ]] || { echo 'Install the frontend Bun dependencies first' >&2; exit 1; }
    ! lsof -nP -iTCP:"$FRONTEND_PORT" -sTCP:LISTEN >/dev/null 2>&1 || { echo 'Frontend port occupied; no process will be killed' >&2; exit 1; }
fi
! lsof -nP -iTCP:"$BACKEND_PORT" -sTCP:LISTEN >/dev/null 2>&1 || { echo 'Backend port occupied; choose --backend-port, no process will be killed' >&2; exit 1; }
docker info >/dev/null 2>&1 || { echo 'Docker is unavailable' >&2; exit 1; }
umask 077
RUN_ID="$(date +%Y%m%d-%H%M%S)-$$"
ARTIFACT_ROOT="$REPO_ROOT/.local/project-workbench-debug/canvas-contract-review/$RUN_ID"
PRIVATE_ROOT="$ARTIFACT_ROOT/private"
mkdir -p "$PRIVATE_ROOT"
PG_NAME="canvas-contract-pg-$RUN_ID"
REDIS_NAME="canvas-contract-redis-$RUN_ID"
NETWORK_NAME="canvas-contract-network-$RUN_ID"
UPSTREAM_NAME="canvas-contract-upstream-$RUN_ID"
TARGET_NAME="canvas-contract-target-$RUN_ID"
GATEWAY_NAME="canvas-contract-gateway-$RUN_ID"
PG_CREATED=0
REDIS_CREATED=0
NETWORK_CREATED=0
UPSTREAM_CREATED=0
TARGET_CREATED=0
GATEWAY_CREATED=0
BACKEND_PID=''
FRONTEND_PID=''
SHARE_FIXTURE_PID=''
PASSWORD="$(openssl rand -hex 18)"

stop_backend() {
    if [[ -n "$FRONTEND_PID" ]]; then
        kill "$FRONTEND_PID" 2>/dev/null || true
        wait "$FRONTEND_PID" 2>/dev/null || true
        FRONTEND_PID=''
    fi
    if [[ -n "$SHARE_FIXTURE_PID" ]]; then
        kill "$SHARE_FIXTURE_PID" 2>/dev/null || true
        wait "$SHARE_FIXTURE_PID" 2>/dev/null || true
        SHARE_FIXTURE_PID=''
    fi
    if (( GATEWAY_CREATED )); then
        docker rm --force --volumes "$GATEWAY_NAME" >/dev/null 2>&1 || true
        GATEWAY_CREATED=0
    fi
    if (( TARGET_CREATED )); then
        docker rm --force --volumes "$TARGET_NAME" >/dev/null 2>&1 || true
        TARGET_CREATED=0
    fi
    if [[ -n "$BACKEND_PID" ]]; then
        kill "$BACKEND_PID" 2>/dev/null || true
        wait "$BACKEND_PID" 2>/dev/null || true
        BACKEND_PID=''
    fi
}
cleanup() {
    stop_backend
    if (( UPSTREAM_CREATED )); then docker rm --force --volumes "$UPSTREAM_NAME" >/dev/null 2>&1 || true; fi
    if (( REDIS_CREATED )); then docker rm --force --volumes "$REDIS_NAME" >/dev/null 2>&1 || true; fi
    if (( PG_CREATED )); then docker rm --force --volumes "$PG_NAME" >/dev/null 2>&1 || true; fi
    if (( NETWORK_CREATED )); then docker network rm "$NETWORK_NAME" >/dev/null 2>&1 || true; fi
    if [[ -d "$PRIVATE_ROOT" ]]; then find "$PRIVATE_ROOT" -depth -delete; fi
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

printf 'Building Go target; evidence: %s\n' "${ARTIFACT_ROOT#"$REPO_ROOT/"}"
(
    cd "$REPO_ROOT/backend"
    GOCACHE="$REPO_ROOT/.local/cache/go-build" GOMODCACHE="$REPO_ROOT/.local/cache/go-mod" go build -o "$PRIVATE_ROOT/server" ./cmd/server
)
(
    cd "$SUITE_ROOT"
    go build -o "$PRIVATE_ROOT/target-control" ./cmd/target-control
    go build -o "$PRIVATE_ROOT/share-fixture" ./cmd/share-fixture
)
docker run --detach --name "$PG_NAME" -e POSTGRES_USER=canvas -e "POSTGRES_PASSWORD=$PASSWORD" -e POSTGRES_DB=canvas -p 127.0.0.1::5432 postgres:18 >/dev/null
PG_CREATED=1
docker run --detach --name "$REDIS_NAME" -p 127.0.0.1::6379 redis:7-alpine >/dev/null
REDIS_CREATED=1
DEPENDENCIES_READY=0
for attempt in $(seq 1 60); do
    if docker exec "$PG_NAME" pg_isready -h 127.0.0.1 -U canvas -d canvas >/dev/null 2>&1 && [[ "$(docker exec "$REDIS_NAME" redis-cli ping 2>/dev/null)" == PONG ]]; then DEPENDENCIES_READY=1; break; fi
    sleep 1
done
(( DEPENDENCIES_READY )) || { echo 'Isolated dependencies did not become ready' >&2; exit 1; }
PG_PORT="$(docker port "$PG_NAME" 5432/tcp | sed 's/.*://')"
REDIS_PORT="$(docker port "$REDIS_NAME" 6379/tcp | sed 's/.*://')"
if [[ "$SEEN_PROFILES" == *,import,* ]]; then
    case "$(docker info --format '{{.Architecture}}')" in
        aarch64|arm64) DOCKER_ARCH=arm64 ;;
        x86_64|amd64) DOCKER_ARCH=amd64 ;;
        *) echo 'Unsupported Docker architecture for the isolated import target' >&2; exit 1 ;;
    esac
    (
        cd "$REPO_ROOT/backend"
        CGO_ENABLED=0 GOOS=linux GOARCH="$DOCKER_ARCH" GOCACHE="$REPO_ROOT/.local/cache/go-build" GOMODCACHE="$REPO_ROOT/.local/cache/go-mod" go build -o "$PRIVATE_ROOT/server-linux" ./cmd/server
    )
    (
        cd "$SUITE_ROOT"
        CGO_ENABLED=0 GOOS=linux GOARCH="$DOCKER_ARCH" go build -o "$PRIVATE_ROOT/import-upstream" ./cmd/import-upstream
        CGO_ENABLED=0 GOOS=linux GOARCH="$DOCKER_ARCH" go build -o "$PRIVATE_ROOT/http-gateway" ./cmd/http-gateway
    )
    docker network create --internal "$NETWORK_NAME" >/dev/null
    NETWORK_CREATED=1
    docker network connect "$NETWORK_NAME" "$PG_NAME"
    docker network connect "$NETWORK_NAME" "$REDIS_NAME"
    docker run --detach --name "$UPSTREAM_NAME" --network "$NETWORK_NAME" --user "$(id -u):$(id -g)" \
        --mount "type=bind,src=$PRIVATE_ROOT,dst=/contract" --entrypoint /contract/import-upstream postgres:18 \
        --listen=0.0.0.0:18081 --ca-file=/contract/import-ca.pem >/dev/null
    UPSTREAM_CREATED=1
    CA_READY=0
    for attempt in $(seq 1 30); do
        if [[ -s "$PRIVATE_ROOT/import-ca.pem" ]]; then CA_READY=1; break; fi
        [[ "$(docker inspect --format '{{.State.Running}}' "$UPSTREAM_NAME")" == true ]] || break
        sleep 1
    done
    (( CA_READY )) || { echo 'Controlled import upstream did not initialize its ephemeral CA' >&2; exit 1; }
fi
{
    printf 'checkout='; git -C "$REPO_ROOT" rev-parse HEAD
    go version
    printf 'bun='; bun --version
    docker exec "$PG_NAME" psql -U canvas -d canvas -Atc 'SHOW server_version'
    docker exec "$REDIS_NAME" redis-server --version
    docker image inspect postgres:18 redis:7-alpine --format '{{.Id}}'
    printf 'rounds=%s profiles=%s\n' "$ROUNDS" "$PROFILES"
    printf 'browser=%s\n' "$BROWSER"
    if [[ "$SEEN_PROFILES" == *,import,* ]]; then
        printf 'import_target=linux-static-on-internal-docker-network; private-API-gateway; ephemeral-CA; no-upstream-forwarding\n'
    else
        printf 'import_target=not-selected\n'
    fi
    find "$SUITE_ROOT" -type f | sort | while IFS= read -r filename; do shasum -a 256 "$filename" | sed "s|$REPO_ROOT/||"; done
    printf 'runtime_binary_sha256 '
    shasum -a 256 "$PRIVATE_ROOT/server" | sed "s|$REPO_ROOT/||"
    if [[ -f "$PRIVATE_ROOT/server-linux" ]]; then
        shasum -a 256 "$PRIVATE_ROOT/server-linux" "$PRIVATE_ROOT/import-upstream" "$PRIVATE_ROOT/http-gateway" | sed 's|^|runtime_binary_sha256 |; s|'"$REPO_ROOT"'/||'
    fi
    printf 'runtime_binary_sha256 '
    shasum -a 256 "$PRIVATE_ROOT/share-fixture" | sed "s|$REPO_ROOT/||"
} > "$ARTIFACT_ROOT/evidence.txt"

COUNTER=0
for round in $(seq 1 "$ROUNDS"); do
    RESULT_ARGUMENTS=()
    for profile in "${SELECTED_PROFILES[@]}"; do
        COUNTER=$((COUNTER + 1))
        DATABASE="canvas_run_$COUNTER"
        PROFILE_ROOT="$ARTIFACT_ROOT/$round-$profile"
        DATA_ROOT="$PRIVATE_ROOT/data-$COUNTER"
        mkdir -p "$PROFILE_ROOT" "$DATA_ROOT/tmp"
        docker exec "$PG_NAME" psql -U canvas -d canvas -v ON_ERROR_STOP=1 -c "CREATE DATABASE $DATABASE" >/dev/null
        docker exec "$REDIS_NAME" redis-cli FLUSHALL >/dev/null
        CONTROL_TOKEN="$(openssl rand -hex 24)"
        CONTROL_URL=''
        SHARE_FIXTURE_TOKEN=''
        SHARE_FIXTURE_URL=''
        SHARE_FIXTURE_READY="$PRIVATE_ROOT/share-fixture-$COUNTER.txt"
        IMPORT_FIXTURE=false
        ALLOWED_UPSTREAM_HOSTS=''
        if [[ "$profile" == resource-integrity || "$profile" == public-resource || "$profile" == share-public ]]; then ALLOWED_UPSTREAM_HOSTS=127.0.0.1; fi
        TARGET_COMMAND=("$PRIVATE_ROOT/server")
        if [[ "$profile" == restart-recovery ]]; then
            TARGET_COMMAND=("$PRIVATE_ROOT/target-control" "--target-bin=$PRIVATE_ROOT/server" "--target-dir=$REPO_ROOT/backend" "--target-origin=http://127.0.0.1:$BACKEND_PORT" "--ready-file=$PRIVATE_ROOT/control-$COUNTER.json")
        fi
        if [[ "$profile" == import ]]; then
            IMPORT_FIXTURE=true
            docker run --detach --name "$TARGET_NAME" --network "$NETWORK_NAME" --add-host files.tapnow.media:203.0.113.10 --user "$(id -u):$(id -g)" --workdir /data \
                --mount "type=bind,src=$PRIVATE_ROOT,dst=/contract,readonly" --mount "type=bind,src=$DATA_ROOT,dst=/data" \
                --entrypoint /contract/server-linux \
                -e CANVAS_DATABASE_DRIVER=postgres -e "DATABASE_URL=postgres://canvas:$PASSWORD@$PG_NAME:5432/$DATABASE?sslmode=disable" -e "REDIS_URL=redis://$REDIS_NAME:6379/0" \
                -e TMPDIR=/data/tmp -e CANVAS_BACKEND_DATA_DIR=/data -e "CANVAS_BACKEND_ADDR=0.0.0.0:$BACKEND_PORT" \
                -e CANVAS_REGISTRATION_ENABLED=false -e CANVAS_AUTO_MIGRATE=true -e GIN_MODE=debug -e CANVAS_LOG_LEVEL=info \
                -e YINGCE_AGENT_URL=http://127.0.0.1:9 -e YINGCE_AGENT_TOKEN=contract-fixture-only-not-a-real-agent-token -e "HTTPS_PROXY=http://$UPSTREAM_NAME:18081" -e "HTTP_PROXY=http://$UPSTREAM_NAME:18081" \
                -e SSL_CERT_FILE=/contract/import-ca.pem -e CANVAS_ALLOW_PRIVATE_UPSTREAMS=false -e CANVAS_ALLOWED_PRIVATE_UPSTREAM_HOSTS= \
                postgres:18 >/dev/null
            TARGET_CREATED=1
            docker run --detach --name "$GATEWAY_NAME" --network bridge --user "$(id -u):$(id -g)" \
                --mount "type=bind,src=$PRIVATE_ROOT,dst=/contract,readonly" -p "127.0.0.1:$BACKEND_PORT:$BACKEND_PORT" \
                --entrypoint /contract/http-gateway postgres:18 --listen="0.0.0.0:$BACKEND_PORT" --target-origin="http://$TARGET_NAME:$BACKEND_PORT" >/dev/null
            GATEWAY_CREATED=1
            docker network connect "$NETWORK_NAME" "$GATEWAY_NAME"
        else
        (
            cd "$REPO_ROOT/backend"
            CANVAS_DATABASE_DRIVER=postgres DATABASE_URL="postgres://canvas:$PASSWORD@127.0.0.1:$PG_PORT/$DATABASE?sslmode=disable" REDIS_URL="redis://127.0.0.1:$REDIS_PORT/0" \
            TMPDIR="$DATA_ROOT/tmp" CANVAS_BACKEND_DATA_DIR="$DATA_ROOT" CANVAS_BACKEND_ADDR="127.0.0.1:$BACKEND_PORT" CANVAS_REGISTRATION_ENABLED=false \
            CANVAS_AUTO_MIGRATE=true CANVAS_LOG_LEVEL=info GIN_MODE=debug YINGCE_AGENT_URL='' CANVAS_PUBLIC_BASE_URL='' CANVAS_ALLOWED_PRIVATE_UPSTREAM_HOSTS="$ALLOWED_UPSTREAM_HOSTS" CANVAS_ALLOW_PRIVATE_UPSTREAMS=false \
            CANVAS_CONTRACT_CONTROL_TOKEN="$CONTROL_TOKEN" exec "${TARGET_COMMAND[@]}"
        ) > "$PRIVATE_ROOT/service-$COUNTER.log" 2>&1 &
        BACKEND_PID=$!
        fi
        READY=0
        for attempt in $(seq 1 60); do
            if curl --noproxy '*' --fail --silent --max-time 2 "http://127.0.0.1:$BACKEND_PORT/api/health" >/dev/null; then READY=1; break; fi
            if (( TARGET_CREATED )); then
                [[ "$(docker inspect --format '{{.State.Running}}' "$TARGET_NAME")" == true ]] || break
            else
                kill -0 "$BACKEND_PID" 2>/dev/null || break
            fi
            sleep 1
        done
        if (( ! READY )); then
            if (( TARGET_CREATED )); then
                docker inspect --format 'isolated target running={{.State.Running}} exit={{.State.ExitCode}}' "$TARGET_NAME" >&2
                docker logs "$TARGET_NAME" > "$PRIVATE_ROOT/service-$COUNTER.log" 2>&1
                CANVAS_CONTRACT_REDACT_PASSWORD="$PASSWORD" bun -e 'const lines=(await Bun.file(process.argv[1]).text()).split("\n").filter(line=>/panic:|fatal|error|permission|failed|listening/i.test(line)); for(const line of lines.slice(-8)) console.error(line.replaceAll(process.env.CANVAS_CONTRACT_REDACT_PASSWORD,"<redacted>"))' "$PRIVATE_ROOT/service-$COUNTER.log"
            fi
            printf 'Target failed to start: round=%s profile=%s\n' "$round" "$profile" >&2
            exit 1
        fi
        if [[ "$profile" == restart-recovery ]]; then
            CONTROL_READY=0
            for attempt in $(seq 1 30); do
                if [[ -s "$PRIVATE_ROOT/control-$COUNTER.json" ]]; then CONTROL_READY=1; break; fi
                kill -0 "$BACKEND_PID" 2>/dev/null || break
                sleep 1
            done
            (( CONTROL_READY )) || { echo 'Private target control did not become ready' >&2; exit 1; }
            CONTROL_URL="$(bun -e 'console.log((await Bun.file(process.argv[1]).json()).origin)' "$PRIVATE_ROOT/control-$COUNTER.json")"
        fi
        if [[ "$profile" == share-public ]]; then
            SHARE_FIXTURE_TOKEN="$(openssl rand -hex 24)"
            (
                cd "$SUITE_ROOT"
                CANVAS_CONTRACT_SHARE_FIXTURE_TOKEN="$SHARE_FIXTURE_TOKEN" exec "$PRIVATE_ROOT/share-fixture" \
                    --database-url="postgres://canvas:$PASSWORD@127.0.0.1:$PG_PORT/$DATABASE?sslmode=disable" \
                    --ready-file="$SHARE_FIXTURE_READY"
            ) > "$PRIVATE_ROOT/share-fixture-$COUNTER.log" 2>&1 &
            SHARE_FIXTURE_PID=$!
            SHARE_FIXTURE_READY_STATE=0
            for attempt in $(seq 1 30); do
                if [[ -s "$SHARE_FIXTURE_READY" ]]; then SHARE_FIXTURE_READY_STATE=1; break; fi
                kill -0 "$SHARE_FIXTURE_PID" 2>/dev/null || break
                sleep 1
            done
            (( SHARE_FIXTURE_READY_STATE )) || { cat "$PRIVATE_ROOT/share-fixture-$COUNTER.log" >&2; echo 'Private share-expiry fixture did not become ready' >&2; exit 1; }
            SHARE_FIXTURE_URL="$(cat "$SHARE_FIXTURE_READY")"
        fi
        if (( TARGET_CREATED )); then docker logs "$TARGET_NAME" > "$PRIVATE_ROOT/service-$COUNTER.log" 2>&1; fi
        awk '/^\[GIN-debug\] (GET|POST|PUT|PATCH|DELETE) /' "$PRIVATE_ROOT/service-$COUNTER.log" > "$PROFILE_ROOT/routes.log"
        printf 'Running round=%s profile=%s\n' "$round" "$profile"
        (
            cd "$SUITE_ROOT"
            CANVAS_CONTRACT_BASE_URL="http://127.0.0.1:$BACKEND_PORT" CANVAS_CONTRACT_REGISTER_USERS=true CANVAS_CONTRACT_PROFILE="$profile" \
            CANVAS_CONTRACT_USER_A_USERNAME='' CANVAS_CONTRACT_USER_A_PASSWORD='' CANVAS_CONTRACT_USER_B_USERNAME='' CANVAS_CONTRACT_USER_B_PASSWORD='' \
            CANVAS_CONTRACT_IMPLEMENTATION=go CANVAS_CONTRACT_DB_PROFILE=postgres-18 CANVAS_CONTRACT_RUN_ID="$RUN_ID-$COUNTER" \
            CANVAS_CONTRACT_CONTROL_URL="$CONTROL_URL" CANVAS_CONTRACT_CONTROL_TOKEN="$CONTROL_TOKEN" \
            CANVAS_CONTRACT_SHARE_FIXTURE_URL="$SHARE_FIXTURE_URL" CANVAS_CONTRACT_SHARE_FIXTURE_TOKEN="$SHARE_FIXTURE_TOKEN" \
            CANVAS_CONTRACT_IMPORT_FIXTURE="$IMPORT_FIXTURE" CANVAS_CONTRACT_ADMIN_USERNAME='' CANVAS_CONTRACT_ADMIN_PASSWORD='' \
            CANVAS_CONTRACT_SETTINGS_FIXTURE=true \
            go test ./... -count=1 -json > "$PROFILE_ROOT/result.jsonl"
        )
        if [[ "$BROWSER" == true && "$profile" == core ]]; then
            (
                cd "$REPO_ROOT/web"
                VITE_API_PROXY_TARGET="http://127.0.0.1:$BACKEND_PORT" VITE_CANVAS_BACKEND_URL=/api exec bun node_modules/vite/bin/vite.js --host 127.0.0.1 --port "$FRONTEND_PORT" --strictPort
            ) > "$PRIVATE_ROOT/vite-$COUNTER.log" 2>&1 &
            FRONTEND_PID=$!
            FRONTEND_READY=0
            for attempt in $(seq 1 60); do
                if curl --fail --silent "http://127.0.0.1:$FRONTEND_PORT/login" >/dev/null && curl --fail --silent "http://127.0.0.1:$FRONTEND_PORT/api/health" >/dev/null; then FRONTEND_READY=1; break; fi
                kill -0 "$FRONTEND_PID" 2>/dev/null || break
                sleep 1
            done
            (( FRONTEND_READY )) || { echo 'Real Vite frontend failed to start on the isolated API target' >&2; exit 1; }
            CANVAS_CONTRACT_BROWSER_ADMIN_USERNAME="$(python3 -c 'import hashlib,sys; print("canvas_admin_" + hashlib.sha256(sys.argv[1].encode()).hexdigest()[:12])' "$RUN_ID-$COUNTER")" \
            CANVAS_CONTRACT_BROWSER_ADMIN_PASSWORD='CanvasContract-Bootstrap-2026!' CANVAS_CONTRACT_IMPLEMENTATION=go PYTHONDONTWRITEBYTECODE=1 \
                python3 "$SUITE_ROOT/browser/run.py" --implementation=go --web-origin="http://127.0.0.1:$FRONTEND_PORT" --run-id="$RUN_ID-$COUNTER-browser" --report="$PROFILE_ROOT/browser-report.json" --isolated-fixture > "$PROFILE_ROOT/browser-result.log" 2>&1 || { cat "$PROFILE_ROOT/browser-result.log" >&2; exit 1; }
            printf 'Browser round %s passed; separate real-page report generated.\n' "$round"
        fi
        stop_backend
        RESULT_ARGUMENTS+=("--results=$PROFILE_ROOT/result.jsonl")
    done
    bun "$SUITE_ROOT/check-catalog.ts" "--routes-log=$PROFILE_ROOT/routes.log" "${RESULT_ARGUMENTS[@]}" > "$ARTIFACT_ROOT/report-round-$round.json"
    printf 'Round %s passed; report generated.\n' "$round"
done
printf 'Baseline complete; Required remaining_paths and contract approval are still separate gates.\n'

#!/bin/bash

################################################################################
# ImmoLux Deployment Script
#
# Deploys backend (Go) and frontend (SvelteKit) with minimal downtime:
# - Pulls latest changes from git (--ff-only, machine-generated files restored
#   first so pulls never fail on a dirty package-lock.json / go.sum again)
# - Installs dependencies reproducibly (npm ci — never rewrites the lockfile;
#   go mod download — never rewrites go.mod/go.sum)
# - Builds BOTH services BEFORE stopping anything: a failed build leaves the
#   currently running version untouched
# - Swaps the backend binary atomically, restarts services, health-checks
#
# Usage: ./deploy.sh [options]
#   --skip-restart    Build and stage everything, but do not restart services
#   --backend-only    Only update/build/restart the backend
#   --frontend-only   Only update/build/restart the frontend
#   --force-install   If npm ci fails (lockfile out of sync), fall back to
#                     npm install instead of aborting
#   --help            Show this help
#
# Environment overrides:
#   PROJECT_ROOT         (default /opt/immolux)
#   BACKEND_HEALTH_URL   (default http://127.0.0.1:8082/v1/api/ping)
################################################################################

set -Eeuo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
PROJECT_ROOT="${PROJECT_ROOT:-/opt/immolux}"
BACKEND_DIR="$PROJECT_ROOT/backend"
FRONTEND_DIR="$PROJECT_ROOT/frontend"
SERVICE_NAME="immolux"
FRONTEND_SERVICE_NAME="immolux-frontend"
BACKEND_BINARY="immo-lux-server"
BACKEND_HEALTH_URL="${BACKEND_HEALTH_URL:-http://127.0.0.1:8082/v1/api/ping}"

SKIP_RESTART=false
DO_BACKEND=true
DO_FRONTEND=true
FORCE_INSTALL=false
SERVICES_STOPPED=false

################################################################################
# Helper Functions
################################################################################

log_info()    { echo -e "${BLUE}[INFO]${NC} $1"; }
log_success() { echo -e "${GREEN}[SUCCESS]${NC} $1"; }
log_warning() { echo -e "${YELLOW}[WARNING]${NC} $1"; }
log_error()   { echo -e "${RED}[ERROR]${NC} $1"; }

banner() {
    log_info "========================================="
    log_info "  $1"
    log_info "========================================="
    echo ""
}

usage() {
    sed -n '/^# Usage:/,/^####/p' "$0" | sed 's/^# \{0,1\}//' | head -n -1
}

check_command() {
    if ! command -v "$1" &> /dev/null; then
        log_error "$1 is not installed. Please install it first."
        exit 1
    fi
}

# Frontend toolchain (ESLint 10, vite-imagetools 12) requires Node ^22.13 || >=24;
# .npmrc sets engine-strict, so an older Node makes `npm ci` fail.
# Recommended: Node 26 + npm 12. npm < 12 ignores the front end's package.json
# `allowScripts` policy and runs every dependency install script.
check_node_version() {
    local major minor npm_major
    major=$(node --version | sed 's/v//' | cut -d. -f1)
    minor=$(node --version | sed 's/v//' | cut -d. -f2)
    if [ "$major" -ge 24 ] || { [ "$major" -eq 22 ] && [ "$minor" -ge 13 ]; }; then
        log_info "Node.js version check passed: $(node --version)"
    else
        log_error "Node.js 22.13+ (or 24+) is required. Current version: $(node --version)"
        log_info "Install Node 26 and npm 12:"
        log_info "  curl -fsSL https://deb.nodesource.com/setup_26.x | sudo -E bash -"
        log_info "  sudo apt install -y nodejs"
        log_info "  sudo npm install -g npm@12"
        exit 1
    fi

    npm_major=$(npm --version | cut -d. -f1)
    if [ "$npm_major" -lt 12 ]; then
        log_warning "npm $(npm --version) ignores package.json allowScripts — upgrade with: sudo npm install -g npm@12"
    fi
}

# Keep in sync with the `go` directive in go.mod. An older local Go would
# otherwise download the toolchain at build time (or fail with GOTOOLCHAIN=local).
REQUIRED_GO_VERSION="1.27.1"

# version_ge A B — true when dotted version A >= B
version_ge() {
    [ "$(printf '%s\n%s\n' "$2" "$1" | sort -V | head -n1)" = "$2" ]
}

check_go_version() {
    local version
    version=$(go version | awk '{print $3}' | sed 's/^go//')
    if version_ge "$version" "$REQUIRED_GO_VERSION"; then
        log_info "Go version check passed: $version"
    else
        log_error "Go $REQUIRED_GO_VERSION+ is required. Current version: $version"
        log_info "Replace the old Go install with the official tarball:"
        log_info "  curl -fsSL https://go.dev/dl/go$REQUIRED_GO_VERSION.linux-amd64.tar.gz -o /tmp/go.tgz"
        log_info "  sudo rm -rf /usr/local/go && sudo tar -C /usr/local -xzf /tmp/go.tgz"
        exit 1
    fi
}

# Restore machine-generated files the build tools may have touched on the
# server, so `git pull` never fails because of them. Anything else that is
# dirty is reported but left alone.
git_prepare_tree() {
    local file
    for file in "$@"; do
        if [ -f "$file" ] && ! git diff --quiet -- "$file" 2>/dev/null; then
            log_warning "Restoring locally modified $file (server copies must stay pristine)"
            git checkout -- "$file"
        fi
    done
    if [ -n "$(git status --porcelain)" ]; then
        log_warning "Working tree has local changes (left untouched):"
        git status --porcelain | head -10
    fi
}

git_pull_repo() {
    log_info "Pulling latest changes from git..."
    git fetch origin
    local branch before after
    branch=$(git rev-parse --abbrev-ref HEAD)
    log_info "Current branch: $branch"

    before=$(git rev-parse HEAD)
    if ! git pull --ff-only origin "$branch"; then
        log_error "git pull failed. Resolve the conflict on the server (usually:"
        log_error "  git status && git checkout -- <file>  or  git stash) and redeploy."
        exit 1
    fi
    after=$(git rev-parse HEAD)

    if [ "$before" = "$after" ]; then
        log_info "Already up to date"
    else
        log_success "Updated: ${before:0:9} -> ${after:0:9}"
    fi
    echo ""
}

# If the deploy fails after services were stopped, try to bring them back up
# with whatever version is on disk instead of leaving the site down.
on_error() {
    local exit_code=$?
    log_error "Deployment failed (exit code $exit_code, line ${BASH_LINENO[0]})"
    if [ "$SERVICES_STOPPED" = true ]; then
        log_warning "Attempting to restart services with the version currently on disk..."
        [ "$DO_BACKEND" = true ] && sudo systemctl start "$SERVICE_NAME" || true
        [ "$DO_FRONTEND" = true ] && sudo systemctl start "$FRONTEND_SERVICE_NAME" || true
    fi
    exit "$exit_code"
}
trap on_error ERR

################################################################################
# Phases
################################################################################

preflight() {
    banner "PRE-FLIGHT CHECKS"

    if [ ! -d "$PROJECT_ROOT" ]; then
        log_error "Project root directory not found: $PROJECT_ROOT"
        exit 1
    fi

    check_command "git"
    if [ "$DO_BACKEND" = true ]; then
        check_command "go"
        check_go_version
    fi
    if [ "$DO_FRONTEND" = true ]; then
        check_command "npm"
        check_command "node"
        check_node_version
    fi

    log_success "Pre-flight checks passed!"
    echo ""
}

build_backend() {
    banner "BUILDING BACKEND"
    cd "$BACKEND_DIR"

    git_prepare_tree go.mod go.sum
    git_pull_repo

    # Reproducible install: go.mod/go.sum come from git and are never
    # rewritten here (no `go mod tidy` on the server!).
    log_info "Downloading Go dependencies..."
    go mod download
    log_success "Go dependencies ready"
    echo ""

    if [ -d "internal/database/ent/schema" ]; then
        log_info "Generating Ent client code..."
        go generate ./internal/database/ent
        log_success "Ent code generated"
        echo ""
    fi

    # Build to a staging name — the running binary is not touched until the
    # swap phase, and a failed build changes nothing.
    log_info "Building backend..."
    go build -ldflags="-s -w" -o "$BACKEND_BINARY.new" ./internal
    chmod +x "$BACKEND_BINARY.new"
    log_success "Backend built successfully ($(du -h "$BACKEND_BINARY.new" | cut -f1))"
    echo ""
}

build_frontend() {
    banner "BUILDING FRONTEND"
    cd "$FRONTEND_DIR"

    git_prepare_tree package-lock.json
    git_pull_repo

    # npm ci installs exactly what package-lock.json pins and NEVER rewrites
    # it — this is what keeps the next `git pull` conflict-free.
    log_info "Installing npm dependencies (npm ci)..."
    if ! npm ci --include=dev --no-audit --no-fund; then
        if [ "$FORCE_INSTALL" = true ]; then
            log_warning "npm ci failed — falling back to npm install (--force-install)"
            npm install --include=dev --no-audit --no-fund
        else
            log_error "npm ci failed. package-lock.json is likely out of sync with package.json."
            log_error "Fix it on your dev machine: run 'npm install', commit package-lock.json, redeploy."
            log_error "(Or rerun with --force-install to bypass once.)"
            exit 1
        fi
    fi
    log_success "npm dependencies installed"
    echo ""

    log_info "Building frontend for production..."
    npm run build
    log_success "Frontend built successfully"

    if [ -d "build" ]; then
        log_info "Build folder size: $(du -sh build | cut -f1), files: $(find build -type f | wc -l)"
    fi
    echo ""
}

verify_outputs() {
    banner "VERIFYING BUILD OUTPUTS"

    if [ "$DO_BACKEND" = true ]; then
        if [ -f "$BACKEND_DIR/$BACKEND_BINARY.new" ]; then
            log_success "Backend binary staged: $BACKEND_DIR/$BACKEND_BINARY.new"
        else
            log_error "Staged backend binary not found!"
            exit 1
        fi
    fi

    if [ "$DO_FRONTEND" = true ]; then
        if [ -f "$FRONTEND_DIR/build/index.js" ]; then
            log_success "Frontend build exists: $FRONTEND_DIR/build/index.js"
        else
            log_error "Frontend build/index.js not found!"
            exit 1
        fi
    fi
    echo ""
}

swap_backend_binary() {
    # Atomic on the same filesystem; safe even if the old binary is running.
    mv -f "$BACKEND_DIR/$BACKEND_BINARY.new" "$BACKEND_DIR/$BACKEND_BINARY"
    log_success "Backend binary swapped into place"
}

health_check_backend() {
    if ! command -v curl &> /dev/null; then
        return 0
    fi
    log_info "Health-checking backend at $BACKEND_HEALTH_URL ..."
    local i
    for i in $(seq 1 15); do
        if curl -sf --max-time 2 "$BACKEND_HEALTH_URL" > /dev/null 2>&1; then
            log_success "Backend responds to ping"
            return 0
        fi
        sleep 1
    done
    log_warning "Backend did not answer at $BACKEND_HEALTH_URL after 15s."
    log_warning "If the port/path differs, set BACKEND_HEALTH_URL. Logs:"
    log_warning "  sudo journalctl -u $SERVICE_NAME -n 50"
}

restart_services() {
    if [ "$SKIP_RESTART" = true ]; then
        log_warning "Skipping service restart (--skip-restart flag)"
        if [ "$DO_BACKEND" = true ]; then
            swap_backend_binary
            log_info "New backend binary is in place; restart manually with:"
            log_info "  sudo systemctl restart $SERVICE_NAME"
        fi
        if [ "$DO_FRONTEND" = true ]; then
            log_info "Restart frontend manually with:"
            log_info "  sudo systemctl restart $FRONTEND_SERVICE_NAME"
        fi
        echo ""
        return 0
    fi

    banner "RESTARTING SERVICES"

    # Everything is already built — the stop/start window is only seconds.
    log_info "Stopping services..."
    SERVICES_STOPPED=true
    if [ "$DO_FRONTEND" = true ]; then
        sudo systemctl stop "$FRONTEND_SERVICE_NAME" || log_warning "Frontend service was not running"
    fi
    if [ "$DO_BACKEND" = true ]; then
        sudo systemctl stop "$SERVICE_NAME" || log_warning "Backend service was not running"
    fi

    if [ "$DO_BACKEND" = true ]; then
        swap_backend_binary
        log_info "Starting backend service..."
        sudo systemctl start "$SERVICE_NAME"
        sleep 2
        if systemctl is-active --quiet "$SERVICE_NAME"; then
            log_success "Backend service started"
        else
            log_error "Backend service failed to start!"
            log_error "Check logs with: sudo journalctl -u $SERVICE_NAME -n 50"
            exit 1
        fi
        health_check_backend
    fi

    if [ "$DO_FRONTEND" = true ]; then
        log_info "Starting frontend service..."
        sudo systemctl start "$FRONTEND_SERVICE_NAME"
        sleep 2
        if systemctl is-active --quiet "$FRONTEND_SERVICE_NAME"; then
            log_success "Frontend service started"
        else
            log_error "Frontend service failed to start!"
            log_error "Check logs with: sudo journalctl -u $FRONTEND_SERVICE_NAME -n 50"
            exit 1
        fi
    fi

    SERVICES_STOPPED=false
    echo ""
}

summary() {
    banner "DEPLOYMENT COMPLETED SUCCESSFULLY!"

    if [ "$DO_BACKEND" = true ]; then
        cd "$BACKEND_DIR"
        log_info "Backend:  $(git rev-parse --short HEAD) - $(git log -1 --pretty=%B | head -n 1)"
    fi
    if [ "$DO_FRONTEND" = true ]; then
        cd "$FRONTEND_DIR"
        log_info "Frontend: $(git rev-parse --short HEAD) - $(git log -1 --pretty=%B | head -n 1)"
    fi
    echo ""

    log_info "Useful commands:"
    log_info "  Backend logs:     tail -f $BACKEND_DIR/logs/SERVER.log"
    log_info "  Backend status:   sudo systemctl status $SERVICE_NAME"
    log_info "  Frontend status:  sudo systemctl status $FRONTEND_SERVICE_NAME"
    log_info "  Restart all:      sudo systemctl restart $SERVICE_NAME $FRONTEND_SERVICE_NAME"
    echo ""

    log_success "Deployment completed at $(date)"
}

main() {
    for arg in "$@"; do
        case $arg in
            --skip-restart)  SKIP_RESTART=true ;;
            --backend-only)  DO_FRONTEND=false ;;
            --frontend-only) DO_BACKEND=false ;;
            --force-install) FORCE_INSTALL=true ;;
            --help|-h)       usage; exit 0 ;;
            *) log_error "Unknown option: $arg"; usage; exit 1 ;;
        esac
    done

    if [ "$DO_BACKEND" = false ] && [ "$DO_FRONTEND" = false ]; then
        log_error "--backend-only and --frontend-only are mutually exclusive"
        exit 1
    fi

    log_info "Starting ImmoLux deployment..."
    echo ""

    preflight
    [ "$DO_BACKEND" = true ] && build_backend
    [ "$DO_FRONTEND" = true ] && build_frontend
    verify_outputs
    restart_services
    summary
}

# The entire script is parsed before main() runs, so a `git pull` that
# updates this very file mid-deploy cannot corrupt the running execution.
main "$@"

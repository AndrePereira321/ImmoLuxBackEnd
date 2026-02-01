#!/bin/bash

################################################################################
# ImmoLux Deployment Script
#
# This script automatically:
# - Pulls latest changes from git
# - Installs/updates dependencies
# - Builds frontend (SvelteKit)
# - Builds backend (Go)
# - Restarts the service
#
# Usage: ./deploy.sh [--skip-restart]
################################################################################

set -e  # Exit on any error

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
PROJECT_ROOT="/opt/immolux"
BACKEND_DIR="$PROJECT_ROOT/backend"
FRONTEND_DIR="$PROJECT_ROOT/frontend"
SERVICE_NAME="immolux"
SKIP_RESTART=false

# Parse arguments
for arg in "$@"; do
    case $arg in
        --skip-restart)
            SKIP_RESTART=true
            shift
            ;;
    esac
done

################################################################################
# Helper Functions
################################################################################

log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

check_command() {
    if ! command -v $1 &> /dev/null; then
        log_error "$1 is not installed. Please install it first."
        exit 1
    fi
}

check_node_version() {
    NODE_VERSION=$(node --version | sed 's/v//' | cut -d. -f1)
    if [ "$NODE_VERSION" -lt 20 ]; then
        log_error "Node.js version 20+ is required. Current version: $(node --version)"
        log_info "Install Node 20+:"
        log_info "  curl -fsSL https://deb.nodesource.com/setup_20.x | sudo -E bash -"
        log_info "  sudo apt install -y nodejs"
        exit 1
    fi
    log_info "Node.js version check passed: v$NODE_VERSION"
}


################################################################################
# Pre-flight Checks
################################################################################

log_info "Starting ImmoLux deployment..."
echo ""

log_info "Running pre-flight checks..."

# Check if running from correct directory
if [ ! -d "$PROJECT_ROOT" ]; then
    log_error "Project root directory not found: $PROJECT_ROOT"
    exit 1
fi

# Check required commands
check_command "git"
check_command "go"
check_command "npm"

# Check Go version
GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
log_info "Go version: $GO_VERSION"

# Check Node version
check_node_version
NODE_VERSION=$(node --version)
log_info "Node version: $NODE_VERSION"

log_success "Pre-flight checks passed!"
echo ""

################################################################################
# Stop Service (if not skipping restart)
################################################################################

if [ "$SKIP_RESTART" = false ]; then
    log_info "Stopping $SERVICE_NAME service..."

    if systemctl is-active --quiet $SERVICE_NAME; then
        sudo systemctl stop $SERVICE_NAME
        log_success "Service stopped"
    else
        log_warning "Service was not running"
    fi
    echo ""
fi

################################################################################
# Update Backend
################################################################################

log_info "========================================="
log_info "  UPDATING BACKEND"
log_info "========================================="
echo ""

cd "$BACKEND_DIR"

# Pull latest changes
log_info "Pulling latest changes from git..."
git fetch origin
BACKEND_BRANCH=$(git rev-parse --abbrev-ref HEAD)
log_info "Current branch: $BACKEND_BRANCH"

BEFORE_COMMIT=$(git rev-parse HEAD)
git pull origin "$BACKEND_BRANCH"
AFTER_COMMIT=$(git rev-parse HEAD)

if [ "$BEFORE_COMMIT" = "$AFTER_COMMIT" ]; then
    log_info "Backend is already up to date"
else
    log_success "Backend updated: $BEFORE_COMMIT -> $AFTER_COMMIT"
fi
echo ""

# Install/update dependencies
log_info "Installing Go dependencies..."
go mod download
go mod tidy
log_success "Go dependencies installed"
echo ""

# Generate Ent code (if schema changed)
if [ -d "internal/database/ent/schema" ]; then
    log_info "Generating Ent client code..."
    go generate ./internal/database/ent
    log_success "Ent code generated"
    echo ""
fi

# Build backend
log_info "Building backend..."
go build -ldflags="-s -w" -o immo-lux-server ./internal

if [ $? -eq 0 ]; then
    log_success "Backend built successfully"

    # Make executable
    chmod +x immo-lux-server

    # Show binary info
    BINARY_SIZE=$(du -h immo-lux-server | cut -f1)
    log_info "Binary size: $BINARY_SIZE"
else
    log_error "Backend build failed!"
    exit 1
fi
echo ""

################################################################################
# Update Frontend
################################################################################

log_info "========================================="
log_info "  UPDATING FRONTEND"
log_info "========================================="
echo ""

cd "$FRONTEND_DIR"

# Pull latest changes
log_info "Pulling latest changes from git..."
git fetch origin
FRONTEND_BRANCH=$(git rev-parse --abbrev-ref HEAD)
log_info "Current branch: $FRONTEND_BRANCH"

BEFORE_COMMIT=$(git rev-parse HEAD)
git pull origin "$FRONTEND_BRANCH"
AFTER_COMMIT=$(git rev-parse HEAD)

if [ "$BEFORE_COMMIT" = "$AFTER_COMMIT" ]; then
    log_info "Frontend is already up to date"
else
    log_success "Frontend updated: $BEFORE_COMMIT -> $AFTER_COMMIT"
fi
echo ""

# Install/update dependencies
log_info "Installing npm dependencies..."
npm install --production=false
log_success "npm dependencies installed"
echo ""

# Build frontend
log_info "Building frontend for production..."
npm run build

if [ $? -eq 0 ]; then
    log_success "Frontend built successfully"

    # Show build folder size
    if [ -d "build" ]; then
        BUILD_SIZE=$(du -sh build | cut -f1)
        log_info "Build folder size: $BUILD_SIZE"

        # Count files in build
        FILE_COUNT=$(find build -type f | wc -l)
        log_info "Files in build: $FILE_COUNT"
    fi
else
    log_error "Frontend build failed!"
    exit 1
fi
echo ""

################################################################################
# Verify Build Outputs
################################################################################

log_info "========================================="
log_info "  VERIFYING BUILD OUTPUTS"
log_info "========================================="
echo ""

# Check backend binary
if [ -f "$BACKEND_DIR/immo-lux-server" ]; then
    log_success "Backend binary exists: $BACKEND_DIR/immo-lux-server"
else
    log_error "Backend binary not found!"
    exit 1
fi

# Check frontend build
if [ -d "$FRONTEND_DIR/build" ]; then
    log_success "Frontend build exists: $FRONTEND_DIR/build"

    # Check for index.html
    if [ -f "$FRONTEND_DIR/build/index.html" ]; then
        log_success "Frontend index.html found"
    else
        log_error "Frontend index.html not found!"
        exit 1
    fi
else
    log_error "Frontend build directory not found!"
    exit 1
fi
echo ""

################################################################################
# Start Service (if not skipping restart)
################################################################################

if [ "$SKIP_RESTART" = false ]; then
    log_info "========================================="
    log_info "  RESTARTING SERVICE"
    log_info "========================================="
    echo ""

    log_info "Starting $SERVICE_NAME service..."
    sudo systemctl start $SERVICE_NAME

    # Wait a moment for service to start
    sleep 2

    # Check if service started successfully
    if systemctl is-active --quiet $SERVICE_NAME; then
        log_success "Service started successfully"

        # Show service status
        sudo systemctl status $SERVICE_NAME --no-pager -l
    else
        log_error "Service failed to start!"
        log_error "Check logs with: sudo journalctl -u $SERVICE_NAME -n 50"
        exit 1
    fi
    echo ""
else
    log_warning "Skipping service restart (--skip-restart flag)"
    log_info "To start manually: sudo systemctl start $SERVICE_NAME"
    echo ""
fi

################################################################################
# Summary
################################################################################

log_info "========================================="
log_success "  DEPLOYMENT COMPLETED SUCCESSFULLY!"
log_info "========================================="
echo ""

# Show versions/commits
cd "$BACKEND_DIR"
BACKEND_COMMIT=$(git rev-parse --short HEAD)
BACKEND_MSG=$(git log -1 --pretty=%B | head -n 1)

cd "$FRONTEND_DIR"
FRONTEND_COMMIT=$(git rev-parse --short HEAD)
FRONTEND_MSG=$(git log -1 --pretty=%B | head -n 1)

log_info "Backend:  $BACKEND_COMMIT - $BACKEND_MSG"
log_info "Frontend: $FRONTEND_COMMIT - $FRONTEND_MSG"
echo ""

log_info "Useful commands:"
log_info "  View logs:        tail -f $BACKEND_DIR/logs/SERVER.log"
log_info "  Service status:   sudo systemctl status $SERVICE_NAME"
log_info "  Restart service:  sudo systemctl restart $SERVICE_NAME"
log_info "  Stop service:     sudo systemctl stop $SERVICE_NAME"
echo ""

log_success "Deployment completed at $(date)"
#!/bin/bash

# Pipeliner Deployment Script
# Automates Docker deployment process

set -e

echo "🚀 Pipeliner Docker Deployment"
echo "=============================="
echo ""

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Functions
print_success() {
    echo -e "${GREEN}✓${NC} $1"
}

print_error() {
    echo -e "${RED}✗${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}⚠${NC} $1"
}

# Check prerequisites
echo "Checking prerequisites..."

if ! command -v docker &> /dev/null; then
    print_error "Docker is not installed"
    exit 1
fi
print_success "Docker found"

if ! command -v docker-compose &> /dev/null; then
    print_error "Docker Compose is not installed"
    exit 1
fi
print_success "Docker Compose found"

# Check if .env exists
if [ ! -f .env ]; then
    print_warning ".env file not found, creating from template..."
    cp .env.example .env
    print_warning "Please edit .env file and update the database password!"
    echo ""
    read -p "Press Enter to edit .env now or Ctrl+C to exit..."
    ${EDITOR:-nano} .env
fi
print_success ".env file exists"

# Check environment selection
echo ""
echo "Select deployment environment:"
echo "1) Development (docker-compose.yml)"
echo "2) Production (docker-compose.prod.yml)"
read -p "Enter choice [1-2]: " env_choice

case $env_choice in
    1)
        COMPOSE_FILE="docker-compose.yml"
        ENV_TYPE="development"
        ;;
    2)
        COMPOSE_FILE="docker-compose.prod.yml"
        ENV_TYPE="production"
        ;;
    *)
        print_error "Invalid choice"
        exit 1
        ;;
esac

echo ""
print_success "Selected $ENV_TYPE environment"

# Ask about Adminer
PROFILE_FLAGS=""
if [ "$ENV_TYPE" = "development" ]; then
    echo ""
    read -p "Start Adminer (database UI)? [y/N]: " start_adminer
    if [[ $start_adminer =~ ^[Yy]$ ]]; then
        PROFILE_FLAGS="--profile tools"
        print_success "Adminer will be started on port 8080"
    fi
fi

# Ask about nginx proxy for production
if [ "$ENV_TYPE" = "production" ]; then
    echo ""
    read -p "Start Nginx reverse proxy? [y/N]: " start_nginx
    if [[ $start_nginx =~ ^[Yy]$ ]]; then
        PROFILE_FLAGS="--profile proxy"
        
        # Check if nginx config exists
        if [ ! -f nginx/nginx.conf ]; then
            print_warning "Nginx config not found. You'll need to create nginx/nginx.conf"
            read -p "Continue anyway? [y/N]: " continue_deploy
            if [[ ! $continue_deploy =~ ^[Yy]$ ]]; then
                exit 1
            fi
        fi
    fi
fi

# Build or pull
echo ""
echo "Select action:"
echo "1) Build from source (recommended for first deployment)"
echo "2) Pull pre-built images"
echo "3) Use existing images"
read -p "Enter choice [1-3]: " build_choice

BUILD_FLAGS=""
case $build_choice in
    1)
        BUILD_FLAGS="--build"
        print_success "Will build from source"
        ;;
    2)
        print_success "Will pull images"
        docker-compose -f $COMPOSE_FILE pull
        ;;
    3)
        print_success "Will use existing images"
        ;;
    *)
        print_error "Invalid choice"
        exit 1
        ;;
esac

# Confirm deployment
echo ""
echo "===== Deployment Summary ====="
echo "Environment: $ENV_TYPE"
echo "Compose file: $COMPOSE_FILE"
echo "Profile flags: ${PROFILE_FLAGS:-none}"
echo "Build flags: ${BUILD_FLAGS:-none}"
echo "=============================="
echo ""
read -p "Proceed with deployment? [y/N]: " confirm

if [[ ! $confirm =~ ^[Yy]$ ]]; then
    print_warning "Deployment cancelled"
    exit 0
fi

# Create necessary directories
echo ""
echo "Creating directories..."
mkdir -p scans logs backups
print_success "Directories created"

# Stop existing containers
echo ""
echo "Stopping existing containers..."
docker-compose -f $COMPOSE_FILE down 2>/dev/null || true
print_success "Stopped existing containers"

# Start services
echo ""
echo "Starting services..."
docker-compose -f $COMPOSE_FILE $PROFILE_FLAGS up -d $BUILD_FLAGS

# Wait for services to be healthy
echo ""
echo "Waiting for services to be healthy..."
sleep 5

# Check if services are running
if docker-compose -f $COMPOSE_FILE ps | grep -q "Up"; then
    print_success "Services started successfully!"
else
    print_error "Failed to start services"
    echo ""
    echo "Checking logs:"
    docker-compose -f $COMPOSE_FILE logs --tail=50
    exit 1
fi

# Display service URLs
echo ""
echo "===== Deployment Complete ====="
echo ""
print_success "Pipeliner is running!"
echo ""
echo "Access points:"
echo "  📱 Application: http://localhost:${APP_PORT:-3000}"

if [[ $start_adminer =~ ^[Yy]$ ]]; then
    echo "  🗄️  Adminer: http://localhost:${ADMINER_PORT:-8080}"
fi

if [[ $start_nginx =~ ^[Yy]$ ]]; then
    echo "  🌐 Nginx: http://localhost:80"
fi

echo ""
echo "Useful commands:"
echo "  View logs:     docker-compose -f $COMPOSE_FILE logs -f"
echo "  Stop services: docker-compose -f $COMPOSE_FILE down"
echo "  Restart:       docker-compose -f $COMPOSE_FILE restart"
echo "  Status:        docker-compose -f $COMPOSE_FILE ps"
echo ""
echo "For more information, see DOCKER_DEPLOYMENT.md"
echo "================================"

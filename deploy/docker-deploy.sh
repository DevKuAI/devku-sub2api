#!/bin/bash
# =============================================================================
# Sub2API Docker Deployment Preparation Script
# =============================================================================
# This script prepares deployment files for Sub2API:
#   - Downloads docker-compose.local.yml and .env.example
#   - Generates secure secrets (JWT_SECRET, DESKTOP_JWT_SECRET,
#     TOTP_ENCRYPTION_KEY, POSTGRES_PASSWORD)
#   - Creates necessary data directories
#
# After running this script, you can start services with:
#   docker-compose up -d
# =============================================================================

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# GitHub raw content base URL
GITHUB_RAW_URL="https://raw.githubusercontent.com/DevKuAI/devku-sub2api/main/deploy"

# Print colored message
print_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Generate random secret
generate_secret() {
    openssl rand -hex 32
}

# Generate a standard base64 secret accepted by Desktop JWT validation
generate_desktop_jwt_secret() {
    openssl rand -base64 32
}

# Check if command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Replace one dotenv value without treating base64 slashes as sed delimiters
replace_env_value() {
    local key="$1"
    local value="$2"

    if sed --version >/dev/null 2>&1; then
        sed -i "s|^${key}=.*|${key}=${value}|" .env
    else
        sed -i '' "s|^${key}=.*|${key}=${value}|" .env
    fi
}

# Keep the downloaded Compose file and environment template in sync
validate_desktop_configuration() {
    local variable
    local compose_count
    local template_count
    local has_errors=false
    local desktop_variables=(
        DESKTOP_ENABLED
        DESKTOP_JWT_SECRET
        DESKTOP_PUBLIC_GATEWAY_BASE_URL
        DESKTOP_ACCESS_TOKEN_TTL_MINUTES
        DESKTOP_REFRESH_FAMILY_TTL_DAYS
        DESKTOP_LOOKUP_IP_PER_MINUTE
        DESKTOP_LOGIN_IP_PER_MINUTE
        DESKTOP_LOGIN_ORGANIZATION_PER_MINUTE
        DESKTOP_LOGIN_PHONE_FAILURE_LIMIT
        DESKTOP_LOGIN_PHONE_FREEZE_MINUTES
    )

    for variable in "${desktop_variables[@]}"; do
        compose_count=$(grep -Ec "^[[:space:]]*-[[:space:]]*${variable}=" docker-compose.yml || true)
        template_count=$(grep -Ec "^${variable}=" .env.example || true)

        if [ "$compose_count" -ne 1 ]; then
            print_error "docker-compose.yml must map ${variable} exactly once."
            has_errors=true
        fi
        if [ "$template_count" -ne 1 ]; then
            print_error ".env.example must define ${variable} exactly once."
            has_errors=true
        fi
    done

    if [ "$has_errors" = true ]; then
        return 1
    fi
}

# Main installation function
main() {
    echo ""
    echo "=========================================="
    echo "  Sub2API Deployment Preparation"
    echo "=========================================="
    echo ""

    # Check if openssl is available
    if ! command_exists openssl; then
        print_error "openssl is not installed. Please install openssl first."
        exit 1
    fi

    # Check if deployment already exists
    if [ -f "docker-compose.yml" ] && [ -f ".env" ]; then
        print_warning "Deployment files already exist in current directory."
        read -p "Overwrite existing files? (y/N): " -r
        echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            print_info "Cancelled."
            exit 0
        fi
    fi

    # Download docker-compose.local.yml and save as docker-compose.yml
    print_info "Downloading docker-compose.yml..."
    if command_exists curl; then
        curl -sSL "${GITHUB_RAW_URL}/docker-compose.local.yml" -o docker-compose.yml
    elif command_exists wget; then
        wget -q "${GITHUB_RAW_URL}/docker-compose.local.yml" -O docker-compose.yml
    else
        print_error "Neither curl nor wget is installed. Please install one of them."
        exit 1
    fi
    print_success "Downloaded docker-compose.yml"

    # Download .env.example
    print_info "Downloading .env.example..."
    if command_exists curl; then
        curl -sSL "${GITHUB_RAW_URL}/.env.example" -o .env.example
    else
        wget -q "${GITHUB_RAW_URL}/.env.example" -O .env.example
    fi
    print_success "Downloaded .env.example"

    print_info "Validating Desktop deployment configuration..."
    validate_desktop_configuration
    print_success "Desktop deployment configuration is valid"

    # Generate .env file with auto-generated secrets
    print_info "Generating secure secrets..."
    echo ""

    # Generate secrets
    JWT_SECRET=$(generate_secret)
    DESKTOP_JWT_SECRET=$(generate_desktop_jwt_secret)
    TOTP_ENCRYPTION_KEY=$(generate_secret)
    POSTGRES_PASSWORD=$(generate_secret)

    # Create .env from .env.example
    cp .env.example .env

    # Update .env with generated secrets (cross-platform compatible)
    replace_env_value JWT_SECRET "$JWT_SECRET"
    replace_env_value DESKTOP_JWT_SECRET "$DESKTOP_JWT_SECRET"
    replace_env_value TOTP_ENCRYPTION_KEY "$TOTP_ENCRYPTION_KEY"
    replace_env_value POSTGRES_PASSWORD "$POSTGRES_PASSWORD"

    # Create data directories
    print_info "Creating data directories..."
    mkdir -p data postgres_data redis_data
    print_success "Created data directories"

    # Set secure permissions for .env file (readable/writable only by owner)
    chmod 600 .env
    echo ""

    # Display completion message
    echo "=========================================="
    echo "  Preparation Complete!"
    echo "=========================================="
    echo ""
    echo "Generated secure credentials:"
    echo "  POSTGRES_PASSWORD:     ${POSTGRES_PASSWORD}"
    echo "  JWT_SECRET:            ${JWT_SECRET}"
    echo "  DESKTOP_JWT_SECRET:    generated and saved to .env"
    echo "  TOTP_ENCRYPTION_KEY:   ${TOTP_ENCRYPTION_KEY}"
    echo ""
    print_warning "These credentials have been saved to .env file."
    print_warning "Please keep them secure and do not share publicly!"
    echo ""
    echo "Directory structure:"
    echo "  docker-compose.yml        - Docker Compose configuration"
    echo "  .env                      - Environment variables (generated secrets)"
    echo "  .env.example              - Example template (for reference)"
    echo "  data/                     - Application data (will be created on first run)"
    echo "  postgres_data/            - PostgreSQL data"
    echo "  redis_data/               - Redis data"
    echo ""
    echo "Next steps:"
    echo "  1. (Optional) Edit .env to customize configuration"
    echo "  2. Start services:"
    echo "     docker-compose up -d"
    echo ""
    echo "  3. View logs:"
    echo "     docker-compose logs -f sub2api"
    echo ""
    echo "  4. Access Web UI:"
    echo "     http://localhost:8080"
    echo ""
    print_info "If admin password is not set in .env, it will be auto-generated."
    print_info "Check logs for the generated admin password on first startup."
    echo ""
}

# Run main function
main "$@"

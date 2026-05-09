# Contributing to Traverse

Thank you for your interest in contributing to Traverse! This document provides guidelines for contributing.

## Code of Conduct

This project and everyone participating in it is governed by our commitment to:

- Be respectful and inclusive
- Welcome newcomers
- Focus on constructive feedback
- Respect different viewpoints and experiences

## How Can I Contribute?

### Reporting Bugs

Before creating a bug report:

1. Check if the issue already exists
2. Try the latest version to see if it's already fixed
3. Collect information about the bug

**When reporting bugs, include:**

- Traverse version
- Operating system
- Steps to reproduce
- Expected vs actual behavior
- Configuration (redact secrets)
- Logs (redact sensitive info)

### Suggesting Enhancements

Enhancement suggestions are tracked as GitHub issues. Include:

- Clear description of the enhancement
- Use case and motivation
- Possible implementation approach
- Examples if applicable

### Pull Requests

1. Fork the repository
2. Create a branch (`git checkout -b feat/amazing-feature`)
3. Make your changes
4. Run tests
5. Commit your changes (`git commit -m 'feat: add amazing feature'`)
6. Push to your fork
7. Open a Pull Request

## Development Setup

### Prerequisites

- Go 1.23+
- Docker and Docker Compose
- Make

### Setup

```bash
# Clone your fork
git clone https://github.com/YOUR_USERNAME/traverse.git
cd traverse

# Install dependencies
go mod download

# Run tests
make test

# Run locally
make run
```

### Project Structure

```
traverse/
├── cmd/traverse/         # Main application entry point
├── internal/
│   ├── api/             # HTTP handlers
│   ├── auth/            # Authentication
│   ├── config/          # Configuration management
│   ├── server/          # HTTP server and middleware
│   ├── storage/         # Database/storage layer
│   └── audit/           # Audit logging
├── spec/                # Interface definitions
├── examples/            # Deployment examples
├── docs/                # Documentation
└── tests/               # Integration tests
```

## Coding Standards

### Go Code Style

- Follow [Effective Go](https://golang.org/doc/effective_go)
- Use `gofmt` for formatting
- Use `golint` and `go vet`
- Keep functions focused and small
- Document exported functions and types

### Example

```go
// RequestHandler handles secret access requests.
type RequestHandler struct {
    store   storage.Store
    policies policy.Engine
}

// CreateRequest creates a new access request.
// Returns the created request or an error if validation fails.
func (h *RequestHandler) CreateRequest(ctx context.Context, req *CreateRequestInput) (*Request, error) {
    // Validate input
    if err := validateRequest(req); err != nil {
        return nil, fmt.Errorf("invalid request: %w", err)
    }
    
    // Create request
    request := &Request{
        ID:         generateID(),
        ClientID:   req.ClientID,
        SecretPath: req.SecretPath,
        Status:     StatusPending,
    }
    
    // Save to storage
    if err := h.store.CreateRequest(ctx, request); err != nil {
        return nil, fmt.Errorf("failed to create request: %w", err)
    }
    
    return request, nil
}
```

### Testing

- Write tests for new functionality
- Maintain >80% code coverage
- Use table-driven tests
- Test error cases

```go
func TestCreateRequest(t *testing.T) {
    tests := []struct {
        name    string
        input   *CreateRequestInput
        wantErr bool
    }{
        {
            name: "valid request",
            input: &CreateRequestInput{
                SecretPath: "prod/api-key",
                Reason:     "Testing",
            },
            wantErr: false,
        },
        {
            name: "empty path",
            input: &CreateRequestInput{
                SecretPath: "",
                Reason:     "Testing",
            },
            wantErr: true,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            handler := NewRequestHandler(mockStore)
            _, err := handler.CreateRequest(context.Background(), tt.input)
            if (err != nil) != tt.wantErr {
                t.Errorf("CreateRequest() error = %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}
```

### Documentation

- Update README.md if adding features
- Update relevant docs/ files
- Add examples for new functionality
- Keep API documentation current

## Commit Messages

Use [Conventional Commits](https://www.conventionalcommits.org/):

```
<type>(<scope>): <description>

[optional body]

[optional footer]
```

**Types:**
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation only
- `style`: Formatting, missing semicolons, etc.
- `refactor`: Code change that neither fixes a bug nor adds a feature
- `perf`: Performance improvement
- `test`: Adding or fixing tests
- `chore`: Build process or auxiliary tool changes

**Examples:**

```
feat(auth): add mTLS authentication support

fix(storage): handle PostgreSQL connection timeout

docs(api): update authentication endpoints

refactor(config): simplify configuration loading
```

## Testing

### Running Tests

```bash
# All tests
make test

# With coverage
make test-coverage

# Specific package
go test ./internal/auth/...

# Integration tests
make test-integration
```

### Writing Tests

```bash
# Create test file
touch internal/mypackage/feature_test.go

# Run specific test
go test -v -run TestFeature ./internal/mypackage

# Run with race detector
go test -race ./...
```

## Documentation

### Building Docs

```bash
# Serve docs locally
make docs-serve

# Build static site
make docs-build
```

### Adding Documentation

1. Add to appropriate file in `docs/`
2. Follow existing format and style
3. Include code examples
4. Update table of contents if needed

## Release Process

1. Update version in relevant files
2. Update CHANGELOG.md
3. Create a new release on GitHub
4. Tag with semantic version (e.g., `v1.2.3`)
5. CI will build and publish artifacts

## Security

### Reporting Security Issues

**DO NOT** create public issues for security vulnerabilities.

Instead:
1. Email security@company.com
2. Include detailed description
3. Include steps to reproduce
4. Allow time for response before public disclosure

### Security Best Practices

- Never commit secrets or credentials
- Use environment variables for sensitive data
- Follow OWASP guidelines
- Keep dependencies updated

## Getting Help

- **GitHub Issues**: Bug reports and feature requests
- **GitHub Discussions**: Questions and general discussion
- **Documentation**: Check docs/ directory

## Recognition

Contributors will be recognized in our README.md and release notes.

## License

By contributing, you agree that your contributions will be licensed under the MIT License.

Thank you for contributing to Traverse!

# Contributing to EBS SSO Gateway

Thank you for your interest in contributing to the EBS SSO Gateway project!

## Development Setup

1. **Prerequisites**
   - Go 1.21 or higher
   - Oracle Instant Client (for testing with real Oracle DB)
   - Git

2. **Clone and Build**
   ```bash
   git clone https://github.com/vmuthadi-winfo/ebssso.git
   cd ebssso
   make install-deps
   make build
   ```

3. **Configuration**
   ```bash
   cp config.yaml.template config.yaml
   # Edit config.yaml with your test environment settings
   ```

## Code Structure

```
ebssso/
├── main.go              # Application entry point and server setup
├── config/             # Configuration management
│   └── config.go       # Config loading and validation
├── handlers/           # HTTP handlers
│   └── auth.go         # OIDC authentication logic
├── db/                 # Database layer
│   └── oracle.go       # Oracle DB operations
└── templates/          # HTML templates
    ├── error.html      # Error page
    └── logout.html     # Logout page
```

## Development Workflow

1. **Format Code**
   ```bash
   make fmt
   ```

2. **Run Linter**
   ```bash
   make lint
   ```

3. **Run Tests**
   ```bash
   make test
   ```

4. **Build**
   ```bash
   make build
   ```

5. **Run Locally**
   ```bash
   make run
   # Or
   ./ebssso
   ```

## Testing

### Unit Tests
Run unit tests with:
```bash
make test
```

### Integration Tests
For integration testing with a real Oracle database:
1. Set up a test EBS database
2. Configure `config.yaml` with test database credentials
3. Run the application and test the login flow

### Manual Testing
1. Start the application: `./ebssso`
2. Navigate to `http://localhost:8080/login`
3. Complete the OIDC authentication flow
4. Verify session creation and redirect to EBS

## Code Style

- Follow standard Go formatting (`go fmt`)
- Use meaningful variable and function names
- Add comments for exported functions and complex logic
- Keep functions focused and small
- Handle errors explicitly

## Commit Messages

Use clear, descriptive commit messages:
```
Add user session validation endpoint

- Implement session validation in handlers
- Add database query for session lookup
- Include tests for session validation
```

## Pull Request Process

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/your-feature`)
3. Make your changes
4. Run tests and linting (`make check`)
5. Commit your changes with clear messages
6. Push to your fork
7. Create a Pull Request with a clear description

## Adding New Features

When adding new features:
1. Update relevant documentation (README.md, code comments)
2. Add appropriate tests
3. Ensure backward compatibility
4. Update configuration template if needed
5. Consider security implications

## Security

- Never commit sensitive credentials
- Use environment variables or config files for secrets
- Follow OWASP best practices
- Report security vulnerabilities privately

## Questions?

If you have questions or need help:
1. Check existing issues
2. Create a new issue with your question
3. Provide context and examples

Thank you for contributing!

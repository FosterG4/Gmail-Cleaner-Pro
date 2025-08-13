# Security Policy

## Supported Versions

We actively support the following versions of Gmail Cleaner Pro with security updates:

| Version | Supported          |
| ------- | ------------------ |
| 1.0.x   | :white_check_mark: |
| < 1.0   | :x:                |

## Security Features

### Authentication & Authorization
- **OAuth2 Integration**: Uses Google's official OAuth2 flow for secure authentication
- **No Password Storage**: Application never stores user passwords
- **Minimal Permissions**: Requests only necessary Gmail API scopes
- **Secure Token Handling**: Access tokens are handled securely and not logged

### Data Privacy
- **No Data Storage**: Application does not store user emails or personal data
- **Local Processing**: All email processing happens locally on your machine
- **Audit Trails**: Comprehensive logging for security monitoring (without sensitive data)
- **HTTPS Enforcement**: All external API calls use HTTPS

### Application Security
- **Input Validation**: All user inputs are validated and sanitized
- **CORS Protection**: Cross-Origin Resource Sharing policies implemented
- **Rate Limiting**: Built-in rate limiting to prevent abuse
- **Error Handling**: Secure error handling that doesn't expose sensitive information

## Reporting a Vulnerability

### How to Report

If you discover a security vulnerability in Gmail Cleaner Pro, please report it responsibly:

1. **GitHub Issues**: Report security vulnerabilities at https://github.com/FosterG4/Gmail-Cleaner-Pro/issues
2. **Include**:
   - Description of the vulnerability
   - Steps to reproduce
   - Potential impact
   - Suggested fix (if any)



### Vulnerability Disclosure Policy

- **Coordinated Disclosure**: We follow responsible disclosure practices
- **Public Disclosure**: Vulnerabilities will be publicly disclosed after fixes are available
- **Credit**: Security researchers will be credited (unless they prefer anonymity)

## Security Best Practices for Users

### Setup Security
1. **Environment Variables**: Never commit `.env` files to version control
2. **Google Cloud Console**: Use least-privilege principles when setting up OAuth2
3. **Network Security**: Run the application in a secure network environment

### Operational Security
1. **Regular Updates**: Keep the application updated to the latest version
2. **Monitor Logs**: Review application logs for suspicious activity
3. **Access Control**: Limit access to the application to authorized users only
4. **Token Management**: Regularly rotate OAuth2 credentials

### Development Security
1. **Code Review**: All code changes should be reviewed for security implications
2. **Dependency Updates**: Keep dependencies updated to patch known vulnerabilities
3. **Static Analysis**: Use security scanning tools during development

## Known Security Considerations

### Current Limitations
- **Local Storage**: Access tokens are stored in browser localStorage (development convenience)
- **HTTP Development**: Development server runs on HTTP (production should use HTTPS)
- **Single User**: Currently designed for single-user deployment

### Planned Improvements
- **Secure Session Management**: Server-side session storage with HTTP-only cookies
- **HTTPS by Default**: TLS/SSL support for production deployments
- **Multi-User Support**: User isolation and access controls
- **Token Refresh**: Automatic token refresh mechanisms

## Security Contact

For security-related questions or concerns:
- **GitHub Issues**: Use the "Security" label for public security discussions
- **Private Reports**: Contact maintainers directly for sensitive security issues

## Compliance

This application is designed to comply with:
- **Google API Terms of Service**
- **OAuth2 Security Best Practices**
- **General Security Standards** for web applications

---

**Note**: This security policy is subject to updates as the project evolves. Users are encouraged to review this document regularly and report any security concerns promptly.
# SSL Certificate Notifier

A Go-based application to monitor SSL certificates for multiple websites. It notifies users via various channels (e.g., Telegram, Email) if certificates are expired or nearing expiry, based on a configurable `config.yaml` file. The application supports retry mechanisms with exponential backoff for failed checks and is designed for easy deployment with Docker Compose.

## Features

-   **Certificate Monitoring:** Checks SSL certificates for configured websites.
-   **Configurable Expiry Warnings:** Notifies if the number of days left for certificate expiry is less than or equal to a configured threshold.
-   **Multiple Notification Channels:** Extensible notifier system (currently supports Telegram, Email, HTTP webhooks, and logging).
-   **Retry Mechanism:** Configurable retry attempts with exponential backoff for failed checks and alert notifications.
-   **Offline Alert Queueing:** Failed alert notifications (e.g., Telegram, Email, HTTP) during network outages are queued and automatically delivered when network connectivity recovers.
-   **Scheduled Checks:** Runs checks at a defined interval (e.g., every 24 hours).
-   **Docker Compose Deployment:** Easy to set up and run using `docker-compose`.
-   **Environment Variable Support:** Sensitive credentials can be managed via environment variables.
-   **High Test Coverage:** Comprehensive test suite with dependency injection for testable external service integrations.
-   **Improved Architecture:** Clean, maintainable codebase with proper separation of concerns.

## Getting Started

These instructions will get you a copy of the project up and running on your local machine for development and testing purposes.

### Prerequisites

-   [Docker](https://www.docker.com/get-started) and [Docker Compose](https://docs.docker.com/compose/install/)
-   [Go (Golang)](https://golang.org/doc/install) (for local development and testing)

### Installation and Setup

1.  **Clone the repository:**
    ```bash
    git clone https://github.com/the100rabh/ssl-cert-notifier.git
    cd ssl-cert-notifier
    ```

2.  **Configure the Application:**
    Edit the `config.yaml` file to define your websites, notification channels, and application settings.

    **Important:** This `config.yaml` is the central place to define your notifiers. For sensitive credentials like `bot_token` for Telegram or `username` and `password` for email, you should **hardcode** them directly into your *local* `config.yaml`.

    **Security Note:** Because `config.yaml` will contain sensitive information, it is already added to `.gitignore` to prevent accidental commits to version control. Do NOT remove `config.yaml` from `.gitignore`.

    Refer to `sample.config.yaml` for a complete example of the structure. Replace the placeholder values in your `config.yaml` with your actual credentials.
    ```yaml
    # config.yaml
    settings:
      check_interval: "24h" # How often to run checks (e.g., "1h", "30m")
      check_time: "09:00"    # Specific daily time to run checks (e.g., "09:00", "14:30")
      retry:
        attempts: 3
        initial_delay: "30s" # Initial delay before retrying (e.g., "30s", "1m")
        backoff_factor: 2.0 # Factor by which to multiply the delay for each subsequent retry

    notifiers:
      telegram_personal:
        type: telegram
        # Replace with your actual Telegram Bot Token
        bot_token: "your_telegram_bot_token_here"
        # Replace with your actual Telegram Chat ID
        chat_id: "your_telegram_chat_id_here"

      email_alerts:
        type: email
        host: "smtp.example.com" # Replace with your SMTP host
        port: 587
        # Replace with your SMTP username
        username: "your_smtp_username_here"
        # Replace with your SMTP password
        password: "your_smtp_password_here"
        from: "ssl-notifier@example.com" # Replace with your From address
        to:
          - "alerts@example.com" # Replace with your To address

    websites:
      - url: "google.com:443"
        days_until_expiry: 30 # Notify if days left for expiry is less than or equal to this value
        notifiers:
          - "telegram_personal"
          - "email_alerts"

      - url: "expired.badssl.com:443"
        days_until_expiry: 1
        notifiers:
          - "telegram_personal"

      - url: "wrong.host.badssl.com:443"
        days_until_expiry: 1
        notifiers:
          - "telegram_personal"
        retry: # Override global retry settings for this specific site
          attempts: 5
          initial_delay: "10s"
          backoff_factor: 1.5
    ```

3.  **No Separate Environment Variables File (`.env`):**
    Since sensitive credentials are now directly configured in `config.yaml`, there is no need for a separate `.env` file to manage these specific secrets.

### Obtaining Telegram Details

To use the Telegram notifier, you need a Bot Token and a Chat ID. You will paste these directly into your `config.yaml`:

### Obtaining Email Details

To use the Email notifier, you need SMTP server details, including host, port, username, and password. Here are common settings for popular providers:

#### Gmail

-   **Host:** `smtp.gmail.com`
-   **Port:** `587` (for TLS/STARTTLS)
-   **Username:** Your full Gmail address (e.g., `your_email@gmail.com`)
-   **Password:**
    -   You **cannot** use your regular Gmail password if you have 2-Factor Authentication (2FA) enabled.
    -   You must generate an **App Password**. Go to your [Google Account Security settings](https://myaccount.google.com/security) -> "App passwords" -> Generate a new app password. Use this generated password in your `.env` file.
    -   Ensure "Less secure app access" is turned **off** if you are using App Passwords.

#### Outlook/Office 365

-   **Host:** `smtp.office365.com`
-   **Port:** `587` (for TLS/STARTTLS)
-   **Username:** Your full Outlook/Office 365 email address.
-   **Password:** Your regular email password or an App Password if 2FA is enabled and your organization allows it.

**Important Notes for Email:**

-   **`FROM` Address:** The `from` field in `config.yaml` should generally match your `SMTP_USERNAME`.
-   **`TO` Addresses:** The `to` field can be a list of one or more email addresses.
-   **Security:** Always use App Passwords or dedicated SMTP credentials rather than your primary email password, especially if 2FA is enabled. Ensure your SMTP server supports TLS/STARTTLS encryption.

### Running the Application

#### Using Docker Compose (Recommended for Production)

Build and run the Docker container in detached mode. Ensure your environment variables are set (either in a `.env` file or in your shell):
```bash
docker-compose up --build -d
```
To view the logs:
```bash
docker-compose logs -f
```
To stop the application:
```bash
docker-compose down
```

#### Running Locally (for Development/Testing)

Ensure you have Go installed.
1.  **Install dependencies:**
    ```bash
    go mod tidy
    ```
2.  **Run the application:**
    ```bash
    go run cmd/cert-notifier/main.go
    ```
    Note: When running locally, environment variables for notifiers (`TELEGRAM_BOT_TOKEN`, etc.) need to be set in your shell environment before running the command, e.g., `export TELEGRAM_BOT_TOKEN="your_token"`.

## Architecture Improvements

### Testable Design
The application has been refactored to support better testability using dependency injection patterns:

- **Dependency Injection**: The email notifier now supports dependency injection through the `SendWithDeps` method, allowing for easy mocking of external services during testing.
- **Interface-Based Design**: External service dependencies (SMTP, HTTP clients) are abstracted behind interfaces, making them easily mockable.
- **Separation of Concerns**: Production code uses default implementations while test code can inject mock implementations.

### Enhanced Test Coverage
- **App Package**: 97.1% coverage
- **Checker Package**: 94.4% coverage
- **Config Package**: 90.5% coverage
- **Notifiers Package**: 82.5% coverage
- **Comprehensive Error Path Testing**: All notifier types include tests for various failure scenarios

## Extending Notifiers

The application is designed with an extensible notifier system, making it straightforward to add new notification methods (e.g., Slack, PagerDuty, HTTP endpoints). Here's how:

1.  **Define the Notifier Struct and `New` Function:**
    *   Create a new file in `internal/notifiers/` (e.g., `slack.go`).
    *   Define a struct for your new notifier (e.g., `SlackNotifier`). This struct should hold any necessary configuration (e.g., webhook URL, API token).
    *   Implement a `NewSlackNotifier(cfg config.Notifier) (*SlackNotifier, error)` function. This function should:
        *   Take a `config.Notifier` struct as input.
        *   Validate that all required fields for your notifier (e.g., `WebhookURL` from `cfg.URL`) are present and valid.
        *   Return an initialized `SlackNotifier` instance or an error if validation fails.

2.  **Implement the `notifiers.Notifier` Interface:**
    *   Your new notifier struct must implement the `Send(subject, body string) error` method. This is where the actual logic to send the notification via your chosen service's API will reside.
    *   For testable implementations, consider using dependency injection patterns similar to the email notifier.
    *   Refer to `internal/notifiers/telegram.go`, `internal/notifiers/email.go`, or `internal/notifiers/http.go` for examples. The `HTTPNotifier` specifically demonstrates sending to a generic webhook endpoint.

3.  **Update the Notifier Factory:**
    *   Open `internal/notifiers/notifiers.go`.
    *   Add a new `case` to the `switch` statement in the `GetNotifier` function to recognize your new notifier's `type` from the `config.yaml` (e.g., `"slack"`).
    *   Call your `NewSlackNotifier` function within this case.

4.  **Update `config.Notifier` Struct:**
    *   If your new notifier requires new configuration fields not already present in the `config.Notifier` struct (in `internal/config/config.go`), add them there with appropriate `yaml:"field_name,omitempty"` tags. For example, the `HTTPNotifier` reuses the `URL` field.

### Testing New Notifiers

Robust testing is crucial for new notifiers:

1.  **Unit Tests (`internal/notifiers/notifiers_test.go`):**
    *   Add tests to `TestGetNotifier_MissingConfigFields` to ensure your `NewNotifier` function correctly validates and returns errors when required configuration fields are missing.
    *   Add dedicated unit tests for your notifier's `Send` method. This will typically involve **mocking** the external API call (e.g., `net/http/httptest` for HTTP APIs or dependency injection for email). Ensure the `Send` method:
        *   Sends the correct payload.
        *   Handles successful API responses.
        *   Handles API errors (e.g., rate limiting, authentication failures).
        *   Handles network errors.

2.  **End-to-End (E2E) Tests (`test/e2e/e2e_test.go`):**
    *   Consider adding an E2E test case that uses your new notifier type. This often involves setting up a mock HTTP server (like `mockNotificationReceiver` in `test/e2e/e2e_test.go`) that simulates the external service and verifies the notification payload it receives from your application. This confirms the entire flow, from config loading to actual notification dispatch.

## Running Tests

To run all unit tests for the project:
```bash
go test ./...
```
To run tests for a specific package:
```bash
go test ./internal/config
go test ./internal/checker
```
To run tests with coverage:
```bash
go test ./... -cover
```

## Development Best Practices

### Code Quality
- All code passes `go vet` and `go fmt` checks
- Consistent error handling and logging
- Proper separation of concerns
- Dependency injection for testable external service integrations

### Testing Strategy
- Comprehensive unit tests for all business logic
- Mock implementations for external service dependencies
- Error path testing for all failure scenarios
- Integration tests to verify end-to-end functionality

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

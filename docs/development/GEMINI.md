# Gemini Go CLI Project Guidelines

This document outlines the conventions and best practices to be followed in this project.

## Project Structure

We adhere to the standard Go project layout to maintain a clean and scalable architecture.

-   **/cmd**: Contains the main application entry point. For a CLI named `my-cli`, the entry point will be in `/cmd/my-cli/main.go`.
-   **/internal**: Houses the core application logic. This code is considered private to the project.
-   **/pkg**: Contains reusable code that can be shared with other projects.

## Dependencies

We use Go modules for dependency management. To add a new dependency, use `go get`. For CLI argument parsing and command structure, we prefer to use the [Cobra](https://github.com/spf13/cobra) library. For configuration management, [Viper](https://github.com/spf13/viper) is recommended and integrates well with Cobra.

## Testing

-   **Unit Tests**: All new functionality should be accompanied by unit tests. Place test files next to the code they are testing (e.g., `main_test.go` for `main.go`).
-   **Integration Tests**: For end-to-end testing of CLI commands, we use the `go-testscript` package. This allows us to test the compiled binary in a controlled environment.

## Building and Releasing

-   **Cross-Compilation**: We use `GOOS` and `GOARCH` to build for different platforms.
-   **Automation**: [GoReleaser](https://goreleaser.com/) is used to automate the release process, including building, archiving, and publishing.
-   **Versioning**: We follow semantic versioning (e.g., `v1.2.3`).

## Configuration

Configuration should be handled in a flexible manner, with the following order of precedence:

1.  Command-line flags
2.  Environment variables
3.  Configuration file (e.g., `config.yaml`)

A struct in the `/internal/config` package should be used to hold the application's configuration in a type-safe way.

## Error Handling

-   **Contextual Errors**: Wrap errors with `fmt.Errorf` and the `%w` verb to provide context.
-   **User-Friendly Messages**: Present clear and concise error messages to the user. Avoid technical jargon in user-facing errors.
-   **No Panics**: Do not use `panic` for expected errors. Return an error and handle it gracefully.

## UI Development

For building Terminal User Interfaces (TUIs), we use the Bubble Tea framework. All UI development should follow the principles outlined in `UI_GUIDE.md`.

When a development mistake reveals a misunderstanding of the framework or a better way to do something, a note should be added to `UI_GUIDE.md` to document this lesson for the future.

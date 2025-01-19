# Nextgen Go CLI

**Nextgen Go CLI** is a cross-platform CLI tool built in Go that provides a TUI (Text User Interface) for quickly accessing recently used commands and toggling between offline and online (“logged in”) modes. It leverages [Bubble Tea](https://github.com/charmbracelet/bubbletea) and [Lip Gloss](https://github.com/charmbracelet/lipgloss) for a stylish, terminal-based user experience.

## Features

- **Bubble Tea TUI** for user-friendly navigation:
  - Pick between “Login” or “Stay Offline” on startup.
  - Access recent commands in a horizontal, multi-row menu.
  - Toggle an “Online/Offline” mode mid-session.
- **Cross-Platform Binaries** included for macOS, Linux, and Windows.
- **Minimal dependencies** – only [Bubble Tea](https://github.com/charmbracelet/bubbletea) and [Lip Gloss](https://github.com/charmbracelet/lipgloss).

## Installation

### Prerequisites

- Go 1.21 or later
- Node.js (for npm scripts)

### Steps

1. Clone the repository:
   ```bash
   git clone https://github.com/guerrilla-interactive/nextgen-go-cli.git
   cd nextgen-go-cli
   ```

2. Build the binaries:
   ```bash
   npm run build-binaries
   ```

3. Run the CLI:
   ```bash
   ./dist/nextgen-go-cli_<platform>_<arch>/nextgen-go-cli
   ```

## Usage

- Start the CLI and choose between "Login" or "Stay Offline".
- Navigate through recent commands using the TUI.
- Toggle between online and offline modes as needed.

## Configuration

- Environment variables can be set in the `.env` file for custom configurations.

## Development

1. Ensure Go and Node.js are installed.
2. Clone the repository and navigate to the project directory.
3. Install dependencies and build the project using the provided npm scripts.

## Contributing

Contributions are welcome! Please open an issue or submit a pull request on GitHub.


## Contact

For questions or support, please open an issue on [GitHub](https://github.com/guerrilla-interactive/nextgen-go-cli/issues).

---

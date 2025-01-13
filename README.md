# Nextgen Go CLI

**Nextgen Go CLI** is a cross-platform CLI tool built in Go that provides a TUI (Text User Interface) for quickly accessing recently used commands and toggling between offline and online (“logged in”) modes. It leverages [Bubble Tea](https://github.com/charmbracelet/bubbletea) and [Lip Gloss](https://github.com/charmbracelet/lipgloss) for a stylish, terminal-based user experience.

---

## Table of Contents

1. [Features](#features)  
2. [Installation](#installation)  
3. [Usage](#usage)  
4. [TUI Overview](#tui-overview)  
5. [Development & Build](#development--build)  
6. [Contributing](#contributing)  
7. [License](#license)

---

## Features

- **Bubble Tea TUI** for user-friendly navigation:
  - Pick between “Login” or “Stay Offline” on startup.
  - Access recent commands in a horizontal, multi-row menu.
  - Toggle an “Online/Offline” mode mid-session.
- **Cross-Platform Binaries** included for macOS, Linux, and Windows.
- **Minimal dependencies** – only [Bubble Tea](https://github.com/charmbracelet/bubbletea) and [Lip Gloss](https://github.com/charmbracelet/lipgloss).

---

## Installation

> **Note**: Requires [Node.js](https://nodejs.org/) and [npm](https://www.npmjs.com/) to install from the registry.

1. **Install Globally (recommended):**
   ```bash
   npm install -g nextgen-go-cli

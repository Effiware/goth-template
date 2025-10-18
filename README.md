# 🚀 GOTH Stack Complete Setup

[![Go Version](https://img.shields.io/badge/Go-1.25.0-00ADD8?style=flat-square&logo=go)](https://go.dev/doc/go1.25)
[![Templ](https://img.shields.io/badge/Templ-0.3.943-red?style=flat-square)](https://templ.guide)
[![Tailwind CSS](https://img.shields.io/badge/Tailwind_CSS-3.4.11-38B2AC?style=flat-square&logo=tailwind-css)](https://tailwindcss.com)
[![HTMX](https://img.shields.io/badge/HTMX-2.0.7-purple?style=flat-square)](https://htmx.org)
[![Alpine.js](https://img.shields.io/badge/Alpine.js-3.15.0-2D3441?style=flat-square)](https://alpinejs.dev)

A modern, fully configured starter template for building fast, type-safe web applications using Go, HTMX, Tailwind CSS, Alpine.js, and Templ. This stack provides an incredible developer experience with type safety from your database all the way to your HTML.

## ✨ Features

- **📜 Type-Safe Templates**: Using [Templ](https://templ.guide) for compile-time checked templates
- **🔥 Hot Reload**: Using [Air](https://github.com/cosmtrek/air) for instant feedback during development
- **🎨 HTMX + Tailwind**: [Modern, interactive UIs](https://htmx.org) without complex client-side JavaScript
- **🛠️ Modern JS Utilities**: [Alpine.js](https://alpinejs.dev) for lightweight interactivity
- **📱 Responsive Design**: [Mobile-first approach](https://tailwindcss.com) with Tailwind CSS

## 🚀 Quick Start

### Prerequisites

At least the following major versions are required (when installing on your local machine):

- Go v1.25
- npm v11.4
- node v24.4
- Air v1.63.0
- Templ CLI 0.3.943
- GNU Make 3.81 (recommended, but optional)

Or just use Docker for everything:

- Docker 28.1 (recommended, but optional)

### Usage

1. Start the server using either local setup or Docker (see below).
2. Open your browser and navigate to `http://localhost:<app-port>` (default is 8080).
3. Play with HDA (interactions, dynamic content loading, etc.) and see changes instantly with hot reload!
4. Play with API 
   - get clicks count: `curl http://localhost:8080/api/v1/clicks`
   - increment clicks count: `curl -X POST http://localhost:8080/api/v1/clicks/increment`

---

## Development Setup

### Option 1: Local Machine

1. **Install dependencies**
   ```bash
   make prep
   ```

2. **Build Tailwind and Go**
   ```bash
   make build
   ```

3. **Run the application** with Hot Reload using Air 
   ```bash
    make air
    ```

### Option 2: Docker (Recommended)

Either do `make prep` (will also install Go/Node dependencies on the host machine) or copy `.env.example` to `.env` and modify as needed.

1. **Build the Docker image**
   ```bash
   make docker-build
   ```

2. **Run the Docker container**
   ```bash
   make docker-up
   ```

3. **Stop the Docker container**
   ```bash
   make docker-down
   ```

---

## Access the Application

In both cases, open your browser and navigate to:

```bash
http://localhost:<local-port>
```

Note: The `<app-port>` should match the `APP_PORT` in your `.env` file.

---

## Notes

### Port taken
If you get an error that a port is already taken, you can change the port in the `.env` file or kill the process using it:

```bash
sudo lsof -i -P | grep LISTEN | grep :<PORT>
```
Then kill the process using the PID:
```bash
sudo kill -9 <PID>
```

### Remove package
To remove a package, use the following command:
```bash
go mod edit -dropreplace <package>
go mod tidy
```

If you used go install package@latest then to remove:

```bash
go install package@none
go clean -cache -modcache
```

### Hot-reloading explanation

There is an in detail explanation of how the hot reloading works in [this](https://medium.com/ostinato-rigore/go-htmx-templ-tailwind-complete-project-setup-hot-reloading-2ca1ba6c28be) article.

# Patungan App (Split Bill App)

A robust web application designed to manage shared expenses, recurring plans, and payment dues. Built with performance and scalability in mind using Go and modern web technologies.

## 🚀 Features

-   **User Management**: Secure authentication via Firebase.
-   **Plan Management**: Create, edit, and schedule recurring billing plans.
-   **Payment Dues**: Automatically generate payment dues for plan participants.
-   **Payment Integration**: Seamless integration with Midtrans for payment processing.
-   **Dashboard**: Overview of active plans and pending payments.
-   **Background Workers**: Asynq-based workers for processing scheduled tasks.
-   **Responsive UI**: Modern interface with TailwindCSS and mobile-first design.

## 🛠 Tech Stack

**Backend**
-   **Language**: Go (Golang)
-   **Framework**: [Echo](https://echo.labstack.com/)
-   **Template Engine**: [Templ](https://templ.guide/)
-   **Database**: PostgreSQL
-   **ORM**: GORM
-   **Caching**: Redis
-   **Payment Gateway**: Midtrans
-   **Worker Queue**: Asynq (implied by worker service)

**Frontend**
-   **Styling**: [TailwindCSS](https://tailwindcss.com/)
-   **Interactivity**: [HTMX](https://htmx.org/)
-   **Templating**: Templ (Components)

## 📋 Prerequisites

-   [Docker](https://www.docker.com/) and [Docker Compose](https://docs.docker.com/compose/)
-   [Go](https://go.dev/) (for local toolchain)
-   [Air](https://github.com/cosmtrek/air) (optional, for local non-docker dev)

## ⚡️ Installation & Setup

1.  **Clone the repository**
    ```bash
    git clone <repository-url>
    cd patungan_app_echo
    ```

2.  **Environment Configuration**
    Copy the example environment file and configure your credentials.
    ```bash
    cp .env.example .env
    ```
    You will need to provide:
    -   Firebase Service Account (`firebase-service-account.json`)
    -   Midtrans Server/Client Keys
    -   Database & Redis credentials (defaults provided in `docker-compose.yml` work for local dev)

3.  **Firebase Setup**
    -   Place your `firebase-service-account.json` in the root directory.
    -   Ensure `FIREBASE_CREDENTIALS_PATH` in `.env` points to this file.

4.  **Run with Docker Compose**
    Start the entire stack (App, Worker, Postgres, Redis, PgAdmin).
    ```bash
    docker-compose up -d --build
    ```

5.  **Access the Application**
    -   **Web App**: [http://localhost:8080](http://localhost:8080)
    -   **PgAdmin**: [http://localhost:5050](http://localhost:5050)
        -   Email: `admin@admin.com`
        -   Password: `admin`

## 💻 Development

The project uses **Air** for live reloading during development.
-   **App Service**: Configured with `.air.toml`
-   **Worker Service**: Configured with `.air.worker.toml`

To view logs:
```bash
docker-compose logs -f app
```

## 📂 Project Structure

-   `cmd/`: Entry points for the application.
-   `internal/`: Private application code (Handlers, Models, Services).
-   `web/`: Web assets, templates (Templ), and static files.
-   `tmp/`: Temporary build artifacts (ignored by git).

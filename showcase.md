# Patungan App Showcase

This document provides a visual overview and technical description of the Patungan App interfaces.

Built using Go (Echo), Templ, HTMX, and Tailwind CSS, the application offers a server-rendered user experience with dynamic, localized updates. The codebase is organized into `internal/modules` (business services, repositories, scheduled tasks) and `internal/pages` (HTTP handlers and templates per audience: admin, member, public, auth).

---

## Table of Contents
1. [Authentication (Login)](#1-authentication-login)
2. [Dashboard](#2-dashboard)
3. [Plan Management](#3-plan-management)
4. [User Directory](#4-user-directory)
5. [Payment Dues by Plan](#5-payment-dues-by-plan)
6. [Payment Portal](#6-payment-portal)
7. [System Settings](#7-system-settings)

---

## 1. Authentication (Login)

The user entry point for authentication.

![Login Screen](docs/screenshots/login.webp)

### Key Technical Aspects:
- **Firebase Authentication**: Sign-in is handled by Firebase Auth with a single Google sign-in button, backed by the Firebase Admin SDK for token verification on the server.
- **Session Cookies**: On success the backend exchanges the ID token for a secure, HTTP-only session cookie (valid for 5 days).
- **Responsive Layout**: Scales to both desktop and mobile viewports.

---

## 2. Dashboard

The main dashboard providing an overview of active costing groups, outstanding dues, and recent system events.

![Dashboard](docs/screenshots/dashboard.webp)

### Key Technical Aspects:
- **Metrics Grid**: Card elements showing active plans, pending dues, pending amount, and total paid to date.
- **Upcoming Dues**: Table of the nearest subscription payments requiring attention, with one-click *Pay Now* shortcuts.
- **Quick Actions**: Shortcut cards for managing plans, tracking payments, and managing members.

---

## 3. Plan Management

Allows administrators to define and organize cost-sharing plans and billing schedules.

![Plan Management](docs/screenshots/plans.webp)

### Key Technical Aspects:
- **Plan Cards**: Displays the group name, owner, cost, recurrence type (one-time/recurring), participant count with per-member cost, and status badges (Active, Dispatched, Manual).
- **Contextual Operations**: Schedule due generation, edit, or delete plans directly from each card; the schedule dialog loads over the page via HTMX.
- **Filtering and Sorting**: Narrow the list by owner and payment type, with configurable sort field and order.

---

## 4. User Directory

Manages user list, contact details, and account activation states.

![User Directory](docs/screenshots/users.webp)

### Key Technical Aspects:
- **Directory Table**: Lists each user's name, email, phone, and role badge (Admin or Member).
- **User Management**: Create and edit users through dedicated forms; activate, update contact details, or remove accounts.
- **Notification Preferences**: Per-user communication settings (email/WhatsApp) load in an HTMX-driven modal from the row's bell action.

---

## 5. Payment Dues by Plan

Tracks payment cycles, billing items, and payment progress for specific plans.

![Payment Dues By Plan](docs/screenshots/payment-dues-by-plan-fixed.webp)

### Key Technical Aspects:
- **Multiple Views**: Toggle between All Dues, By Plan, By Period, and By User, with plan/user filters and sorting applied via HTMX partial swaps.
- **Per-Fee Actions**: Copy the member's payment link, trigger gateway checkout, re-check gateway status, or manually mark a due as complete.
- **Reminder Engine**: Payment notices are dispatched by the notification module (email via SMTP, WhatsApp via WAHA) through scheduled tasks.

---

## 6. Payment Portal

The portal interface shown to users during checkout.

![Payment Due Portal](docs/screenshots/payment-due-payment.webp)

### Key Technical Aspects:
- **Tokenized Access**: Each due is reachable via a public, unguessable URL (`/p/:uuid`) — no login required for members.
- **Invoice Summary**: Shows the total amount due, payment status, subscription plan, member details, due date, and portion split.
- **Gateway Checkout**: *Pay Now* initiates a Midtrans or Mayar.id session, with a *Check Status* action for polling; payment webhooks verify transactions and update the database automatically.

---

## 7. System Settings

Administrative settings for system-wide variables and credentials.

![System Settings](docs/screenshots/settings.webp)

### Key Technical Aspects:
- **Active Gateway**: A global selector decides which payment gateway (Midtrans or Mayar.id) handles new payment initiations by default.
- **Gateway Keys**: Configure Midtrans merchant/server/client keys and the Mayar.id API key, each with a production-mode toggle; secrets are masked in the UI.
- **Persistence**: Settings are stored server-side and applied application-wide on save.

---

*This document was generated with the assistance of Gemini.*


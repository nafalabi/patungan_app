# Database Schema Documentation

This document describes the database schema based on the GORM models located in `internal/models`.

## Overview

The application uses the following tables:
- `users`: Stores user information.
- `plans`: Stores plan details.
- `payment_dues`: Tracks payment schedules for plans.
- `user_payments`: Records payments made by users.
- `refunds`: Records refunds issued to users.

---

## Tables

### 1. Users (`users`)
Represents a user in the system.

| Column | Type | Description | Constraints |
|--------|------|-------------|-------------|
| `id` | `uint` | Primary Key | Auto Increment |
| `created_at` | `time.Time` | Record creation timestamp | |
| `updated_at` | `time.Time` | Record update timestamp | |
| `deleted_at` | `time.Time` | Soft delete timestamp | Index |
| `name` | `string` | User's full name | |
| `phone` | `string` | User's phone number | |
| `email` | `string` | User's email address | Unique |

### 2. Plans (`plans`)
Represents a subscription or payment plan.

| Column | Type | Description | Constraints |
|--------|------|-------------|-------------|
| `id` | `uint` | Primary Key | Auto Increment |
| `created_at` | `time.Time` | Record creation timestamp | |
| `updated_at` | `time.Time` | Record update timestamp | |
| `deleted_at` | `time.Time` | Soft delete timestamp | Index |
| `name` | `string` | Name of the plan | |
| `plan_start_date` | `time.Time` | Start date of the plan | |
| `total_price` | `float64` | Total price of the plan | |
| `pay_interval` | `string` | Payment interval (e.g., monthly) | |
| `is_active` | `bool` | Whether the plan is active | |
| `allow_invitation_after_pay` | `bool` | Allow invites after payment | |

**Relationships:**
- **Many-to-Many** with `users` via `plan_user` join table.

### 3. Payment Dues (`payment_dues`)
Represents a scheduled payment period for a plan.

| Column | Type | Description | Constraints |
|--------|------|-------------|-------------|
| `id` | `uint` | Primary Key | Auto Increment |
| `created_at` | `time.Time` | Record creation timestamp | |
| `updated_at` | `time.Time` | Record update timestamp | |
| `deleted_at` | `time.Time` | Soft delete timestamp | Index |
| `plan_id` | `uint` | Foreign Key to `plans` | |
| `payment_period` | `string` | Period description | |
| `payment_status` | `string` | Status of the payment | |

**Relationships:**
- **Belongs To** `Plan`.

### 4. User Payments (`user_payments`)
Records a payment made by a specific user for a specific due.

| Column | Type | Description | Constraints |
|--------|------|-------------|-------------|
| `id` | `uint` | Primary Key | Auto Increment |
| `created_at` | `time.Time` | Record creation timestamp | |
| `updated_at` | `time.Time` | Record update timestamp | |
| `deleted_at` | `time.Time` | Soft delete timestamp | Index |
| `plan_id` | `uint` | Foreign Key to `plans` | |
| `payment_due_id` | `uint` | Foreign Key to `payment_dues` | |
| `user_id` | `uint` | Foreign Key to `users` | |
| `total_pay` | `float64` | Amount paid | |
| `channel_payment` | `string` | Payment channel used | |
| `payment_date` | `time.Time` | Date of payment | |

**Relationships:**
- **Belongs To** `Plan`.
- **Belongs To** `PaymentDue`.
- **Belongs To** `User`.

### 5. Refunds (`refunds`)
Records a refund issued to a user.

| Column | Type | Description | Constraints |
|--------|------|-------------|-------------|
| `id` | `uint` | Primary Key | Auto Increment |
| `created_at` | `time.Time` | Record creation timestamp | |
| `updated_at` | `time.Time` | Record update timestamp | |
| `deleted_at` | `time.Time` | Soft delete timestamp | Index |
| `plan_id` | `uint` | Foreign Key to `plans` | |
| `payment_due_id` | `uint` | Foreign Key to `payment_dues` | |
| `user_payment_id` | `uint` | Foreign Key to `user_payments` | |
| `user_id` | `uint` | Foreign Key to `users` | |
| `total_refund` | `float64` | Amount refunded | |
| `channel_payment` | `string` | Payment channel for refund | |
| `refund_date` | `time.Time` | Date of refund | |

**Relationships:**
- **Belongs To** `Plan`.
- **Belongs To** `PaymentDue`.
- **Belongs To** `UserPayment`.
- **Belongs To** `User`.

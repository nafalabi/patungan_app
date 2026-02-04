# Firebase Auth Implementation Guide (Go)

This guide describes how to implement a Firebase Authentication flow with a custom login page in a Go application. The implementation favors a session-cookie-based approach for better security and server-side control, rather than relying solely on client-side tokens.

## Prerequisites

1.  **Google Cloud Project**: A project with Firebase Authentication enabled.
2.  **Service Account**: A JSON service account key file for the backend.
3.  **Client Config**: API Key, Auth Domain, Project ID, etc., for the frontend.

## 1. Frontend Implementation

The frontend is responsible for initiating the Google Sign-In flow and obtaining an ID token.

### HTML & JavaScript Logic

The login page should load the Firebase SDK and handle the sign-in process.

1.  **Load Firebase SDKs**: Import `firebase-app` and `firebase-auth`.
2.  **Initialize App**: Use the public configuration (API Key, etc.).
3.  **Sign In**: Use `signInWithPopup` with `GoogleAuthProvider`.
4.  **Get Token**: Retrieve the ID Token from the signed-in user (`user.getIdToken()`).
5.  **Send to Backend**: POST the ID Token to your Go server (e.g., via `Authorization` header).
6.  **Handle Response**: On success, redirect to the dashboard.

**Example Implementation (Vanilla JS):**

```html
<script type="module">
  import { initializeApp } from "https://www.gstatic.com/firebasejs/10.7.1/firebase-app.js";
  import { getAuth, GoogleAuthProvider, signInWithPopup } from "https://www.gstatic.com/firebasejs/10.7.1/firebase-auth.js";

  const firebaseConfig = {
    apiKey: "YOUR_API_KEY",
    authDomain: "YOUR_PROJECT_ID.firebaseapp.com",
    // ... other config
  };

  const app = initializeApp(firebaseConfig);
  const auth = getAuth(app);
  const provider = new GoogleAuthProvider();

  document.getElementById('login-btn').addEventListener('click', async () => {
    try {
      // 1. Client-side login
      const result = await signInWithPopup(auth, provider);
      const idToken = await result.user.getIdToken();

      // 2. Send token to backend
      const response = await fetch('/auth/login', {
        method: 'POST',
        headers: {
          'Authorization': `Bearer ${idToken}`
        }
      });

      if (response.ok) {
        window.location.href = "/dashboard";
      }
    } catch (error) {
      console.error("Login failed", error);
    }
  });
</script>
```

## 2. Backend Implementation (Go)

The backend uses the Firebase Admin SDK to verify the client's ID token and create a session cookie.

### Dependencies
- `firebase.google.com/go/v4`
- `firebase.google.com/go/v4/auth`

### Step A: Initialization

Initialize the Firebase App using your service account credentials.

```go
import (
    "context"
    firebase "firebase.google.com/go/v4"
    "google.golang.org/api/option"
)

func InitFirebase(credPath string) (*auth.Client, error) {
    opt := option.WithCredentialsFile(credPath)
    app, err := firebase.NewApp(context.Background(), nil, opt)
    if err != nil {
        return nil, err
    }
    return app.Auth(context.Background())
}
```

### Step B: Login Handler

This handler receives the ID token, verifies it, and exchanges it for a session cookie.

1.  **Extract Token**: Read the ID token from the request header.
2.  **Verify ID Token**: Check validity using `client.VerifyIDToken()`.
3.  **Create Session Cookie**: Generate a long-lived cookie using `client.SessionCookie()`.
4.  **Set Cookie**: Write the cookie to the response (HttpOnly, Secure).

**Handler Logic (Framework Agnostic):**

```go
func HandleLogin(w http.ResponseWriter, r *http.Request, authClient *auth.Client) {
    // 1. Get ID Token from Authorization Header
    authHeader := r.Header.Get("Authorization")
    tokenString := strings.TrimPrefix(authHeader, "Bearer ")

    // 2. Verify ID Token
    // This ensures the token is valid and signed by Google
    _, err := authClient.VerifyIDToken(r.Context(), tokenString)
    if err != nil {
        http.Error(w, "Invalid token", http.StatusUnauthorized)
        return
    }

    // 3. Create Session Cookie
    // Exchange the ID token for a session cookie (e.g., valid for 5 days)
    expiresIn := time.Hour * 24 * 5
    cookieValue, err := authClient.SessionCookie(r.Context(), tokenString, expiresIn)
    if err != nil {
        http.Error(w, "Failed to create session", http.StatusInternalServerError)
        return
    }

    // 4. Set HTTP-Only Cookie
    http.SetCookie(w, &http.Cookie{
        Name:     "session",
        Value:    cookieValue,
        MaxAge:   int(expiresIn.Seconds()),
        HttpOnly: true,
        Secure:   true, // Set to false for localhost/dev
        Path:     "/",
    })

    w.WriteHeader(http.StatusOK)
    w.Write([]byte(`{"status":"success"}`))
}
```

### Step C: Middleware (Authentication)

Protect your routes by verifying the session cookie on every request.

```go
func AuthMiddleware(next http.Handler, authClient *auth.Client) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // 1. Get the session cookie
        cookie, err := r.Cookie("session")
        if err != nil {
            http.Error(w, "Unauthorized", http.StatusUnauthorized)
            return
        }

        // 2. Verify the session cookie
        // This checks if the session is valid and not revoked
        decodedToken, err := authClient.VerifySessionCookie(r.Context(), cookie.Value)
        if err != nil {
            http.Error(w, "Invalid session", http.StatusUnauthorized)
            return
        }

        // 3. (Optional) Set user info in context
        // ctx := context.WithValue(r.Context(), "user", decodedToken)
        // next.ServeHTTP(w, r.WithContext(ctx))

        next.ServeHTTP(w, r)
    })
}
```

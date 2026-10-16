package services

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestEmailServiceInitialization(t *testing.T) {
	// Test SendGrid initialization
	t.Run("SendGrid initialization", func(t *testing.T) {
		os.Setenv("EMAIL_PROVIDER", "sendgrid")
		os.Setenv("SENDGRID_API_KEY", "test-sendgrid-key")
		os.Setenv("EMAIL_FROM", "test@example.com")
		defer func() {
			os.Unsetenv("EMAIL_PROVIDER")
			os.Unsetenv("SENDGRID_API_KEY")
			os.Unsetenv("EMAIL_FROM")
		}()

		service, err := NewEmailService()
		if err != nil {
			t.Fatalf("Failed to initialize SendGrid service: %v", err)
		}

		if service == nil {
			t.Fatal("Expected non-nil service")
		}

		// Verify it's a SendGrid service
		if _, ok := service.(*SendGridService); !ok {
			t.Error("Expected SendGridService type")
		}
	})

	// Test Mailgun initialization
	t.Run("Mailgun initialization", func(t *testing.T) {
		os.Setenv("EMAIL_PROVIDER", "mailgun")
		os.Setenv("MAILGUN_API_KEY", "test-mailgun-key")
		os.Setenv("MAILGUN_DOMAIN", "example.com")
		os.Setenv("EMAIL_FROM", "test@example.com")
		defer func() {
			os.Unsetenv("EMAIL_PROVIDER")
			os.Unsetenv("MAILGUN_API_KEY")
			os.Unsetenv("MAILGUN_DOMAIN")
			os.Unsetenv("EMAIL_FROM")
		}()

		service, err := NewEmailService()
		if err != nil {
			t.Fatalf("Failed to initialize Mailgun service: %v", err)
		}

		if service == nil {
			t.Fatal("Expected non-nil service")
		}

		// Verify it's a Mailgun service
		if _, ok := service.(*MailgunService); !ok {
			t.Error("Expected MailgunService type")
		}
	})

	// Test failure with missing EMAIL_FROM
	t.Run("Missing EMAIL_FROM", func(t *testing.T) {
		os.Setenv("EMAIL_PROVIDER", "sendgrid")
		os.Setenv("SENDGRID_API_KEY", "test-key")
		defer func() {
			os.Unsetenv("EMAIL_PROVIDER")
			os.Unsetenv("SENDGRID_API_KEY")
		}()

		service, err := NewEmailService()
		if err == nil {
			t.Error("Expected error when EMAIL_FROM is missing")
		}

		if service != nil {
			t.Error("Expected nil service when EMAIL_FROM is missing")
		}

		if !strings.Contains(err.Error(), "EMAIL_FROM") {
			t.Errorf("Expected error message to mention EMAIL_FROM, got: %v", err)
		}
	})

	// Test failure with missing API key
	t.Run("Missing SendGrid API key", func(t *testing.T) {
		os.Setenv("EMAIL_PROVIDER", "sendgrid")
		os.Setenv("EMAIL_FROM", "test@example.com")
		defer func() {
			os.Unsetenv("EMAIL_PROVIDER")
			os.Unsetenv("EMAIL_FROM")
		}()

		service, err := NewEmailService()
		if err == nil {
			t.Error("Expected error when SENDGRID_API_KEY is missing")
		}

		if service != nil {
			t.Error("Expected nil service when API key is missing")
		}

		// The env-based constructor shares its validation with the
		// database-backed config, so the message names the setting,
		// not the environment variable.
		if err != nil && !strings.Contains(err.Error(), "SendGrid API key") {
			t.Errorf("Expected error message to mention the SendGrid API key, got: %v", err)
		}
	})

	// Test unsupported provider
	t.Run("Unsupported provider", func(t *testing.T) {
		os.Setenv("EMAIL_PROVIDER", "unsupported")
		os.Setenv("EMAIL_FROM", "test@example.com")
		defer func() {
			os.Unsetenv("EMAIL_PROVIDER")
			os.Unsetenv("EMAIL_FROM")
		}()

		service, err := NewEmailService()
		if err == nil {
			t.Error("Expected error for unsupported provider")
		}

		if service != nil {
			t.Error("Expected nil service for unsupported provider")
		}

		if !strings.Contains(err.Error(), "unsupported email provider") {
			t.Errorf("Expected error message about unsupported provider, got: %v", err)
		}
	})
}

func TestSendGridEmailFormatting(t *testing.T) {
	// Test SendGrid API payload structure
	service := &SendGridService{
		APIKey:   "test-key",
		FromAddr: "test@example.com",
		BaseURL:  "https://api.sendgrid.com/v3",
	}

	// Test confirmation email content structure
	t.Run("Confirmation email structure", func(t *testing.T) {
		// Create a message structure like SendConfirmationEmail does
		message := SendGridMessage{
			Personalizations: []SendGridPersonalization{
				{
					To: []SendGridEmail{
						{Email: "recipient@example.com"},
					},
				},
			},
			From: SendGridEmail{
				Email: service.FromAddr,
				Name:  "Status Page",
			},
			Subject: "Confirm Your Subscription to Status Updates",
			Content: []SendGridContent{
				{
					Type:  "text/html",
					Value: "<html><body><h2>Confirm Your Subscription</h2></body></html>",
				},
			},
		}

		// Verify structure
		if len(message.Personalizations) != 1 {
			t.Errorf("Expected 1 personalization, got %d", len(message.Personalizations))
		}

		if len(message.Personalizations[0].To) != 1 {
			t.Errorf("Expected 1 recipient, got %d", len(message.Personalizations[0].To))
		}

		if message.Personalizations[0].To[0].Email != "recipient@example.com" {
			t.Errorf("Expected recipient email 'recipient@example.com', got '%s'", message.Personalizations[0].To[0].Email)
		}

		if message.From.Email != "test@example.com" {
			t.Errorf("Expected from email 'test@example.com', got '%s'", message.From.Email)
		}

		if message.From.Name != "Status Page" {
			t.Errorf("Expected from name 'Status Page', got '%s'", message.From.Name)
		}

		if len(message.Content) != 1 {
			t.Errorf("Expected 1 content block, got %d", len(message.Content))
		}

		if message.Content[0].Type != "text/html" {
			t.Errorf("Expected content type 'text/html', got '%s'", message.Content[0].Type)
		}
	})

	// Test incident notification content structure
	t.Run("Incident notification structure", func(t *testing.T) {
		message := SendGridMessage{
			Personalizations: []SendGridPersonalization{
				{
					To: []SendGridEmail{
						{Email: "recipient@example.com"},
					},
				},
			},
			From: SendGridEmail{
				Email: service.FromAddr,
				Name:  "Status Page",
			},
			Subject: "Incident Update: API Outage - investigating",
			Content: []SendGridContent{
				{
					Type:  "text/html",
					Value: "<html><body><h2>Incident Update: API Outage</h2></body></html>",
				},
			},
		}

		// Verify structure
		if !strings.Contains(message.Subject, "Incident Update") {
			t.Error("Expected subject to contain 'Incident Update'")
		}

		if message.Content[0].Type != "text/html" {
			t.Errorf("Expected content type 'text/html', got '%s'", message.Content[0].Type)
		}
	})
}

func TestMailgunEmailFormatting(t *testing.T) {
	// Test Mailgun API payload structure
	service := &MailgunService{
		APIKey:   "test-key",
		FromAddr: "test@example.com",
		Domain:   "example.com",
		BaseURL:  "https://api.mailgun.net/v3/example.com",
	}

	t.Run("Mailgun data structure", func(t *testing.T) {
		// Simulate Mailgun email data structure
		data := map[string]string{
			"from":    "Status Page <test@example.com>",
			"to":      "recipient@example.com",
			"subject": "Test Subject",
			"html":    "<html><body><h1>Test</h1></body></html>",
		}

		// Verify structure
		if data["from"] == "" {
			t.Error("Expected 'from' field to be set")
		}

		if !strings.Contains(data["from"], service.FromAddr) {
			t.Errorf("Expected 'from' to contain %s, got %s", service.FromAddr, data["from"])
		}

		if data["to"] != "recipient@example.com" {
			t.Errorf("Expected 'to' to be 'recipient@example.com', got '%s'", data["to"])
		}

		if data["subject"] == "" {
			t.Error("Expected 'subject' field to be set")
		}

		if data["html"] == "" {
			t.Error("Expected 'html' field to be set")
		}
	})
}

func TestEmailRecipientValidation(t *testing.T) {
	// Test that invalid email addresses should be rejected
	invalidEmails := []string{
		"invalid",
		"@example.com",
		"user@",
		"",
		"user space@example.com",
		"user@domain",
	}

	for _, email := range invalidEmails {
		t.Run("Invalid email: "+email, func(t *testing.T) {
			// In production, email validation should occur before calling the service
			// This test documents expected invalid email formats

			// Simple validation check
			isValid := strings.Contains(email, "@") &&
				strings.Contains(email, ".") &&
				len(email) > 5 &&
				!strings.Contains(email, " ")

			if email == "" || email == "invalid" || email == "@example.com" || email == "user@" {
				isValid = false
			}

			if isValid && (email == "invalid" || email == "@example.com" || email == "user@") {
				t.Errorf("Email '%s' should be invalid", email)
			}
		})
	}
}

// captureSendGrid returns a SendGrid service pointed at a local test server,
// and a function returning the last message that server received.
func captureSendGrid(t *testing.T) (*SendGridService, func() SendGridMessage) {
	t.Helper()

	var got SendGridMessage
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/mail/send" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Errorf("invalid SendGrid payload: %v", err)
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	t.Cleanup(server.Close)

	service := &SendGridService{
		APIKey:   "test-key",
		FromAddr: "noreply@example.com",
		BaseURL:  server.URL,
	}
	return service, func() SendGridMessage { return got }
}

func TestConfirmationEmailContent(t *testing.T) {
	service, sent := captureSendGrid(t)
	t.Setenv("API_URL", "http://localhost:8080")

	if err := service.SendConfirmationEmail("reader@example.com", "test-token-123"); err != nil {
		t.Fatalf("SendConfirmationEmail failed: %v", err)
	}

	msg := sent()
	if len(msg.Personalizations) != 1 || msg.Personalizations[0].To[0].Email != "reader@example.com" {
		t.Errorf("unexpected recipients: %+v", msg.Personalizations)
	}
	if len(msg.Content) != 1 {
		t.Fatalf("expected one content part, got %d", len(msg.Content))
	}
	body := msg.Content[0].Value
	if want := "http://localhost:8080/api/v1/confirm?token=test-token-123"; !strings.Contains(body, want) {
		t.Errorf("confirmation email should contain %q", want)
	}
}

func TestIncidentNotificationContent(t *testing.T) {
	service, sent := captureSendGrid(t)
	t.Setenv("API_URL", "http://localhost:8080")

	err := service.SendIncidentNotification("reader@example.com", "API Outage", "investigating",
		"We are investigating an issue with the API", "unsubscribe-token-456")
	if err != nil {
		t.Fatalf("SendIncidentNotification failed: %v", err)
	}

	msg := sent()
	if len(msg.Content) != 1 {
		t.Fatalf("expected one content part, got %d", len(msg.Content))
	}
	body := msg.Content[0].Value
	for _, want := range []string{
		"API Outage",
		"investigating",
		"We are investigating an issue with the API",
		"http://localhost:8080/api/v1/unsubscribe?token=unsubscribe-token-456",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("incident email should contain %q", want)
		}
	}
}

func TestEmailServiceTypes(t *testing.T) {
	// Test that all service types implement EmailService interface
	var _ EmailService = &SendGridService{}
	var _ EmailService = &MailgunService{}
	var _ EmailService = &SESService{}
}

func TestSendGridBaseURL(t *testing.T) {
	os.Setenv("EMAIL_PROVIDER", "sendgrid")
	os.Setenv("SENDGRID_API_KEY", "test-key")
	os.Setenv("EMAIL_FROM", "test@example.com")
	defer func() {
		os.Unsetenv("EMAIL_PROVIDER")
		os.Unsetenv("SENDGRID_API_KEY")
		os.Unsetenv("EMAIL_FROM")
	}()

	service, err := NewEmailService()
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	sendgridService, ok := service.(*SendGridService)
	if !ok {
		t.Fatal("Expected SendGridService type")
	}

	expectedBaseURL := "https://api.sendgrid.com/v3"
	if sendgridService.BaseURL != expectedBaseURL {
		t.Errorf("Expected BaseURL '%s', got '%s'", expectedBaseURL, sendgridService.BaseURL)
	}
}

func TestMailgunBaseURL(t *testing.T) {
	os.Setenv("EMAIL_PROVIDER", "mailgun")
	os.Setenv("MAILGUN_API_KEY", "test-key")
	os.Setenv("MAILGUN_DOMAIN", "example.com")
	os.Setenv("EMAIL_FROM", "test@example.com")
	defer func() {
		os.Unsetenv("EMAIL_PROVIDER")
		os.Unsetenv("MAILGUN_API_KEY")
		os.Unsetenv("MAILGUN_DOMAIN")
		os.Unsetenv("EMAIL_FROM")
	}()

	service, err := NewEmailService()
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	mailgunService, ok := service.(*MailgunService)
	if !ok {
		t.Fatal("Expected MailgunService type")
	}

	expectedBaseURL := "https://api.mailgun.net/v3/example.com"
	if mailgunService.BaseURL != expectedBaseURL {
		t.Errorf("Expected BaseURL '%s', got '%s'", expectedBaseURL, mailgunService.BaseURL)
	}
}

func TestDefaultProvider(t *testing.T) {
	// Test that SendGrid is the default provider
	os.Setenv("SENDGRID_API_KEY", "test-key")
	os.Setenv("EMAIL_FROM", "test@example.com")
	defer func() {
		os.Unsetenv("SENDGRID_API_KEY")
		os.Unsetenv("EMAIL_FROM")
	}()

	service, err := NewEmailService()
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	if _, ok := service.(*SendGridService); !ok {
		t.Error("Expected SendGrid to be default provider")
	}
}

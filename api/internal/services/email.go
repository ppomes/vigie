package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
)

// EmailService interface defines the contract for email providers
type EmailService interface {
	SendConfirmationEmail(to, confirmationToken string) error
	SendIncidentNotification(to, incidentName, incidentStatus, incidentMessage, unsubscribeToken string) error
}

// EmailProvider represents the email service provider configuration
type EmailProvider struct {
	Provider string
	APIKey   string
	FromAddr string
	BaseURL  string
}

// NewEmailService creates a new email service based on the configured provider
func NewEmailService() (EmailService, error) {
	provider := os.Getenv("EMAIL_PROVIDER")
	apiKey := os.Getenv("SENDGRID_API_KEY")
	fromAddr := os.Getenv("EMAIL_FROM")

	if provider == "" {
		provider = "sendgrid" // Default to SendGrid
	}

	if fromAddr == "" {
		return nil, fmt.Errorf("EMAIL_FROM environment variable is required")
	}

	switch provider {
	case "sendgrid":
		if apiKey == "" {
			return nil, fmt.Errorf("SENDGRID_API_KEY environment variable is required for SendGrid provider")
		}
		return &SendGridService{
			APIKey:   apiKey,
			FromAddr: fromAddr,
			BaseURL:  "https://api.sendgrid.com/v3",
		}, nil
	case "mailgun":
		mailgunDomain := os.Getenv("MAILGUN_DOMAIN")
		apiKey = os.Getenv("MAILGUN_API_KEY")
		if apiKey == "" || mailgunDomain == "" {
			return nil, fmt.Errorf("MAILGUN_API_KEY and MAILGUN_DOMAIN environment variables are required for Mailgun provider")
		}
		return &MailgunService{
			APIKey:   apiKey,
			FromAddr: fromAddr,
			Domain:   mailgunDomain,
			BaseURL:  fmt.Sprintf("https://api.mailgun.net/v3/%s", mailgunDomain),
		}, nil
	case "ses":
		region := os.Getenv("AWS_REGION")
		accessKey := os.Getenv("AWS_ACCESS_KEY_ID")
		secretKey := os.Getenv("AWS_SECRET_ACCESS_KEY")
		if region == "" || accessKey == "" || secretKey == "" {
			return nil, fmt.Errorf("AWS_REGION, AWS_ACCESS_KEY_ID, and AWS_SECRET_ACCESS_KEY environment variables are required for SES provider")
		}
		return &SESService{
			Region:    region,
			AccessKey: accessKey,
			SecretKey: secretKey,
			FromAddr:  fromAddr,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported email provider: %s (supported: sendgrid, mailgun, ses)", provider)
	}
}

// SendGridService implements EmailService for SendGrid
type SendGridService struct {
	APIKey   string
	FromAddr string
	BaseURL  string
}

// SendGridMessage represents the SendGrid API v3 mail/send payload
type SendGridMessage struct {
	Personalizations []SendGridPersonalization `json:"personalizations"`
	From             SendGridEmail             `json:"from"`
	Subject          string                    `json:"subject"`
	Content          []SendGridContent         `json:"content"`
}

type SendGridPersonalization struct {
	To []SendGridEmail `json:"to"`
}

type SendGridEmail struct {
	Email string `json:"email"`
	Name  string `json:"name,omitempty"`
}

type SendGridContent struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

// SendConfirmationEmail sends a confirmation email via SendGrid
func (s *SendGridService) SendConfirmationEmail(to, confirmationToken string) error {
	apiURL := os.Getenv("API_URL")
	if apiURL == "" {
		apiURL = "http://localhost:8080"
	}

	confirmationURL := fmt.Sprintf("%s/api/v1/confirm?token=%s", apiURL, confirmationToken)

	htmlContent := fmt.Sprintf(`
		<html>
		<body>
			<h2>Confirm Your Subscription</h2>
			<p>Thank you for subscribing to our status updates!</p>
			<p>Please click the link below to confirm your subscription:</p>
			<p><a href="%s">Confirm Subscription</a></p>
			<p>Or copy and paste this URL into your browser:</p>
			<p>%s</p>
			<p>If you did not request this subscription, please ignore this email.</p>
		</body>
		</html>
	`, confirmationURL, confirmationURL)

	message := SendGridMessage{
		Personalizations: []SendGridPersonalization{
			{
				To: []SendGridEmail{
					{Email: to},
				},
			},
		},
		From: SendGridEmail{
			Email: s.FromAddr,
			Name:  "Status Page",
		},
		Subject: "Confirm Your Subscription to Status Updates",
		Content: []SendGridContent{
			{
				Type:  "text/html",
				Value: htmlContent,
			},
		},
	}

	return s.sendEmail(message)
}

// SendIncidentNotification sends an incident notification email via SendGrid
func (s *SendGridService) SendIncidentNotification(to, incidentName, incidentStatus, incidentMessage, unsubscribeToken string) error {
	apiURL := os.Getenv("API_URL")
	if apiURL == "" {
		apiURL = "http://localhost:8080"
	}

	unsubscribeURL := fmt.Sprintf("%s/api/v1/unsubscribe?token=%s", apiURL, unsubscribeToken)

	htmlContent := fmt.Sprintf(`
		<html>
		<body>
			<h2>Incident Update: %s</h2>
			<p><strong>Status:</strong> %s</p>
			<p><strong>Message:</strong></p>
			<p>%s</p>
			<hr>
			<p style="font-size: 12px; color: #666;">
				If you no longer wish to receive these notifications,
				<a href="%s">click here to unsubscribe</a>.
			</p>
		</body>
		</html>
	`, incidentName, incidentStatus, incidentMessage, unsubscribeURL)

	message := SendGridMessage{
		Personalizations: []SendGridPersonalization{
			{
				To: []SendGridEmail{
					{Email: to},
				},
			},
		},
		From: SendGridEmail{
			Email: s.FromAddr,
			Name:  "Status Page",
		},
		Subject: fmt.Sprintf("Incident Update: %s - %s", incidentName, incidentStatus),
		Content: []SendGridContent{
			{
				Type:  "text/html",
				Value: htmlContent,
			},
		},
	}

	return s.sendEmail(message)
}

// sendEmail sends an email via SendGrid API
func (s *SendGridService) sendEmail(message SendGridMessage) error {
	payload, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal email payload: %w", err)
	}

	req, err := http.NewRequest("POST", s.BaseURL+"/mail/send", bytes.NewBuffer(payload))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+s.APIKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("sendgrid API returned non-success status: %d", resp.StatusCode)
	}

	return nil
}

// MailgunService implements EmailService for Mailgun
type MailgunService struct {
	APIKey   string
	FromAddr string
	Domain   string
	BaseURL  string
}

// SendConfirmationEmail sends a confirmation email via Mailgun
func (m *MailgunService) SendConfirmationEmail(to, confirmationToken string) error {
	apiURL := os.Getenv("API_URL")
	if apiURL == "" {
		apiURL = "http://localhost:8080"
	}

	confirmationURL := fmt.Sprintf("%s/api/v1/confirm?token=%s", apiURL, confirmationToken)

	htmlContent := fmt.Sprintf(`
		<html>
		<body>
			<h2>Confirm Your Subscription</h2>
			<p>Thank you for subscribing to our status updates!</p>
			<p>Please click the link below to confirm your subscription:</p>
			<p><a href="%s">Confirm Subscription</a></p>
			<p>Or copy and paste this URL into your browser:</p>
			<p>%s</p>
			<p>If you did not request this subscription, please ignore this email.</p>
		</body>
		</html>
	`, confirmationURL, confirmationURL)

	data := map[string]string{
		"from":    fmt.Sprintf("Status Page <%s>", m.FromAddr),
		"to":      to,
		"subject": "Confirm Your Subscription to Status Updates",
		"html":    htmlContent,
	}

	return m.sendEmail(data)
}

// SendIncidentNotification sends an incident notification email via Mailgun
func (m *MailgunService) SendIncidentNotification(to, incidentName, incidentStatus, incidentMessage, unsubscribeToken string) error {
	apiURL := os.Getenv("API_URL")
	if apiURL == "" {
		apiURL = "http://localhost:8080"
	}

	unsubscribeURL := fmt.Sprintf("%s/api/v1/unsubscribe?token=%s", apiURL, unsubscribeToken)

	htmlContent := fmt.Sprintf(`
		<html>
		<body>
			<h2>Incident Update: %s</h2>
			<p><strong>Status:</strong> %s</p>
			<p><strong>Message:</strong></p>
			<p>%s</p>
			<hr>
			<p style="font-size: 12px; color: #666;">
				If you no longer wish to receive these notifications,
				<a href="%s">click here to unsubscribe</a>.
			</p>
		</body>
		</html>
	`, incidentName, incidentStatus, incidentMessage, unsubscribeURL)

	data := map[string]string{
		"from":    fmt.Sprintf("Status Page <%s>", m.FromAddr),
		"to":      to,
		"subject": fmt.Sprintf("Incident Update: %s - %s", incidentName, incidentStatus),
		"html":    htmlContent,
	}

	return m.sendEmail(data)
}

// sendEmail sends an email via Mailgun API
func (m *MailgunService) sendEmail(data map[string]string) error {
	payload := bytes.NewBuffer([]byte{})
	for key, value := range data {
		payload.WriteString(fmt.Sprintf("%s=%s&", key, value))
	}

	req, err := http.NewRequest("POST", m.BaseURL+"/messages", payload)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.SetBasicAuth("api", m.APIKey)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("mailgun API returned non-success status: %d", resp.StatusCode)
	}

	return nil
}

// SESService implements EmailService for AWS SES
type SESService struct {
	Region    string
	AccessKey string
	SecretKey string
	FromAddr  string
}

// SendConfirmationEmail sends a confirmation email via AWS SES
func (s *SESService) SendConfirmationEmail(to, confirmationToken string) error {
	apiURL := os.Getenv("API_URL")
	if apiURL == "" {
		apiURL = "http://localhost:8080"
	}

	confirmationURL := fmt.Sprintf("%s/api/v1/confirm?token=%s", apiURL, confirmationToken)

	htmlContent := fmt.Sprintf(`
		<html>
		<body>
			<h2>Confirm Your Subscription</h2>
			<p>Thank you for subscribing to our status updates!</p>
			<p>Please click the link below to confirm your subscription:</p>
			<p><a href="%s">Confirm Subscription</a></p>
			<p>Or copy and paste this URL into your browser:</p>
			<p>%s</p>
			<p>If you did not request this subscription, please ignore this email.</p>
		</body>
		</html>
	`, confirmationURL, confirmationURL)

	subject := "Confirm Your Subscription to Status Updates"

	return s.sendEmail(to, subject, htmlContent)
}

// SendIncidentNotification sends an incident notification email via AWS SES
func (s *SESService) SendIncidentNotification(to, incidentName, incidentStatus, incidentMessage, unsubscribeToken string) error {
	apiURL := os.Getenv("API_URL")
	if apiURL == "" {
		apiURL = "http://localhost:8080"
	}

	unsubscribeURL := fmt.Sprintf("%s/api/v1/unsubscribe?token=%s", apiURL, unsubscribeToken)

	htmlContent := fmt.Sprintf(`
		<html>
		<body>
			<h2>Incident Update: %s</h2>
			<p><strong>Status:</strong> %s</p>
			<p><strong>Message:</strong></p>
			<p>%s</p>
			<hr>
			<p style="font-size: 12px; color: #666;">
				If you no longer wish to receive these notifications,
				<a href="%s">click here to unsubscribe</a>.
			</p>
		</body>
		</html>
	`, incidentName, incidentStatus, incidentMessage, unsubscribeURL)

	subject := fmt.Sprintf("Incident Update: %s - %s", incidentName, incidentStatus)

	return s.sendEmail(to, subject, htmlContent)
}

// sendEmail sends an email via AWS SES API
// Note: This is a simplified implementation. Production usage should use AWS SDK.
func (s *SESService) sendEmail(to, subject, htmlContent string) error {
	// AWS SES v2 SendEmail API endpoint
	endpoint := fmt.Sprintf("https://email.%s.amazonaws.com/v2/email/outbound-emails", s.Region)

	message := map[string]interface{}{
		"Content": map[string]interface{}{
			"Simple": map[string]interface{}{
				"Subject": map[string]string{
					"Data":    subject,
					"Charset": "UTF-8",
				},
				"Body": map[string]interface{}{
					"Html": map[string]string{
						"Data":    htmlContent,
						"Charset": "UTF-8",
					},
				},
			},
		},
		"Destination": map[string]interface{}{
			"ToAddresses": []string{to},
		},
		"FromEmailAddress": s.FromAddr,
	}

	payload, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal email payload: %w", err)
	}

	req, err := http.NewRequest("POST", endpoint, bytes.NewBuffer(payload))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Note: Production implementation should use AWS Signature Version 4 signing
	// For now, this is a placeholder. Full AWS SES integration requires the AWS SDK.
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send email via SES: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("AWS SES API returned non-success status: %d", resp.StatusCode)
	}

	return nil
}

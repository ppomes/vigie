package services

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"net/smtp"
	"os"
	"strconv"
	"time"
)

// EmailService interface defines the contract for email providers
type EmailService interface {
	SendConfirmationEmail(to, confirmationToken string) error
	SendIncidentNotification(to, incidentName, incidentStatus, incidentMessage, unsubscribeToken string) error
	SendPasswordResetEmail(to, resetToken string) error
	SendRawEmail(to, subject, body string) error
}

// EmailConfig represents the email configuration stored in database
type EmailConfig struct {
	Provider     string `json:"provider"`      // smtp, sendgrid, mailgun, ses, disabled
	FromAddress  string `json:"from_address"`
	FromName     string `json:"from_name"`
	// SMTP settings
	SMTPHost     string `json:"smtp_host"`
	SMTPPort     int    `json:"smtp_port"`
	SMTPUsername string `json:"smtp_username"`
	SMTPPassword string `json:"smtp_password"`
	SMTPTLS      bool   `json:"smtp_tls"`
	// SendGrid settings
	SendGridAPIKey string `json:"sendgrid_api_key"`
	// Mailgun settings
	MailgunAPIKey  string `json:"mailgun_api_key"`
	MailgunDomain  string `json:"mailgun_domain"`
	// AWS SES settings
	AWSRegion    string `json:"aws_region"`
	AWSAccessKey string `json:"aws_access_key"`
	AWSSecretKey string `json:"aws_secret_key"`
}

// NewEmailServiceFromConfig creates a new email service based on the provided configuration
func NewEmailServiceFromConfig(config *EmailConfig) (EmailService, error) {
	if config == nil || config.Provider == "" || config.Provider == "disabled" {
		return nil, fmt.Errorf("email service is disabled or not configured")
	}

	if config.FromAddress == "" {
		return nil, fmt.Errorf("from address is required")
	}

	fromName := config.FromName
	if fromName == "" {
		fromName = "Status Page"
	}

	switch config.Provider {
	case "smtp":
		if config.SMTPHost == "" {
			return nil, fmt.Errorf("SMTP host is required")
		}
		port := config.SMTPPort
		if port == 0 {
			port = 587
		}
		return &SMTPService{
			Host:     config.SMTPHost,
			Port:     port,
			Username: config.SMTPUsername,
			Password: config.SMTPPassword,
			UseTLS:   config.SMTPTLS,
			FromAddr: config.FromAddress,
			FromName: fromName,
		}, nil
	case "sendgrid":
		if config.SendGridAPIKey == "" {
			return nil, fmt.Errorf("SendGrid API key is required")
		}
		return &SendGridService{
			APIKey:   config.SendGridAPIKey,
			FromAddr: config.FromAddress,
			FromName: fromName,
			BaseURL:  "https://api.sendgrid.com/v3",
		}, nil
	case "mailgun":
		if config.MailgunAPIKey == "" || config.MailgunDomain == "" {
			return nil, fmt.Errorf("Mailgun API key and domain are required")
		}
		return &MailgunService{
			APIKey:   config.MailgunAPIKey,
			FromAddr: config.FromAddress,
			FromName: fromName,
			Domain:   config.MailgunDomain,
			BaseURL:  fmt.Sprintf("https://api.mailgun.net/v3/%s", config.MailgunDomain),
		}, nil
	case "ses":
		if config.AWSRegion == "" || config.AWSAccessKey == "" || config.AWSSecretKey == "" {
			return nil, fmt.Errorf("AWS region, access key, and secret key are required for SES")
		}
		return &SESService{
			Region:    config.AWSRegion,
			AccessKey: config.AWSAccessKey,
			SecretKey: config.AWSSecretKey,
			FromAddr:  config.FromAddress,
			FromName:  fromName,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported email provider: %s", config.Provider)
	}
}

// NewEmailService creates a new email service based on environment variables (legacy)
func NewEmailService() (EmailService, error) {
	provider := os.Getenv("EMAIL_PROVIDER")
	fromAddr := os.Getenv("EMAIL_FROM")

	if provider == "" {
		provider = "sendgrid"
	}

	if fromAddr == "" {
		return nil, fmt.Errorf("EMAIL_FROM environment variable is required")
	}

	config := &EmailConfig{
		Provider:       provider,
		FromAddress:    fromAddr,
		FromName:       "Status Page",
		SendGridAPIKey: os.Getenv("SENDGRID_API_KEY"),
		MailgunAPIKey:  os.Getenv("MAILGUN_API_KEY"),
		MailgunDomain:  os.Getenv("MAILGUN_DOMAIN"),
		AWSRegion:      os.Getenv("AWS_REGION"),
		AWSAccessKey:   os.Getenv("AWS_ACCESS_KEY_ID"),
		AWSSecretKey:   os.Getenv("AWS_SECRET_ACCESS_KEY"),
		SMTPHost:       os.Getenv("SMTP_HOST"),
		SMTPUsername:   os.Getenv("SMTP_USERNAME"),
		SMTPPassword:   os.Getenv("SMTP_PASSWORD"),
		SMTPTLS:        os.Getenv("SMTP_TLS") == "true",
	}

	if port := os.Getenv("SMTP_PORT"); port != "" {
		if p, err := strconv.Atoi(port); err == nil {
			config.SMTPPort = p
		}
	}

	return NewEmailServiceFromConfig(config)
}

// SMTPService implements EmailService for custom SMTP servers
type SMTPService struct {
	Host     string
	Port     int
	Username string
	Password string
	UseTLS   bool
	FromAddr string
	FromName string
}

func (s *SMTPService) sendEmail(to, subject, htmlContent string) error {
	from := s.FromAddr
	if s.FromName != "" {
		from = fmt.Sprintf("%s <%s>", s.FromName, s.FromAddr)
	}

	headers := map[string]string{
		"From":         from,
		"To":           to,
		"Subject":      subject,
		"MIME-Version": "1.0",
		"Content-Type": "text/html; charset=\"UTF-8\"",
	}

	var message bytes.Buffer
	for k, v := range headers {
		message.WriteString(fmt.Sprintf("%s: %s\r\n", k, v))
	}
	message.WriteString("\r\n")
	message.WriteString(htmlContent)

	addr := fmt.Sprintf("%s:%d", s.Host, s.Port)

	var auth smtp.Auth
	if s.Username != "" && s.Password != "" {
		auth = smtp.PlainAuth("", s.Username, s.Password, s.Host)
	}

	if s.UseTLS {
		// TLS connection
		tlsConfig := &tls.Config{
			ServerName: s.Host,
		}
		conn, err := tls.Dial("tcp", addr, tlsConfig)
		if err != nil {
			return fmt.Errorf("failed to connect to SMTP server: %w", err)
		}
		defer conn.Close()

		client, err := smtp.NewClient(conn, s.Host)
		if err != nil {
			return fmt.Errorf("failed to create SMTP client: %w", err)
		}
		defer client.Close()

		if auth != nil {
			if err := client.Auth(auth); err != nil {
				return fmt.Errorf("SMTP authentication failed: %w", err)
			}
		}

		if err := client.Mail(s.FromAddr); err != nil {
			return fmt.Errorf("failed to set sender: %w", err)
		}
		if err := client.Rcpt(to); err != nil {
			return fmt.Errorf("failed to set recipient: %w", err)
		}

		w, err := client.Data()
		if err != nil {
			return fmt.Errorf("failed to get data writer: %w", err)
		}
		_, err = w.Write(message.Bytes())
		if err != nil {
			return fmt.Errorf("failed to write message: %w", err)
		}
		err = w.Close()
		if err != nil {
			return fmt.Errorf("failed to close data writer: %w", err)
		}

		return client.Quit()
	}

	// Non-TLS connection - try STARTTLS first, fall back to plaintext
	// Connect to the server
	client, err := smtp.Dial(addr)
	if err != nil {
		return fmt.Errorf("failed to connect to SMTP server: %w", err)
	}
	defer client.Close()

	// Try STARTTLS if available
	if ok, _ := client.Extension("STARTTLS"); ok {
		tlsConfig := &tls.Config{ServerName: s.Host}
		if err := client.StartTLS(tlsConfig); err != nil {
			// STARTTLS failed, reconnect without TLS
			client.Close()
			client, err = smtp.Dial(addr)
			if err != nil {
				return fmt.Errorf("failed to reconnect to SMTP server: %w", err)
			}
			defer client.Close()
		}
	}

	// Authenticate if credentials provided
	if auth != nil {
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("SMTP authentication failed: %w", err)
		}
	}

	// Send the email
	if err := client.Mail(s.FromAddr); err != nil {
		return fmt.Errorf("failed to set sender: %w", err)
	}
	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("failed to set recipient: %w", err)
	}

	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("failed to get data writer: %w", err)
	}
	_, err = w.Write(message.Bytes())
	if err != nil {
		return fmt.Errorf("failed to write message: %w", err)
	}
	err = w.Close()
	if err != nil {
		return fmt.Errorf("failed to close data writer: %w", err)
	}

	return client.Quit()
}

func (s *SMTPService) SendConfirmationEmail(to, confirmationToken string) error {
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

	return s.sendEmail(to, "Confirm Your Subscription to Status Updates", htmlContent)
}

func (s *SMTPService) SendIncidentNotification(to, incidentName, incidentStatus, incidentMessage, unsubscribeToken string) error {
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

func (s *SMTPService) SendPasswordResetEmail(to, resetToken string) error {
	adminURL := os.Getenv("ADMIN_URL")
	if adminURL == "" {
		adminURL = "http://localhost:3001"
	}

	resetURL := fmt.Sprintf("%s/reset-password?token=%s", adminURL, resetToken)

	htmlContent := fmt.Sprintf(`
		<html>
		<body>
			<h2>Password Reset Request</h2>
			<p>You have requested to reset your password.</p>
			<p>Click the link below to reset your password:</p>
			<p><a href="%s">Reset Password</a></p>
			<p>Or copy and paste this URL into your browser:</p>
			<p>%s</p>
			<p>This link will expire in 1 hour.</p>
			<p>If you did not request this password reset, please ignore this email.</p>
		</body>
		</html>
	`, resetURL, resetURL)

	return s.sendEmail(to, "Password Reset Request", htmlContent)
}

// SendRawEmail sends an email with pre-formatted subject and body
func (s *SMTPService) SendRawEmail(to, subject, body string) error {
	// Convert plain text body to simple HTML
	htmlContent := fmt.Sprintf(`<html><body><pre style="font-family: sans-serif;">%s</pre></body></html>`, body)
	return s.sendEmail(to, subject, htmlContent)
}

// SendGridService implements EmailService for SendGrid
type SendGridService struct {
	APIKey   string
	FromAddr string
	FromName string
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
			Name:  s.FromName,
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
			Name:  s.FromName,
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

// SendPasswordResetEmail sends a password reset email via SendGrid
func (s *SendGridService) SendPasswordResetEmail(to, resetToken string) error {
	adminURL := os.Getenv("ADMIN_URL")
	if adminURL == "" {
		adminURL = "http://localhost:3001"
	}

	resetURL := fmt.Sprintf("%s/reset-password?token=%s", adminURL, resetToken)

	htmlContent := fmt.Sprintf(`
		<html>
		<body>
			<h2>Password Reset Request</h2>
			<p>You have requested to reset your password.</p>
			<p>Click the link below to reset your password:</p>
			<p><a href="%s">Reset Password</a></p>
			<p>Or copy and paste this URL into your browser:</p>
			<p>%s</p>
			<p>This link will expire in 1 hour.</p>
			<p>If you did not request this password reset, please ignore this email.</p>
		</body>
		</html>
	`, resetURL, resetURL)

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
			Name:  s.FromName,
		},
		Subject: "Password Reset Request",
		Content: []SendGridContent{
			{
				Type:  "text/html",
				Value: htmlContent,
			},
		},
	}

	return s.sendEmail(message)
}

// SendRawEmail sends an email with pre-formatted subject and body via SendGrid
func (s *SendGridService) SendRawEmail(to, subject, body string) error {
	htmlContent := fmt.Sprintf(`<html><body><pre style="font-family: sans-serif;">%s</pre></body></html>`, body)

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
			Name:  s.FromName,
		},
		Subject: subject,
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
	FromName string
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
		"from":    fmt.Sprintf("%s <%s>", m.FromName, m.FromAddr),
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
		"from":    fmt.Sprintf("%s <%s>", m.FromName, m.FromAddr),
		"to":      to,
		"subject": fmt.Sprintf("Incident Update: %s - %s", incidentName, incidentStatus),
		"html":    htmlContent,
	}

	return m.sendEmail(data)
}

// SendPasswordResetEmail sends a password reset email via Mailgun
func (m *MailgunService) SendPasswordResetEmail(to, resetToken string) error {
	adminURL := os.Getenv("ADMIN_URL")
	if adminURL == "" {
		adminURL = "http://localhost:3001"
	}

	resetURL := fmt.Sprintf("%s/reset-password?token=%s", adminURL, resetToken)

	htmlContent := fmt.Sprintf(`
		<html>
		<body>
			<h2>Password Reset Request</h2>
			<p>You have requested to reset your password.</p>
			<p>Click the link below to reset your password:</p>
			<p><a href="%s">Reset Password</a></p>
			<p>Or copy and paste this URL into your browser:</p>
			<p>%s</p>
			<p>This link will expire in 1 hour.</p>
			<p>If you did not request this password reset, please ignore this email.</p>
		</body>
		</html>
	`, resetURL, resetURL)

	data := map[string]string{
		"from":    fmt.Sprintf("%s <%s>", m.FromName, m.FromAddr),
		"to":      to,
		"subject": "Password Reset Request",
		"html":    htmlContent,
	}

	return m.sendEmail(data)
}

// SendRawEmail sends an email with pre-formatted subject and body via Mailgun
func (m *MailgunService) SendRawEmail(to, subject, body string) error {
	htmlContent := fmt.Sprintf(`<html><body><pre style="font-family: sans-serif;">%s</pre></body></html>`, body)

	data := map[string]string{
		"from":    fmt.Sprintf("%s <%s>", m.FromName, m.FromAddr),
		"to":      to,
		"subject": subject,
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
	FromName  string
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

// SendPasswordResetEmail sends a password reset email via AWS SES
func (s *SESService) SendPasswordResetEmail(to, resetToken string) error {
	adminURL := os.Getenv("ADMIN_URL")
	if adminURL == "" {
		adminURL = "http://localhost:3001"
	}

	resetURL := fmt.Sprintf("%s/reset-password?token=%s", adminURL, resetToken)

	htmlContent := fmt.Sprintf(`
		<html>
		<body>
			<h2>Password Reset Request</h2>
			<p>You have requested to reset your password.</p>
			<p>Click the link below to reset your password:</p>
			<p><a href="%s">Reset Password</a></p>
			<p>Or copy and paste this URL into your browser:</p>
			<p>%s</p>
			<p>This link will expire in 1 hour.</p>
			<p>If you did not request this password reset, please ignore this email.</p>
		</body>
		</html>
	`, resetURL, resetURL)

	return s.sendEmail(to, "Password Reset Request", htmlContent)
}

// SendRawEmail sends an email with pre-formatted subject and body via AWS SES
func (s *SESService) SendRawEmail(to, subject, body string) error {
	htmlContent := fmt.Sprintf(`<html><body><pre style="font-family: sans-serif;">%s</pre></body></html>`, body)
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

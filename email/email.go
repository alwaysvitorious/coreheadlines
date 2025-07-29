package email

import (
	"crypto/tls"
	"fmt"
	"html"
	"net"
	"net/smtp"
	"strings"
	"time"

	"coreheadlines/tools"
	"coreheadlines/typesPkg"
)

func FormatPost(post typesPkg.MainStruct) string {
	if strings.TrimSpace(post.Title) == "" {
		return ""
	}

	emojis := strings.TrimSpace(tools.GetEmojis(post.Title))

	title := strings.TrimSpace(html.EscapeString(post.Title))
	link := strings.TrimSpace(html.EscapeString(post.Link))

	var parts []string

	if emojis != "" {
		parts = append(parts, emojis)
	}

	if post.Header != "" {
		parts = append(parts, post.Header+":")
	}

	parts = append(parts, title)
	display := strings.Join(parts, " ")

	return fmt.Sprintf(
		`<p style="font-family:monospace; font-size:18px; margin:0;"><a href="%s" style="color:#000000; text-decoration:none;">%s</a></p>`,
		link, display,
	)
}

// Connect via SMTPS, build RFC‑822 email with HTML body and send as single msg
func SendToEmail(
	smtpHost string,
	smtpPort int,
	smtpEmail, smtpPassword,
	bodyHTML string,
) error {
	// Build headers
	subject := "Core Headlines"
	headers := map[string]string{
		"From":         smtpEmail,
		"To":           smtpEmail,
		"Subject":      subject,
		"MIME-Version": "1.0",
		"Content-Type": "text/html; charset=\"utf-8\"",
	}

	// Join headers into msg
	var msgBuilder strings.Builder
	for k, v := range headers {
		msgBuilder.WriteString(fmt.Sprintf("%s: %s\r\n", k, v))
	}
	msgBuilder.WriteString("\r\n") // header/body separator
	msgBuilder.WriteString(bodyHTML)

	rawMsg := []byte(msgBuilder.String())

	// Dial TLS
	addr := fmt.Sprintf("%s:%d", smtpHost, smtpPort)
	dialer := &net.Dialer{Timeout: 30 * time.Second}
	conn, err := tls.DialWithDialer(dialer, "tcp", addr, &tls.Config{
		ServerName: smtpHost,
	})
	if err != nil {
		return fmt.Errorf("smtp dial error: %w", err)
	}
	defer conn.Close()

	// New SMTP client
	client, err := smtp.NewClient(conn, smtpHost)
	if err != nil {
		return fmt.Errorf("creating smtp client: %w", err)
	}
	defer client.Quit()

	// Authenticate
	auth := smtp.PlainAuth("", smtpEmail, smtpPassword, smtpHost)
	if err := client.Auth(auth); err != nil {
		return fmt.Errorf("smtp auth error: %w", err)
	}

	// MAIL FROM
	if err := client.Mail(smtpEmail); err != nil {
		return fmt.Errorf("smtp MAIL FROM error: %w", err)
	}

	// RCPT TO
	if err := client.Rcpt(smtpEmail); err != nil {
		return fmt.Errorf("smtp RCPT TO error: %w", err)
	}

	// DATA
	wc, err := client.Data()
	if err != nil {
		return fmt.Errorf("smtp DATA error: %w", err)
	}
	if _, err := wc.Write(rawMsg); err != nil {
		return fmt.Errorf("writing message data: %w", err)
	}
	if err := wc.Close(); err != nil {
		return fmt.Errorf("closing data writer: %w", err)
	}

	return nil
}

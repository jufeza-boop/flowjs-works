package activities

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"strings"

	"github.com/google/uuid"

	fmodels "flowjs-works/engine/internal/models"
)

// MailActivity implements the `mail` node type.
// config fields:
//
//	action: "send" | "receive"
//
// Send: host, port(int), security("TLS"|"STARTTLS"|"NONE"), auth(map: user, password),
//
//	to([]string), cc([]string), subject, body, content_type("text/plain"|"text/html")
//
// Receive: returns stub {"messages": [], "note": "imap receive not yet implemented"}
type MailActivity struct{}

func (a *MailActivity) Name() string { return "mail" }

func (a *MailActivity) Execute(input map[string]interface{}, config map[string]interface{}, ctx *fmodels.ExecutionContext) (map[string]interface{}, error) {
	action, _ := config["action"].(string)
	if action == "" {
		action = "send"
	}
	switch action {
	case "send":
		return mailSend(config)
	case "receive":
		return map[string]interface{}{
			"messages": []interface{}{},
			"note":     "imap receive not yet implemented",
		}, nil
	default:
		return nil, fmt.Errorf("mail activity: unknown action %q", action)
	}
}

func mailSend(config map[string]interface{}) (map[string]interface{}, error) {
	host, _ := config["host"].(string)
	if host == "" {
		return nil, fmt.Errorf("mail activity: missing required config field 'host'")
	}

	port := 587
	switch v := config["port"].(type) {
	case int:
		port = v
	case float64:
		port = int(v)
	}

	security, _ := config["security"].(string)
	if security == "" {
		security = "STARTTLS"
	}

	contentType, _ := config["content_type"].(string)
	if contentType == "" {
		contentType = "text/plain"
	}

	subject, _ := config["subject"].(string)
	body, _ := config["body"].(string)

	var toList []string
	if to, ok := config["to"].([]interface{}); ok {
		for _, t := range to {
			if s, ok := t.(string); ok {
				toList = append(toList, s)
			}
		}
	}
	var ccList []string
	if cc, ok := config["cc"].([]interface{}); ok {
		for _, c := range cc {
			if s, ok := c.(string); ok {
				ccList = append(ccList, s)
			}
		}
	}

	var fromUser, fromPass string
	// Credentials are read from config["auth"] (nested map) when present, or from
	// flat top-level keys (user, password) injected by the secret resolver.
	getCredential := func(key string) string {
		if authMap, ok := config["auth"].(map[string]interface{}); ok {
			if v, ok := authMap[key].(string); ok {
				return v
			}
		}
		v, _ := config[key].(string)
		return v
	}
	fromUser = getCredential("user")
	fromPass = getCredential("password")

	addr := fmt.Sprintf("%s:%d", host, port)
	headers := fmt.Sprintf("From: %s\r\nTo: %s\r\nCc: %s\r\nSubject: %s\r\nContent-Type: %s\r\n\r\n%s",
		fromUser,
		strings.Join(toList, ", "),
		strings.Join(ccList, ", "),
		subject,
		contentType,
		body,
	)
	msgBytes := []byte(headers)

	var auth smtp.Auth
	if fromUser != "" {
		auth = smtp.PlainAuth("", fromUser, fromPass, host)
	}

	var sendErr error
	switch strings.ToUpper(security) {
	case "TLS":
		tlsCfg := &tls.Config{ServerName: host}
		conn, err := tls.Dial("tcp", addr, tlsCfg)
		if err != nil {
			return nil, fmt.Errorf("mail activity: TLS dial failed: %w", err)
		}
		client, err := smtp.NewClient(conn, host)
		if err != nil {
			return nil, fmt.Errorf("mail activity: SMTP client failed: %w", err)
		}
		defer client.Close()
		if auth != nil {
			if err := client.Auth(auth); err != nil {
				return nil, fmt.Errorf("mail activity: SMTP auth failed: %w", err)
			}
		}
		if err := client.Mail(fromUser); err != nil {
			return nil, fmt.Errorf("mail activity: MAIL FROM failed: %w", err)
		}
		recipients := append(toList, ccList...)
		for _, r := range recipients {
			if err := client.Rcpt(r); err != nil {
				return nil, fmt.Errorf("mail activity: RCPT TO failed: %w", err)
			}
		}
		w, err := client.Data()
		if err != nil {
			return nil, fmt.Errorf("mail activity: DATA failed: %w", err)
		}
		if _, err := w.Write(msgBytes); err != nil {
			return nil, fmt.Errorf("mail activity: write failed: %w", err)
		}
		w.Close()

	case "NONE":
		recipients := append(toList, ccList...)
		sendErr = smtp.SendMail(addr, nil, fromUser, recipients, msgBytes)

	default: // STARTTLS
		tlsCfg := &tls.Config{ServerName: host}
		conn, err := net.Dial("tcp", addr)
		if err != nil {
			return nil, fmt.Errorf("mail activity: dial failed: %w", err)
		}
		client, err := smtp.NewClient(conn, host)
		if err != nil {
			return nil, fmt.Errorf("mail activity: SMTP client failed: %w", err)
		}
		defer client.Close()
		if ok, _ := client.Extension("STARTTLS"); ok {
			if err := client.StartTLS(tlsCfg); err != nil {
				return nil, fmt.Errorf("mail activity: STARTTLS failed: %w", err)
			}
		}
		if auth != nil {
			if err := client.Auth(auth); err != nil {
				return nil, fmt.Errorf("mail activity: SMTP auth failed: %w", err)
			}
		}
		if err := client.Mail(fromUser); err != nil {
			return nil, fmt.Errorf("mail activity: MAIL FROM failed: %w", err)
		}
		recipients := append(toList, ccList...)
		for _, r := range recipients {
			if err := client.Rcpt(r); err != nil {
				return nil, fmt.Errorf("mail activity: RCPT TO failed: %w", err)
			}
		}
		w, err := client.Data()
		if err != nil {
			return nil, fmt.Errorf("mail activity: DATA failed: %w", err)
		}
		if _, err := w.Write(msgBytes); err != nil {
			return nil, fmt.Errorf("mail activity: write failed: %w", err)
		}
		w.Close()
	}

	if sendErr != nil {
		return nil, fmt.Errorf("mail activity: send failed: %w", sendErr)
	}

	messageID := uuid.New().String()
	return map[string]interface{}{
		"sent":       true,
		"message_id": messageID,
	}, nil
}

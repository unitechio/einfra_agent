package enroll

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"os"
	"time"

	"einfra/agent/internal/identity"
	"einfra/agent/internal/logger"
	"einfra/agent/internal/transport"
)

// EnrollRequest is sent to the backend
type EnrollRequest struct {
	NodeID      string `json:"node_id"`
	Fingerprint string `json:"fingerprint"`
	Hostname    string `json:"hostname"`
	Platform    string `json:"platform"`
	Arch        string `json:"arch"`
	Token       string `json:"token"`
}

// EnrollResponse from backend
type EnrollResponse struct {
	Status      string `json:"status"` // "pending", "approved", "rejected"
	Certificate string `json:"certificate,omitempty"`
	CACert      string `json:"ca_cert,omitempty"`
	Message     string `json:"message,omitempty"`
}

// Client handles enrollment
type Client struct {
	transport *transport.Client
	identity  *identity.Identity
	token     string
}

// NewClient creates enrollment client
func NewClient(backendURL, token string, id *identity.Identity) *Client {
	return &Client{
		transport: transport.NewClient(backendURL),
		identity:  id,
		token:     token,
	}
}

// Enroll attempts to enroll with the backend
func (c *Client) Enroll(ctx context.Context) (*EnrollResponse, error) {
	req := EnrollRequest{
		NodeID:      c.identity.NodeID,
		Fingerprint: c.identity.Fingerprint,
		Hostname:    c.identity.Hostname,
		Platform:    c.identity.Platform,
		Arch:        c.identity.Arch,
		Token:       c.token,
	}

	logger.Info().
		Str("node_id", req.NodeID).
		Str("hostname", req.Hostname).
		Msg("Sending enrollment request")

	resp, err := c.transport.Post(ctx, "/api/v1/agent/enroll", req)
	if err != nil {
		return nil, fmt.Errorf("enrollment request failed: %w", err)
	}

	var enrollResp EnrollResponse
	if err := transport.DecodeJSON(resp, &enrollResp); err != nil {
		return nil, err
	}

	logger.Info().
		Str("status", enrollResp.Status).
		Str("message", enrollResp.Message).
		Msg("Enrollment response received")

	return &enrollResp, nil
}

// WaitForApproval polls until enrollment is approved
func (c *Client) WaitForApproval(ctx context.Context, interval time.Duration) (*EnrollResponse, error) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			resp, err := c.Enroll(ctx)
			if err != nil {
				logger.Warn().Err(err).Msg("Enrollment check failed, retrying...")
				continue
			}

			if resp.Status == "approved" {
				return resp, nil
			} else if resp.Status == "rejected" {
				return nil, fmt.Errorf("enrollment rejected: %s", resp.Message)
			}

			logger.Info().Msg("Enrollment pending, waiting for approval...")
		}
	}
}

// SaveCertificates saves the received certificates to disk
func SaveCertificates(certPEM, caPEM, certPath, caPath, keyPath string, privateKey *rsa.PrivateKey) error {
	// Save certificate
	if err := os.WriteFile(certPath, []byte(certPEM), 0600); err != nil {
		return fmt.Errorf("failed to save certificate: %w", err)
	}

	// Save CA certificate
	if err := os.WriteFile(caPath, []byte(caPEM), 0644); err != nil {
		return fmt.Errorf("failed to save CA certificate: %w", err)
	}

	// Save private key
	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	})
	if err := os.WriteFile(keyPath, keyPEM, 0600); err != nil {
		return fmt.Errorf("failed to save private key: %w", err)
	}

	logger.Info().
		Str("cert_path", certPath).
		Str("ca_path", caPath).
		Msg("Certificates saved successfully")

	return nil
}

// GenerateCSR generates a certificate signing request
func GenerateCSR(id *identity.Identity) (*rsa.PrivateKey, []byte, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}

	template := x509.CertificateRequest{
		Subject: pkix.Name{
			CommonName:   id.NodeID,
			Organization: []string{"EINFRA"},
		},
		DNSNames: []string{id.Hostname},
	}

	csrBytes, err := x509.CreateCertificateRequest(rand.Reader, &template, privateKey)
	if err != nil {
		return nil, nil, err
	}

	return privateKey, csrBytes, nil
}

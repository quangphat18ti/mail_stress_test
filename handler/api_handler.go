package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"mail-stress-test/models"
)

// APIHandler implements MailHandler by calling a Fiber API
type APIHandler struct {
	baseURL    string
	httpClient *http.Client
}

// NewAPIHandler creates a new APIHandler
func NewAPIHandler(baseURL string) *APIHandler {
	return &APIHandler{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// CreateMail creates a mail via API call
func (h *APIHandler) CreateMail(ctx context.Context, req *models.MailRequest) error {
	body, err := json.Marshal(req)
	if err != nil {
		return err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", h.baseURL+"/api/mails", bytes.NewBuffer(body))
	if err != nil {
		return err
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := h.httpClient.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error: status code %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

// ListMails retrieves mails via API call
func (h *APIHandler) ListMails(ctx context.Context, req *models.ListMailsRequest) ([]*models.Mail, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", h.baseURL+"/api/mails/list", bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := h.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error: status code %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	var mails []*models.Mail
	if err := json.NewDecoder(resp.Body).Decode(&mails); err != nil {
		return nil, err
	}

	return mails, nil
}

// SearchMails searches for mails via API call
func (h *APIHandler) SearchMails(ctx context.Context, req *models.SearchMailsRequest) ([]*models.Mail, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", h.baseURL+"/api/mails/search", bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := h.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error: status code %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	var mails []*models.Mail
	if err := json.NewDecoder(resp.Body).Decode(&mails); err != nil {
		return nil, err
	}

	return mails, nil
}

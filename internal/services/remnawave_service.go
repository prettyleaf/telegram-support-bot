package services

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"remnafamily/internal/config"
)

var ErrRemnawaveNotFound = errors.New("remnawave resource not found")

// RemnawaveService provides a small HTTP client for the panel operations used by the bot.
type RemnawaveService struct {
	baseURL string
	mode    string
	headers map[string]string
	client  *http.Client
	token   string
}

type RemnawaveUser struct {
	UUID            string `json:"uuid"`
	Username        string `json:"username"`
	TelegramID      *int64 `json:"telegramId"`
	HWIDDeviceLimit *int   `json:"hwidDeviceLimit"`
}

type RemnawaveDevice struct {
	HWID        string    `json:"hwid"`
	UserUUID    string    `json:"userUuid"`
	Platform    *string   `json:"platform"`
	OSVersion   *string   `json:"osVersion"`
	DeviceModel *string   `json:"deviceModel"`
	UserAgent   *string   `json:"userAgent"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type remnawaveUsersResponse struct {
	Response []RemnawaveUser `json:"response"`
}

type remnawaveDevicesResponse struct {
	Response struct {
		Devices []RemnawaveDevice `json:"devices"`
		Total   int               `json:"total"`
	} `json:"response"`
}

type remnawaveErrorResponse struct {
	Message    string `json:"message"`
	StatusCode int    `json:"statusCode"`
}

// NewRemnawaveService creates an optional panel client. A nil result means the env is incomplete.
func NewRemnawaveService(cfg *config.Config) *RemnawaveService {
	if cfg == nil || !cfg.HasRemnawave() {
		return nil
	}

	return &RemnawaveService{
		baseURL: strings.TrimRight(cfg.RemnawaveURL, "/"),
		mode:    strings.ToLower(strings.TrimSpace(cfg.RemnawaveMode)),
		headers: cfg.RemnawaveHeaders,
		token:   cfg.RemnawaveToken,
		client: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// GetUsersByTelegramID returns all panel users tied to the provided Telegram ID.
func (s *RemnawaveService) GetUsersByTelegramID(ctx context.Context, telegramID int64) ([]RemnawaveUser, error) {
	var response remnawaveUsersResponse
	err := s.doJSON(ctx, http.MethodGet, fmt.Sprintf("/api/users/by-telegram-id/%d", telegramID), nil, &response)
	if err != nil {
		if errors.Is(err, ErrRemnawaveNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return response.Response, nil
}

// GetUserDevices loads the HWID devices currently registered for a panel user.
func (s *RemnawaveService) GetUserDevices(ctx context.Context, userUUID string) ([]RemnawaveDevice, error) {
	var response remnawaveDevicesResponse
	if err := s.doJSON(ctx, http.MethodGet, "/api/hwid/devices/"+userUUID, nil, &response); err != nil {
		return nil, err
	}
	return response.Response.Devices, nil
}

// DeleteUserDevice removes a single HWID from the selected panel user.
func (s *RemnawaveService) DeleteUserDevice(ctx context.Context, userUUID, hwid string) error {
	request := map[string]string{
		"userUuid": userUUID,
		"hwid":     hwid,
	}
	return s.doJSON(ctx, http.MethodPost, "/api/hwid/devices/delete", request, nil)
}

// DeleteAllUserDevices removes all HWID records for the selected panel user.
func (s *RemnawaveService) DeleteAllUserDevices(ctx context.Context, userUUID string) error {
	request := map[string]string{
		"userUuid": userUUID,
	}
	return s.doJSON(ctx, http.MethodPost, "/api/hwid/devices/delete-all", request, nil)
}

func (s *RemnawaveService) doJSON(ctx context.Context, method, path string, requestBody interface{}, responseBody interface{}) error {
	if s == nil {
		return errors.New("remnawave service is not configured")
	}

	var body io.Reader
	if requestBody != nil {
		payload, err := json.Marshal(requestBody)
		if err != nil {
			return fmt.Errorf("marshal remnawave request: %w", err)
		}
		body = bytes.NewReader(payload)
	}

	req, err := http.NewRequestWithContext(ctx, method, s.baseURL+path, body)
	if err != nil {
		return fmt.Errorf("create remnawave request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.token)
	if requestBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if s.mode == "local" {
		req.Header.Set("X-Forwarded-For", "127.0.0.1")
		req.Header.Set("X-Forwarded-Proto", "https")
	}
	for key, value := range s.headers {
		req.Header.Set(key, value)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("execute remnawave request: %w", err)
	}
	defer resp.Body.Close()

	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read remnawave response: %w", err)
	}

	if resp.StatusCode == http.StatusNotFound {
		return wrapRemnawaveError(ErrRemnawaveNotFound, payload)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return wrapRemnawaveError(fmt.Errorf("remnawave request failed with status %d", resp.StatusCode), payload)
	}
	if responseBody == nil || len(payload) == 0 {
		return nil
	}
	if err := json.Unmarshal(payload, responseBody); err != nil {
		return fmt.Errorf("decode remnawave response: %w", err)
	}
	return nil
}

func wrapRemnawaveError(base error, payload []byte) error {
	var response remnawaveErrorResponse
	if err := json.Unmarshal(payload, &response); err == nil && strings.TrimSpace(response.Message) != "" {
		return fmt.Errorf("%w: %s", base, response.Message)
	}

	message := strings.TrimSpace(string(payload))
	if message == "" {
		return base
	}
	return fmt.Errorf("%w: %s", base, message)
}

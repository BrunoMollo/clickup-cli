package clickup

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const DefaultBaseURL = "https://api.clickup.com/api/v2"

type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
	maxRetries int
}

type HTTPError struct {
	StatusCode int
	Code       string
	Message    string
}

func (e *HTTPError) Error() string {
	if e.Code != "" {
		return fmt.Sprintf("ClickUp respondió HTTP %d (%s): %s", e.StatusCode, e.Code, e.Message)
	}
	return fmt.Sprintf("ClickUp respondió HTTP %d: %s", e.StatusCode, e.Message)
}

func NewClient(token string) *Client {
	return NewClientWithOptions(token, DefaultBaseURL, &http.Client{Timeout: 15 * time.Second})
}

func NewClientWithOptions(token, baseURL string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 15 * time.Second}
	}
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		token:      token,
		httpClient: httpClient,
		maxRetries: 2,
	}
}

func (c *Client) GetView(ctx context.Context, viewID string) (View, error) {
	var response viewResponse
	if err := c.get(ctx, "/view/"+url.PathEscape(viewID), nil, &response); err != nil {
		return View{}, err
	}
	if response.View.ID == "" {
		return View{}, errors.New("ClickUp devolvió una vista sin ID")
	}
	return response.View, nil
}

func (c *Client) GetList(ctx context.Context, listID string) (List, error) {
	var response List
	if err := c.get(ctx, "/list/"+url.PathEscape(listID), nil, &response); err != nil {
		return List{}, err
	}
	return response, nil
}

func (c *Client) GetFolderLists(ctx context.Context, folderID string) ([]List, error) {
	query := url.Values{"archived": []string{"false"}}
	var response listsResponse
	if err := c.get(ctx, "/folder/"+url.PathEscape(folderID)+"/list", query, &response); err != nil {
		return nil, err
	}
	return response.Lists, nil
}

func (c *Client) GetListTasks(ctx context.Context, listID string) ([]Task, error) {
	var all []Task
	for page := 0; page < 10_000; page++ {
		query := url.Values{
			"page":           []string{strconv.Itoa(page)},
			"archived":       []string{"false"},
			"include_closed": []string{"true"},
			"subtasks":       []string{"true"},
			"include_timl":   []string{"true"},
		}
		var response tasksResponse
		if err := c.get(ctx, "/list/"+url.PathEscape(listID)+"/task", query, &response); err != nil {
			return nil, fmt.Errorf("cargar página %d de lista %s: %w", page, listID, err)
		}
		all = append(all, response.Tasks...)
		if response.LastPage || len(response.Tasks) == 0 {
			return all, nil
		}
	}
	return nil, fmt.Errorf("lista %s superó límite de paginación", listID)
}

func (c *Client) get(ctx context.Context, path string, query url.Values, target any) error {
	endpoint := c.baseURL + path
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}

	for attempt := 0; ; attempt++ {
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return err
		}
		request.Header.Set("Authorization", c.token)
		request.Header.Set("Accept", "application/json")

		response, err := c.httpClient.Do(request)
		if err != nil {
			return fmt.Errorf("consultar ClickUp: %w", err)
		}

		if response.StatusCode >= 200 && response.StatusCode < 300 {
			defer response.Body.Close()
			decoder := json.NewDecoder(io.LimitReader(response.Body, 16<<20))
			if err := decoder.Decode(target); err != nil {
				return fmt.Errorf("decodificar respuesta de ClickUp: %w", err)
			}
			return nil
		}

		httpErr := decodeHTTPError(response)
		response.Body.Close()
		if attempt >= c.maxRetries || !retryable(response.StatusCode) {
			return httpErr
		}
		if err := sleepContext(ctx, retryDelay(response, attempt)); err != nil {
			return err
		}
	}
}

func decodeHTTPError(response *http.Response) error {
	var payload struct {
		Code    string `json:"ECODE"`
		Message string `json:"err"`
	}
	_ = json.NewDecoder(io.LimitReader(response.Body, 64<<10)).Decode(&payload)
	if payload.Message == "" {
		payload.Message = http.StatusText(response.StatusCode)
	}
	return &HTTPError{StatusCode: response.StatusCode, Code: payload.Code, Message: payload.Message}
}

func retryable(status int) bool {
	return status == http.StatusTooManyRequests || status >= 500
}

func retryDelay(response *http.Response, attempt int) time.Duration {
	if raw := response.Header.Get("Retry-After"); raw != "" {
		if seconds, err := strconv.Atoi(raw); err == nil {
			return minDuration(time.Duration(seconds)*time.Second, 5*time.Second)
		}
	}
	if raw := response.Header.Get("X-RateLimit-Reset"); raw != "" {
		if unix, err := strconv.ParseInt(raw, 10, 64); err == nil {
			return minDuration(time.Until(time.Unix(unix, 0)), 5*time.Second)
		}
	}
	delay := 250 * time.Millisecond * time.Duration(1<<attempt)
	return minDuration(delay, 5*time.Second)
}

func minDuration(value, maximum time.Duration) time.Duration {
	if value < 0 {
		return 0
	}
	if value > maximum {
		return maximum
	}
	return value
}

func sleepContext(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

package gen

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const duckcodingDefaultBaseURL = "https://api.openai.com/v1"

type Duckcoding struct {
	APIKey  string
	Model   string
	BaseURL string
	HTTP    *http.Client
}

func NewDuckcoding(apiKey, model string) *Duckcoding {
	if model == "" {
		model = DefaultModelFor(ProviderDuckcoding)
	}
	return &Duckcoding{
		APIKey:  apiKey,
		Model:   model,
		BaseURL: duckcodingBaseURL(),
		HTTP:    &http.Client{Timeout: 600 * time.Second},
	}
}

type duckcodingRequest struct {
	Model          string   `json:"model"`
	Prompt         string   `json:"prompt"`
	Images         []string `json:"image,omitempty"`
	AspectRatio    string   `json:"aspect_ratio,omitempty"`
	N              int      `json:"n"`
	Size           string   `json:"size"`
	ResponseFormat string   `json:"response_format"`
}

type duckcodingResponse struct {
	Data []struct {
		B64JSON string `json:"b64_json"`
		URL     string `json:"url"`
	} `json:"data"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (c *Duckcoding) GenerateImage(ctx context.Context, prompt string, refImages [][]byte, aspectRatio string) ([]byte, error) {
	if c.APIKey == "" {
		return nil, errors.New("Duckcoding API 키가 설정되지 않았습니다. DUCKCODING_API_KEY를 입력해 주세요")
	}
	reqData := duckcodingRequest{
		Model:          c.Model,
		Prompt:         prompt + "\n\n" + aspectHint(aspectRatio),
		AspectRatio:    aspectRatio,
		N:              1,
		Size:           duckcodingSizeFor(aspectRatio),
		ResponseFormat: "b64_json",
	}
	for _, img := range refImages {
		reqData.Images = append(reqData.Images, "data:image/png;base64,"+base64.StdEncoding.EncodeToString(img))
	}
	body, err := json.Marshal(reqData)
	if err != nil {
		return nil, fmt.Errorf("요청 직렬화 실패: %w", err)
	}

	var lastErr error
	backoff := 2 * time.Second
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
			backoff *= 2
		}
		img, retryable, err := c.doRequest(ctx, body)
		if err == nil {
			return img, nil
		}
		lastErr = err
		if !retryable {
			return nil, err
		}
	}
	return nil, lastErr
}

func (c *Duckcoding) doRequest(ctx context.Context, body []byte) (img []byte, retryable bool, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(c.BaseURL, "/")+"/images/generations", bytes.NewReader(body))
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.APIKey)

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, true, fmt.Errorf("네트워크 오류: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	if err != nil {
		return nil, true, fmt.Errorf("응답 읽기 실패: %w", err)
	}

	var parsed duckcodingResponse
	_ = json.Unmarshal(respBytes, &parsed)
	if resp.StatusCode != http.StatusOK {
		retryable = resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= http.StatusInternalServerError
		if parsed.Error != nil && parsed.Error.Message != "" {
			return nil, retryable, fmt.Errorf("Duckcoding 오류 (%d): %s", resp.StatusCode, parsed.Error.Message)
		}
		return nil, retryable, fmt.Errorf("Duckcoding 오류 (HTTP %d)", resp.StatusCode)
	}
	if parsed.Error != nil && parsed.Error.Message != "" {
		return nil, true, fmt.Errorf("Duckcoding 오류: %s", parsed.Error.Message)
	}
	if len(parsed.Data) == 0 {
		return nil, true, errors.New("응답에 이미지가 없습니다")
	}
	first := parsed.Data[0]
	if first.B64JSON != "" {
		data, err := base64.StdEncoding.DecodeString(first.B64JSON)
		if err != nil {
			return nil, false, fmt.Errorf("이미지 디코딩 실패: %w", err)
		}
		return data, false, nil
	}
	if first.URL != "" {
		data, err := decodeDataOrDownload(c.HTTP, first.URL)
		if err != nil {
			return nil, false, err
		}
		return data, false, nil
	}
	return nil, true, errors.New("응답에 이미지가 없습니다")
}

func (c *Duckcoding) ValidateKey(_ context.Context) error {
	if len(strings.TrimSpace(c.APIKey)) < 10 {
		return errors.New("API 키가 너무 짧습니다")
	}
	return nil
}

func duckcodingBaseURL() string {
	if v := strings.TrimSpace(os.Getenv("DUCKCODING_BASE_URL")); v != "" {
		return v
	}
	if v := strings.TrimSpace(os.Getenv("IMAGE_PROVIDER_BASE_URL")); v != "" {
		return v
	}
	return duckcodingDefaultBaseURL
}

func duckcodingSizeFor(aspectRatio string) string {
	switch aspectRatio {
	case "21:9":
		return "1536x1024"
	case "16:9":
		return "1536x1024"
	default:
		return "1024x1024"
	}
}

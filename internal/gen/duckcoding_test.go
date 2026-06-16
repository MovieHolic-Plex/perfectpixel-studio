package gen

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDuckcodingGenerateImageSendsReferences_whenRefsProvided(t *testing.T) {
	payload := []byte{0x89, 'P', 'N', 'G', 4, 5, 6}
	var got duckcodingRequest
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("요청 파싱 실패: %v", err)
		}
		_, _ = w.Write([]byte(`{"data":[{"b64_json":"` + base64.StdEncoding.EncodeToString(payload) + `"}]}`))
	}))
	defer srv.Close()

	client := NewDuckcoding("duck-key-12345", "")
	client.BaseURL = srv.URL
	client.HTTP = srv.Client()

	img, err := client.GenerateImage(context.Background(), "draw walk", [][]byte{fakePNG}, "21:9")

	if err != nil {
		t.Fatalf("생성 실패: %v", err)
	}
	if string(img) != string(payload) {
		t.Fatalf("이미지 바이트 불일치: %v", img)
	}
	if gotAuth != "Bearer duck-key-12345" {
		t.Fatalf("인증 헤더 오류: %s", gotAuth)
	}
	if got.Model != "gpt-image-2" || got.N != 1 || got.ResponseFormat != "b64_json" {
		t.Fatalf("기본 요청 필드 오류: %+v", got)
	}
	if got.AspectRatio != "21:9" || got.Size != "1536x1024" || !strings.Contains(got.Prompt, "21:9") {
		t.Fatalf("종횡비 요청 오류: %+v", got)
	}
	if len(got.Images) != 1 || !strings.HasPrefix(got.Images[0], "data:image/png;base64,") {
		t.Fatalf("참조 이미지 누락: %+v", got.Images)
	}
}

func TestDuckcodingGenerateImageReturnsError_whenProviderRejects(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"message":"bad prompt"}}`))
	}))
	defer srv.Close()

	client := NewDuckcoding("duck-key-12345", "")
	client.BaseURL = srv.URL
	client.HTTP = srv.Client()

	_, err := client.GenerateImage(context.Background(), "bad", nil, "1:1")

	if err == nil || !strings.Contains(err.Error(), "bad prompt") {
		t.Fatalf("provider 오류가 전달되어야 합니다: %v", err)
	}
}

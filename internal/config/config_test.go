package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseEnvFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env.local")
	content := "# 주석\n" +
		"FAL_KEY=abc123:secret\n" +
		"export OPENROUTER_API_KEY=\"sk-or-test\"\n" +
		"GEMINI_API_KEY='AIza-test'\n" +
		"DUCKCODING_API_KEY=duck-test\n" +
		"DUCKCODING_IMAGE_MODEL=gpt-image-2\n" +
		"INVALID_LINE\n" +
		"EMPTY=\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	out := map[string]string{}
	parseEnvFile(path, out)

	if out["FAL_KEY"] != "abc123:secret" {
		t.Fatalf("FAL_KEY 파싱 실패: %q", out["FAL_KEY"])
	}
	if out["OPENROUTER_API_KEY"] != "sk-or-test" {
		t.Fatalf("export + 따옴표 파싱 실패: %q", out["OPENROUTER_API_KEY"])
	}
	if out["GEMINI_API_KEY"] != "AIza-test" {
		t.Fatalf("작은따옴표 파싱 실패: %q", out["GEMINI_API_KEY"])
	}
	if out["DUCKCODING_API_KEY"] != "duck-test" || out["DUCKCODING_IMAGE_MODEL"] != "gpt-image-2" {
		t.Fatalf("Duckcoding env 파싱 실패: %v", out)
	}
	if _, ok := out["EMPTY"]; ok {
		t.Fatal("빈 값은 무시되어야 합니다")
	}
}

func TestSettingsCfg(t *testing.T) {
	s := Settings{
		Gemini:     ProviderCfg{APIKey: "g"},
		OpenRouter: ProviderCfg{APIKey: "o"},
		Fal:        ProviderCfg{APIKey: "f"},
		Duckcoding: ProviderCfg{APIKey: "d"},
	}
	if s.Cfg("openrouter").APIKey != "o" || s.Cfg("fal").APIKey != "f" || s.Cfg("duckcoding").APIKey != "d" {
		t.Fatal("프로바이더별 설정 매핑 오류")
	}
	// 알 수 없는 프로바이더는 gemini로 폴백
	if s.Cfg("unknown").APIKey != "g" {
		t.Fatal("기본 폴백 오류")
	}
	// 포인터 반환이므로 수정이 반영되어야 함
	s.Cfg("fal").Model = "m"
	if s.Fal.Model != "m" {
		t.Fatal("Cfg는 포인터를 반환해야 합니다")
	}
}

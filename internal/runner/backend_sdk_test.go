package runner

import "testing"

func TestProviderFromEnv(t *testing.T) {
	values := map[string]string{
		"COPILOT_PROVIDER_BASE_URL":          "https://provider.example/v1",
		"COPILOT_PROVIDER_API_KEY":           "test-key",
		"COPILOT_MODEL":                      "glm-5.2",
		"COPILOT_PROVIDER_WIRE_API":          "completions",
		"COPILOT_PROVIDER_MAX_PROMPT_TOKENS": "1000000",
	}
	provider, model, err := providerFromEnv("auto", func(name string) string { return values[name] })
	if err != nil {
		t.Fatal(err)
	}
	if model != "glm-5.2" || provider == nil || provider.BaseURL != values["COPILOT_PROVIDER_BASE_URL"] {
		t.Fatalf("provider = %#v, model = %q", provider, model)
	}
	if provider.Type != "openai" || provider.WireAPI != "completions" || provider.MaxPromptTokens != 1000000 {
		t.Fatalf("provider = %#v", provider)
	}
}

func TestProviderFromEnvRejectsMissingModelAndInvalidLimits(t *testing.T) {
	_, _, err := providerFromEnv("auto", func(name string) string {
		if name == "COPILOT_PROVIDER_BASE_URL" {
			return "https://provider.example/v1"
		}
		return ""
	})
	if err == nil {
		t.Fatal("expected missing model error")
	}
	_, _, err = providerFromEnv("model", func(name string) string {
		if name == "COPILOT_PROVIDER_BASE_URL" {
			return "https://provider.example/v1"
		}
		if name == "COPILOT_PROVIDER_MAX_OUTPUT_TOKENS" {
			return "zero"
		}
		return ""
	})
	if err == nil {
		t.Fatal("expected invalid token limit error")
	}
}

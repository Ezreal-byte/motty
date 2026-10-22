package workbench

import (
	"testing"

	"motty/internal/ai/store"
)

func TestSavedProviderShowsTheMarkOfTheServiceItsAddressPointsAt(t *testing.T) {
	for _, test := range []struct {
		name     string
		provider store.Provider
		want     string
	}{
		{"official Anthropic, no address", store.Provider{Name: "work", Kind: store.KindAnthropic}, "Claude"},
		{"official OpenAI, no address", store.Provider{Name: "x", Kind: store.KindOpenAI}, "OpenAI"},
		{"renamed provider keeps its mark", store.Provider{Name: "my deepseek", Kind: store.KindOpenAI, BaseURL: "https://api.deepseek.com/v1"}, "DeepSeek"},
		{"path and trailing slash do not matter", store.Provider{Kind: store.KindOpenAI, BaseURL: "https://api.deepseek.com/"}, "DeepSeek"},
		{"the other region of a vendor", store.Provider{Kind: store.KindOpenAI, BaseURL: "https://open.bigmodel.cn/api/paas/v4"}, "Zhipu"},
		{"host case", store.Provider{Kind: store.KindOpenAI, BaseURL: "HTTPS://API.MINIMAXI.COM/v1"}, "MiniMax"},
		{"no scheme", store.Provider{Kind: store.KindOpenAI, BaseURL: "api.moonshot.cn/v1"}, "Kimi"},
		{"Ollama by loopback address", store.Provider{Kind: store.KindOpenAI, BaseURL: "http://127.0.0.1:11434/v1"}, "Ollama"},
		{"another port on localhost is not Ollama", store.Provider{Kind: store.KindOpenAI, BaseURL: "http://localhost:8080/v1"}, ""},
		{"a proxy is custom", store.Provider{Name: "openrouter", Kind: store.KindOpenAI, BaseURL: "https://openrouter.ai/api/v1"}, ""},
		{"an address that does not parse is custom", store.Provider{Kind: store.KindOpenAI, BaseURL: "http://[::1"}, ""},
	} {
		if got := aiPresets[aiPresetOf(test.provider)].label; got != test.want {
			t.Errorf("%s: preset %q, want %q", test.name, got, test.want)
		}
	}
	if len(aiProviderIcon(store.Provider{Kind: store.KindAnthropic}).Layers) == 0 && len(aiProviderIcon(store.Provider{Kind: store.KindAnthropic}).Commands) == 0 {
		t.Fatal("the official Anthropic provider has no icon")
	}
}

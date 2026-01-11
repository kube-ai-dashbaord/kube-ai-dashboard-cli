package ui

import (
	"github.com/kube-ai-dashbaord/kube-ai-dashboard-cli/pkg/config"
	"github.com/rivo/tview"
)

type Settings struct {
	Form   *tview.Form
	OnSave func(cfg *config.LLMConfig)
}

func NewSettings(cfg *config.LLMConfig, onSave func(*config.LLMConfig), onCancel func()) *Settings {
	s := &Settings{
		Form:   tview.NewForm(),
		OnSave: onSave,
	}

	s.Form.AddDropDown("Provider", []string{"openai", "ollama"}, 0, nil).
		AddInputField("Model", cfg.Model, 20, nil, nil).
		AddInputField("Endpoint", cfg.Endpoint, 40, nil, nil).
		AddPasswordField("API Key", cfg.APIKey, 40, '*', nil).
		AddButton("Save", func() {
			_, provider := s.Form.GetFormItemByLabel("Provider").(*tview.DropDown).GetCurrentOption()
			model := s.Form.GetFormItemByLabel("Model").(*tview.InputField).GetText()
			endpoint := s.Form.GetFormItemByLabel("Endpoint").(*tview.InputField).GetText()
			apiKey := s.Form.GetFormItemByLabel("API Key").(*tview.InputField).GetText()

			newCfg := &config.LLMConfig{
				Provider: provider,
				Model:    model,
				Endpoint: endpoint,
				APIKey:   apiKey,
			}
			if s.OnSave != nil {
				s.OnSave(newCfg)
			}
		}).
		AddButton("Cancel", onCancel)

	s.Form.SetBorder(true).SetTitle(" LLM Settings ")
	return s
}

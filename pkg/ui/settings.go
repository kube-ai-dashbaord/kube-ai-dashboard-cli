package ui

import (
	"github.com/kube-ai-dashbaord/kube-ai-dashboard-cli/pkg/config"
	"github.com/rivo/tview"
)

type Settings struct {
	Form   *tview.Form
	OnSave func(cfg *config.LLMConfig)
}

func NewSettings(cfg *config.Config, onSave func(*config.Config), onCancel func()) *Settings {
	s := &Settings{
		Form: tview.NewForm(),
	}

	langs := []string{"en", "ko", "zh", "ja"}
	langIdx := 0
	for i, l := range langs {
		if l == cfg.Language {
			langIdx = i
			break
		}
	}

	s.Form.AddDropDown("Language", langs, langIdx, nil).
		AddDropDown("Provider", []string{"openai", "ollama"}, 0, nil).
		AddInputField("Model", cfg.LLM.Model, 20, nil, nil).
		AddInputField("Endpoint", cfg.LLM.Endpoint, 40, nil, nil).
		AddPasswordField("API Key", cfg.LLM.APIKey, 40, '*', nil).
		AddInputField("Report Path", cfg.ReportPath, 40, nil, nil).
		AddCheckbox("Enable Audit Log", cfg.EnableAudit, nil).
		AddCheckbox("Beginner Mode", cfg.BeginnerMode, nil).
		AddButton("Save", func() {
			_, lang := s.Form.GetFormItemByLabel("Language").(*tview.DropDown).GetCurrentOption()
			_, provider := s.Form.GetFormItemByLabel("Provider").(*tview.DropDown).GetCurrentOption()
			model := s.Form.GetFormItemByLabel("Model").(*tview.InputField).GetText()
			endpoint := s.Form.GetFormItemByLabel("Endpoint").(*tview.InputField).GetText()
			apiKey := s.Form.GetFormItemByLabel("API Key").(*tview.InputField).GetText()
			reportPath := s.Form.GetFormItemByLabel("Report Path").(*tview.InputField).GetText()
			enableAudit := s.Form.GetFormItemByLabel("Enable Audit Log").(*tview.Checkbox).IsChecked()
			beginnerMode := s.Form.GetFormItemByLabel("Beginner Mode").(*tview.Checkbox).IsChecked()

			newCfg := &config.Config{
				Language: lang,
				LLM: config.LLMConfig{
					Provider: provider,
					Model:    model,
					Endpoint: endpoint,
					APIKey:   apiKey,
				},
				ReportPath:   reportPath,
				EnableAudit:  enableAudit,
				BeginnerMode: beginnerMode,
			}
			if onSave != nil {
				onSave(newCfg)
			}
		}).
		AddButton("Cancel", onCancel)

	s.Form.SetBorder(true).SetTitle(" LLM Settings ")
	return s
}

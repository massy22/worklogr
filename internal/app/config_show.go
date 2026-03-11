package app

type ConfigShowRequest struct {
	ConfigPath string
}

type ConfigShowResult struct {
	DatabasePath string
	Timezone     string
	Services     []ConfiguredService
}

type ConfigShowUsecase struct {
	runtime *appRuntime
}

func NewConfigShowUsecase() *ConfigShowUsecase {
	return &ConfigShowUsecase{
		runtime: newAppRuntime(),
	}
}

func (u *ConfigShowUsecase) Run(request ConfigShowRequest) (*ConfigShowResult, error) {
	cfg, err := u.runtime.loadAppConfig(request.ConfigPath)
	if err != nil {
		return nil, err
	}

	services := make([]ConfiguredService, 0, len(appServiceDefinitions))
	for _, service := range appServiceDefinitions {
		configuredService := ConfiguredService{
			Name:        service.Name,
			DisplayName: service.DisplayName,
		}

		switch service.Name {
		case "slack":
			configuredService.Enabled = cfg.Slack.Enabled
			configuredService.Configured = cfg.Slack.ClientID != "" && cfg.Slack.ClientSecret != ""
		case "github":
			configuredService.Enabled = cfg.GitHub.Enabled
			configuredService.Configured = cfg.GitHub.ClientID != "" && cfg.GitHub.ClientSecret != ""
		case "google_calendar":
			configuredService.Enabled = cfg.GoogleCal.Enabled
			configuredService.Configured = cfg.GoogleCal.ClientID != "" && cfg.GoogleCal.ClientSecret != ""
		}

		services = append(services, configuredService)
	}

	return &ConfigShowResult{
		DatabasePath: cfg.DatabasePath,
		Timezone:     cfg.Timezone,
		Services:     services,
	}, nil
}

package app

type ConfigShowRequest struct {
	ConfigPath string
}

type ConfigShowService struct {
	DisplayName string
	Enabled     bool
	Configured  bool
}

type ConfigShowResult struct {
	DatabasePath string
	Timezone     string
	Services     []ConfigShowService
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

	return &ConfigShowResult{
		DatabasePath: cfg.DatabasePath,
		Timezone:     cfg.Timezone,
		Services: []ConfigShowService{
			{
				DisplayName: "Slack",
				Enabled:     cfg.Slack.Enabled,
				Configured:  cfg.Slack.ClientID != "" && cfg.Slack.ClientSecret != "",
			},
			{
				DisplayName: "GitHub",
				Enabled:     cfg.GitHub.Enabled,
				Configured:  cfg.GitHub.ClientID != "" && cfg.GitHub.ClientSecret != "",
			},
			{
				DisplayName: "Google Calendar",
				Enabled:     cfg.GoogleCal.Enabled,
				Configured:  cfg.GoogleCal.ClientID != "" && cfg.GoogleCal.ClientSecret != "",
			},
		},
	}, nil
}

package config;

type (
	Application struct {
		Server		ServerConfig		`yaml:"server"`
		Database	DatabaseConfig		`yaml:"database"`
		SerialPort	SerialPortConfig	`yaml:"serial_port"`
		Rating		RatingConfig		`yaml:"rating"`
		Images		ImageConfig		`yaml:images"`
		Timing		TimingConfig		`yaml"timings"`
	}

	ServerConfig struct {
		Address		string			`yaml:"address"`
		Port		int			`yaml:"port"`
	}

	DatabaseConfig struct {
		Username	string			`yaml:"username"`
		Password	string			`yaml:"password"`
		Database	string			`yaml:"database"`
		Parameters	[]string		`yaml:"parameters"`
	}

	SerialPortConfig struct {
		DeviceFile	string			`yaml:"device_file"`
	}

	RatingConfig struct {
		DefaultPoints	int			`yaml:"default_points"`
		KFactor		float64			`yaml:"k_factor"`
	}

	ImageConfig struct {
		Path		string			`yaml:"path"`
	}

	TimingConfig struct {
		DuelTimeout	int			`yaml:"duel"`
		Leaderboard	int			`yaml:"leaderboard"`
		SplashScreen	int			`yaml:"splash_screen"`
	}
)

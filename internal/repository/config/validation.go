package config

import (
	"strings"
	"net/url"
	"os"
)

func setConfigurationDefaults(c *Application) {
	if c.Server.Port == 0 {
		c.Server.Port = 5000
	}
	if c.Rating.DefaultPoints == 0 {
		c.Rating.DefaultPoints = 800
	}
	if c.Rating.KFactor == 0 {
		c.Rating.KFactor = 16
	}
	if c.Timing.DuelTimeout == 0 {
		c.Timing.DuelTimeout = 20
	}
	if c.Timing.Leaderboard == 0 {
		c.Timing.Leaderboard = 15
	}
	if c.Timing.SplashScreen == 0 {
		c.Timing.SplashScreen = 15
	}
}

const (
	envDbPassword = "ARTBATTLE_SECRET_DB_PASSWORD"
)

func applyEnvVarOverrides(c *Application) {
	dbPassword := os.Getenv(envDbPassword);
	if dbPassword != "" {
		c.Database.Password = dbPassword;
	}
}

func validateServerConfiguration(errs url.Values, c ServerConfig) {
	if c.Port < 1 || c.Port > 65535 {
		errs.Add("server.port", "must be a number between 1 and 65535")
	}
}

func validateDatabaseConfiguration(errs url.Values, c DatabaseConfig) {
	if len(c.Username) < 1 || len(c.Username) > 256 {
		errs.Add("database.username", "must be between 1 and 256 characters long")
	}
	if len(c.Password) < 1 || len(c.Password) > 256 {
		errs.Add("database.password", "must be between 1 and 256 characters long")
	}
	if len(c.Database) < 1 || len(c.Database) > 256 {
		errs.Add("database.database", "must be between 1 and 256 characters long")
	}
}

func validateSerialPortConfiguration(errs url.Values, c SerialPortConfig) {
	if c.DeviceFile == "" {
		errs.Add("serial_port.device_file", "must be a filename for a serial device file, such as /dev/tty/1")
	}
}

func validateRatingConfiguration(errs url.Values, c RatingConfig) {
	if c.DefaultPoints < 1 || c.DefaultPoints > 10000 {
		errs.Add("rating.default_points", "must be a number between 1 and 10000. Default: 800")
	}
	if c.KFactor < 1 || c.KFactor > 100 {
		errs.Add("rating.default_points", "must be a number between 1 and 100. Default: 16")
	}
}

func validateImageConfiguration(errs url.Values, c ImageConfig) {
	if c.Path == ""  {
		errs.Add("images.path", "must be a path to where the image files are locates, e.g. '/home/joe/images/'")
		return
	}
	if c.Path[len(c.Path)-1:] != "/" {
		c.Path = c.Path + "/"
	}
	if strings.Contains(c.Path, "../") {
		errs.Add("images.path", "can't use path element '../', please use wouldn't work in URLs")
	}
}

func validateTimingConfiguration(errs url.Values, c TimingConfig) {
	if c.DuelTimeout < 1 || c.DuelTimeout > 120 {
		errs.Add("timings.duel_timeout", "must be a number between 1 and 120. Default: 20")
	}
	if c.Leaderboard < 1 || c.Leaderboard > 120 {
		errs.Add("timings.leaderboard", "must be a number between 1 and 120. Default: 15")
	}
	if c.SplashScreen < 1 || c.SplashScreen > 120 {
		errs.Add("timings.splash_screen", "must be a number between 1 and 120. Default: 15")
	}
}

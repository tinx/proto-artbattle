package config

import (
	"fmt"
	"strings"
	"time"
)

func ServerAddress() string {
	c := Configuration()
	sa := c.Server.Address
	if sa == "*" {
		sa = ""
	}
	return fmt.Sprintf("%s:%d", sa, c.Server.Port)
}

func DatabaseConnectString() string {
	c := Configuration()
	return fmt.Sprintf("%s:%s@%s?%s", c.Database.Username, c.Database.Password, c.Database.Database, strings.Join(c.Database.Parameters, "&"))
}

func ImagePath() string {
	return Configuration().Images.Path
}

func SerialPortDeviceFile() string {
	return Configuration().SerialPort.DeviceFile
}

func RatingDefaultPoints() int {
	return Configuration().Rating.DefaultPoints
}

func RatingKFactor() float64 {
	return Configuration().Rating.KFactor
}

func TimingsDuelTimeout() time.Duration {
	return time.Duration(Configuration().Timing.DuelTimeout)
}

func TimingsLeaderboard() time.Duration {
	return time.Duration(Configuration().Timing.Leaderboard)
}

func TimingsSplashScreen() time.Duration {
	return time.Duration(Configuration().Timing.SplashScreen)
}

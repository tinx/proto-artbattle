package main

type Configuration struct {
	Port		uint16
	DB		string
	ImagePath	string
}

var Config *Configuration

func LoadConfiguration() error {
	Config = &Configuration{
		Port: 5000,
		DB: "artbattle:hardcoded@tcp(localhost:3306)/artshow_artbattle?charset=utf8mb4&collation=utf8mb4_general_ci&parseTime=True&loc=Local",
		ImagePath: "images",
	}	
	return nil
}

func Port() uint16 {
	return Config.Port
}

func DB() string {
	return Config.DB
}

func ImagePath() string {
	return Config.ImagePath
}

package main

type Configuration struct {
	Port	uint16
	DB	string
}

var Config *Configuration

func LoadConfiguration() error {
	Config = &Configuration{
		Port: 5000,
		DB: "artbattle:hardcoded@tcp(localhost:3306)/artshow_artbattle?charset=utf8mb4&collation=utf8mb4_general_ci&parseTime=True&loc=Local",
	}
	return nil
}

func Port() uint16 {
	return Config.Port
}


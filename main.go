package main

import (
	"context"

	"log"
	logs "silo/pkg/logger"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Println("there is a error loading environment variables", err)
		return
	}
	viper.SetConfigName("silo")
	viper.SetConfigType("toml")
	viper.AddConfigPath("config/") // path to look for the config file in

	err = viper.ReadInConfig()
	if err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// Config file not found; ignore error if desired
			log.Println("there is a error in the path of config file", err)
		} else {
			// Config file was found but another error was produced
			log.Println("error laoding config file from viper", err)
		}
	}

	l, err := logs.InitializeLogger()
	if err != nil {
		log.Println("error initializing logger", err)
	}

	ctx := context.Background()
	ctx = logs.SetLoggerctx(ctx, l)

	l.Sugar().Info("sample super logger info printing")
}
